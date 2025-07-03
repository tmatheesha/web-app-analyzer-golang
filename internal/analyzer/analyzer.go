package analyzer

import (
	"WebAppAnalyzer/config/env"
	"WebAppAnalyzer/config/logger"
	"WebAppAnalyzer/internal/models"
	"WebAppAnalyzer/internal/validator"
	"context"
	"fmt"
	"golang.org/x/net/html"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

type PageAnalyzer struct {
	client    *http.Client
	validator *validator.URLValidator
	logger    *logger.Logger
	config    *env.Config
}

type LinkCheckResult struct {
	URL          string
	IsAccessible bool
	StatusCode   int
	Error        error
}

func NewPageAnalyzer(logger *logger.Logger, c *env.Config) *PageAnalyzer {
	return &PageAnalyzer{
		client: &http.Client{
			Timeout: 30 * time.Second,
			Transport: &http.Transport{
				MaxIdleConns:        100,
				MaxIdleConnsPerHost: 10,
				IdleConnTimeout:     90 * time.Second,
			},
		},
		validator: validator.NewURLValidator(),
		logger:    logger,
		config:    c,
	}
}

func (p *PageAnalyzer) Analyze(ctx context.Context, url string) *models.AnalysisResult {

	result := models.NewAnalysisResult(url)
	validatedUrl, err := p.validator.ValidateURL(url)
	if err != nil {
		result.SetError(fmt.Sprintf("Invalid URL: %s", url), 400)
		p.logger.Error("Invalid URL", err)
		return result
	}

	// Log the analysis request
	p.logger.Info("Analyzing URL", validatedUrl)

	resp, err := p.fetchPage(ctx, validatedUrl)
	if err != nil {
		result.SetError(fmt.Sprintf("Failed to fetch URL: %v", err), 0)
		return result
	}
	defer resp.Body.Close()

	// Check if the response is successful
	if resp.StatusCode != http.StatusOK {
		result.SetError(fmt.Sprintf("HTTP Error: %d - %s", resp.StatusCode, http.StatusText(resp.StatusCode)), resp.StatusCode)
		return result
	}

	// Parse the HTML
	doc, err := p.parseHTML(resp.Body)
	if err != nil {
		result.SetError(fmt.Sprintf("Failed to parse HTML: %v", err), 0)
		return result
	}

	p.logger.Info("Page analysis completed")

	p.analyzeHTMLWithConcurrency(ctx, doc, validatedUrl, result)

	p.logger.Info("Page analysis completed")

	return result
}

func (p *PageAnalyzer) analyzeHTMLWithConcurrency(ctx context.Context, n *html.Node, baseURL string, result *models.AnalysisResult) {
	externalLinks := make(chan string, 100)

	var wg sync.WaitGroup

	linkResults := make(chan LinkCheckResult, 100)

	p.startLinkCheckers(ctx, externalLinks, linkResults, &wg)

	p.analyzeHTML(ctx, n, baseURL, result, externalLinks)

	close(externalLinks)

	wg.Wait()
	close(linkResults)

	p.processLinkResults(linkResults, result)
}

func (p *PageAnalyzer) processLinkResults(results <-chan LinkCheckResult, analysisResult *models.AnalysisResult) {
	for result := range results {
		if !result.IsAccessible {
			analysisResult.InaccessibleLinks++
		}
	}
}

func (p *PageAnalyzer) analyzeHTML(ctx context.Context, n *html.Node, baseURL string, result *models.AnalysisResult, externalLinks chan<- string) {
	// Analyze current node
	switch n.Type {
	case html.DocumentNode:
		// Check for DOCTYPE to determine HTML version
		if n.FirstChild != nil && n.FirstChild.Type == html.DoctypeNode {
			result.HTMLVersion = p.extractHTMLVersion(n.FirstChild)
		}
	case html.ElementNode:
		switch n.Data {
		case "title":
			if n.FirstChild != nil && n.FirstChild.Type == html.TextNode {
				result.PageTitle = strings.TrimSpace(n.FirstChild.Data)
			}
		case "h1", "h2", "h3", "h4", "h5", "h6":
			result.AddHeading(n.Data)
		case "a":
			p.analyzeLink(ctx, n, baseURL, result, externalLinks)
		case "form":
			p.analyzeForm(n, result)
		}
	}

	// Recursively analyze child nodes
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		p.analyzeHTML(ctx, c, baseURL, result, externalLinks)
	}
}

func (p *PageAnalyzer) analyzeForm(n *html.Node, result *models.AnalysisResult) {
	// Look for common login form indicators
	hasPassword := false
	hasUsername := false
	hasEmail := false

	// Check form attributes
	for _, attr := range n.Attr {
		if attr.Key == "action" {
			action := strings.ToLower(attr.Val)
			if strings.Contains(action, "login") || strings.Contains(action, "signin") {
				result.HasLoginForm = true
				return
			}
		}
		if attr.Key == "id" || attr.Key == "class" {
			value := strings.ToLower(attr.Val)
			if strings.Contains(value, "login") || strings.Contains(value, "signin") {
				result.HasLoginForm = true
				return
			}
		}
	}

	// Check for input fields
	p.checkFormInputs(n, &hasPassword, &hasUsername, &hasEmail)

	// Determine if it's a login form based on input fields
	if hasPassword && (hasUsername || hasEmail) {
		result.HasLoginForm = true
	}
}

func (p *PageAnalyzer) checkFormInputs(n *html.Node, hasPassword, hasUsername, hasEmail *bool) {
	if n.Type == html.ElementNode && n.Data == "input" {
		var inputType, inputName, inputID string
		for _, attr := range n.Attr {
			switch attr.Key {
			case "type":
				inputType = strings.ToLower(attr.Val)
			case "name":
				inputName = strings.ToLower(attr.Val)
			case "id":
				inputID = strings.ToLower(attr.Val)
			}
		}

		// Check for password field
		if inputType == "password" {
			*hasPassword = true
		}

		// Check for username/email fields
		if inputType == "text" || inputType == "email" {
			if strings.Contains(inputName, "user") || strings.Contains(inputName, "email") ||
				strings.Contains(inputID, "user") || strings.Contains(inputID, "email") {
				if inputType == "email" {
					*hasEmail = true
				} else {
					*hasUsername = true
				}
			}
		}
	}

	// Recursively check child nodes
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		p.checkFormInputs(c, hasPassword, hasUsername, hasEmail)
	}
}

// analyzeForm checks if a form is a login form and extracts its action and method
func (p *PageAnalyzer) analyzeLink(ctx context.Context, n *html.Node, baseURL string, result *models.AnalysisResult, externalLinks chan<- string) {
	var href string
	for _, attr := range n.Attr {
		if attr.Key == "href" {
			href = attr.Val
			break
		}
	}

	if href == "" {
		return
	}

	// Skip javascript: and mailto: links
	if strings.HasPrefix(href, "javascript:") || strings.HasPrefix(href, "mailto:") {
		return
	}

	parsedURL, err := url.Parse(href)
	if err != nil {
		result.InaccessibleLinks++
		return
	}

	// If it's a relative URL, make it absolute
	if !parsedURL.IsAbs() {
		baseParsed, err := url.Parse(baseURL)
		if err != nil {
			result.InaccessibleLinks++
			return
		}
		parsedURL = baseParsed.ResolveReference(parsedURL)
	}

	// Check if it's internal or external
	if p.validator.IsInternalLink(parsedURL.String(), baseURL) {
		result.InternalLinks++
	} else {
		result.ExternalLinks++
		select {
		case externalLinks <- parsedURL.String():
			// Link sent successfully
		default:
			p.logger.WithField("url", parsedURL.String()).Warn("Link checking channel full, skipping link")
		}
	}
}

func (p *PageAnalyzer) extractHTMLVersion(n *html.Node) string {
	if n.Type != html.DoctypeNode {
		return "Unknown"
	}

	// Check for HTML5
	if len(n.Attr) == 0 {
		return "HTML5"
	}

	// Check for other versions
	for _, attr := range n.Attr {
		if attr.Key == "html" {
			return "HTML " + attr.Val
		}
	}

	return "Unknown"
}

func (pa *PageAnalyzer) parseHTML(body io.Reader) (*html.Node, error) {
	doc, err := html.Parse(body)
	if err != nil {
		return nil, fmt.Errorf("failed to parse HTML: %w", err)
	}
	return doc, nil
}

func (p *PageAnalyzer) fetchPage(ctx context.Context, url string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("User-Agent", "WebPageAnalyzer/1.0")

	resp, err := p.client.Do(req)
	if err != nil {
		p.logger.Error("Failed to fetch URL", url, err)
		return nil, fmt.Errorf("failed to fetch page: %w", err)
	}

	return resp, nil
}

func (p *PageAnalyzer) startLinkCheckers(ctx context.Context, links <-chan string, results chan<- LinkCheckResult, wg *sync.WaitGroup) {
	// Start 5 worker goroutines for link checking
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			p.linkChecker(ctx, links, results, workerID)
		}(i)
	}
}

func (p *PageAnalyzer) linkChecker(ctx context.Context, links <-chan string, results chan<- LinkCheckResult, workerID int) {
	for linkURL := range links {
		select {
		case <-ctx.Done():
			// Context cancelled, stop processing
			return
		default:
			result := p.checkSingleLink(ctx, linkURL)
			results <- result
		}
	}
}

func (p *PageAnalyzer) checkSingleLink(ctx context.Context, linkURL string) LinkCheckResult {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "HEAD", linkURL, nil)
	if err != nil {
		return LinkCheckResult{
			URL:          linkURL,
			IsAccessible: false,
			Error:        err,
		}
	}

	req.Header.Set("User-Agent", "WebPageAnalyzer/1.0")

	resp, err := p.client.Do(req)
	if err != nil {
		return LinkCheckResult{
			URL:          linkURL,
			IsAccessible: false,
			Error:        err,
		}
	}
	defer resp.Body.Close()

	return LinkCheckResult{
		URL:          linkURL,
		IsAccessible: resp.StatusCode == http.StatusOK,
		StatusCode:   resp.StatusCode,
	}
}
