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

// Analyze performs the analysis of the given URL and returns the result
func (p *PageAnalyzer) Analyze(ctx context.Context, url string) *models.AnalysisResult {

	result := models.NewAnalysisResult(url)
	validatedUrl, err := p.validator.ValidateURL(url)
	if err != nil {
		result.SetError(fmt.Sprintf("Invalid URL: %s", url), 400)
		p.logger.Error("Invalid URL", err)
		return result
	}

	p.logger.Info("Analyzing URL", validatedUrl)

	resp, err := p.fetchPage(ctx, validatedUrl)
	if err != nil {
		result.SetError(fmt.Sprintf("Failed to fetch URL: %v", err), 0)
		return result
	}
	defer resp.Body.Close()

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

	p.analyzeHTMLWithConcurrency(ctx, doc, validatedUrl, result)

	p.logger.Info("Page analysis completed")

	return result
}

// analyzeHTMLWithConcurrency analyzes the HTML document concurrently
func (p *PageAnalyzer) analyzeHTMLWithConcurrency(ctx context.Context, n *html.Node, baseURL string, result *models.AnalysisResult) {
	externalLinks := make(chan string, 100)
	linkResults := make(chan LinkCheckResult, 100)

	// Start link checker workers in goroutines because they will block on network I/O
	numWorkers := p.config.NumOfWorkers
	var wg sync.WaitGroup

	// Start multiple link checker workers
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			p.linkChecker(ctx, externalLinks, linkResults)
		}()
	}

	go func() {
		p.analyzeHTML(ctx, n, baseURL, result, externalLinks)
		close(externalLinks)
	}()

	// Wait till all link checkers to finish, then close results channel
	go func() {
		wg.Wait()
		close(linkResults)
	}()

	// Process results (this will block until all results are processed)
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
			p.checkSkipLink(n, result)
		case "form":
			p.analyzeForm(n, result)
		case "img":
			p.analyzeImage(n, baseURL, result)
		case "meta":
			p.analyzeMetaTag(n, result)
		case "script":
			p.analyzeScript(n, baseURL, result)
		case "link":
			p.analyzeStylesheet(n, baseURL, result)
		case "table":
			result.Tables++
		case "ul", "ol":
			result.Lists++
		case "button":
			result.Buttons++
		case "input":
			result.Inputs++
		case "p":
			result.TextContent.Paragraphs++
		case "main", "article", "section", "nav", "header", "footer", "aside":
			result.Accessibility.HasSemanticHTML = true
		}

		// Check for ARIA labels on any element
		p.checkARIALabels(n, result)
	case html.TextNode:
		// Analyze text content
		p.analyzeTextContent(n, result)
	}

	// Recursively analyze child nodes
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		p.analyzeHTML(ctx, c, baseURL, result, externalLinks)
	}
}

func (p *PageAnalyzer) analyzeForm(n *html.Node, result *models.AnalysisResult) {
	formInfo := models.FormInfo{}
	inputCount := 0

	// Look for common login form indicators
	hasPassword := false
	hasUsername := false
	hasEmail := false

	// Check form attributes
	for _, attr := range n.Attr {
		if attr.Key == "action" {
			formInfo.Action = attr.Val
			action := strings.ToLower(attr.Val)
			if strings.Contains(action, "login") || strings.Contains(action, "signin") {
				formInfo.HasLogin = true
			}
		}
		if attr.Key == "method" {
			formInfo.Method = strings.ToUpper(attr.Val)
		}
		if attr.Key == "id" || attr.Key == "class" {
			value := strings.ToLower(attr.Val)
			if strings.Contains(value, "login") || strings.Contains(value, "signin") {
				formInfo.HasLogin = true
			}
		}
	}

	// Check for input fields
	p.checkFormInputs(n, &hasPassword, &hasUsername, &hasEmail, &inputCount)

	if hasPassword && (hasUsername || hasEmail) {
		formInfo.HasLogin = true
	}

	formInfo.InputCount = inputCount
	result.Forms = append(result.Forms, formInfo)

	if formInfo.HasLogin {
		result.HasLoginForm = true
	}
}

// checkFormInputs recursively checks input fields in a form to determine if it contains login fields
func (p *PageAnalyzer) checkFormInputs(n *html.Node, hasPassword, hasUsername, hasEmail *bool, inputCount *int) {
	if n.Type == html.ElementNode && n.Data == "input" {
		*inputCount++
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

		if inputType == "password" {
			*hasPassword = true
		}

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
		p.checkFormInputs(c, hasPassword, hasUsername, hasEmail, inputCount)
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

// linkChecker checks links concurrently and sends results to the results channel
func (p *PageAnalyzer) linkChecker(ctx context.Context, links <-chan string, results chan<- LinkCheckResult) {
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

// analyzeImage analyzes an img element and extracts relevant information
func (p *PageAnalyzer) analyzeImage(n *html.Node, baseURL string, result *models.AnalysisResult) {
	imgInfo := models.ImageInfo{}

	for _, attr := range n.Attr {
		switch attr.Key {
		case "src":
			imgInfo.Src = attr.Val
		case "alt":
			imgInfo.Alt = attr.Val
			if attr.Val != "" {
				result.Accessibility.HasAltText = true
			}
		case "width":
			imgInfo.Width = attr.Val
		case "height":
			imgInfo.Height = attr.Val
		}
	}

	if imgInfo.Src != "" {
		// Check if it's an external image
		parsedURL, err := url.Parse(imgInfo.Src)
		if err == nil {
			if !parsedURL.IsAbs() {
				baseParsed, err := url.Parse(baseURL)
				if err == nil {
					parsedURL = baseParsed.ResolveReference(parsedURL)
				}
			}
			imgInfo.IsExternal = !p.validator.IsInternalLink(parsedURL.String(), baseURL)
		}
		result.Images = append(result.Images, imgInfo)
	}
}

// analyzeMetaTag analyzes a meta element and extracts relevant information
func (p *PageAnalyzer) analyzeMetaTag(n *html.Node, result *models.AnalysisResult) {
	metaTag := models.MetaTag{}

	for _, attr := range n.Attr {
		switch attr.Key {
		case "name":
			metaTag.Name = attr.Val
		case "content":
			metaTag.Content = attr.Val
		case "property":
			metaTag.Property = attr.Val
		}
	}

	if metaTag.Name != "" || metaTag.Property != "" {
		result.MetaTags = append(result.MetaTags, metaTag)
	}
}

// analyzeScript analyzes a script element and extracts relevant information
func (p *PageAnalyzer) analyzeScript(n *html.Node, baseURL string, result *models.AnalysisResult) {
	scriptInfo := models.ScriptInfo{}

	for _, attr := range n.Attr {
		switch attr.Key {
		case "src":
			scriptInfo.Src = attr.Val
		case "type":
			scriptInfo.Type = attr.Val
		}
	}

	if scriptInfo.Src != "" {
		// Check if it's an external script
		parsedURL, err := url.Parse(scriptInfo.Src)
		if err == nil {
			if !parsedURL.IsAbs() {
				baseParsed, err := url.Parse(baseURL)
				if err == nil {
					parsedURL = baseParsed.ResolveReference(parsedURL)
				}
			}
			scriptInfo.IsExternal = !p.validator.IsInternalLink(parsedURL.String(), baseURL)
		}
		result.Scripts = append(result.Scripts, scriptInfo)
	}
}

// analyzeStylesheet analyzes a link element for stylesheets and extracts relevant information
func (p *PageAnalyzer) analyzeStylesheet(n *html.Node, baseURL string, result *models.AnalysisResult) {
	var rel, href, media string

	for _, attr := range n.Attr {
		switch attr.Key {
		case "rel":
			rel = strings.ToLower(attr.Val)
		case "href":
			href = attr.Val
		case "media":
			media = attr.Val
		}
	}

	if rel == "stylesheet" && href != "" {
		stylesheetInfo := models.StylesheetInfo{
			Href:  href,
			Media: media,
		}

		// Check if it's an external stylesheet
		parsedURL, err := url.Parse(href)
		if err == nil {
			if !parsedURL.IsAbs() {
				baseParsed, err := url.Parse(baseURL)
				if err == nil {
					parsedURL = baseParsed.ResolveReference(parsedURL)
				}
			}
			stylesheetInfo.IsExternal = !p.validator.IsInternalLink(parsedURL.String(), baseURL)
		}
		result.Stylesheets = append(result.Stylesheets, stylesheetInfo)
	}
}

// analyzeTextContent analyzes text nodes for content statistics
func (p *PageAnalyzer) analyzeTextContent(n *html.Node, result *models.AnalysisResult) {
	text := strings.TrimSpace(n.Data)
	if text == "" {
		return
	}

	// Count characters
	result.TextContent.CharCount += len(text)

	// Count words (simple word counting)
	words := strings.Fields(text)
	result.TextContent.WordCount += len(words)
}

// checkSkipLink checks if a link is a skip link for accessibility
func (p *PageAnalyzer) checkSkipLink(n *html.Node, result *models.AnalysisResult) {
	for _, attr := range n.Attr {
		if attr.Key == "href" && strings.HasPrefix(attr.Val, "#") {
			// Check if it's a skip link by looking at text content or aria-label
			text := p.getTextContent(n)
			text = strings.ToLower(text)
			if strings.Contains(text, "skip") || strings.Contains(text, "jump") {
				result.Accessibility.HasSkipLinks = true
				return
			}
		}
	}
}

// checkARIALabels checks for ARIA labels on elements
func (p *PageAnalyzer) checkARIALabels(n *html.Node, result *models.AnalysisResult) {
	for _, attr := range n.Attr {
		if strings.HasPrefix(attr.Key, "aria-") && attr.Val != "" {
			result.Accessibility.HasARIALabels = true
			return
		}
	}
}

// getTextContent extracts text content from a node and its children
func (p *PageAnalyzer) getTextContent(n *html.Node) string {
	var text strings.Builder
	p.extractText(n, &text)
	return strings.TrimSpace(text.String())
}

// extractText recursively extracts text from a node and its children
func (p *PageAnalyzer) extractText(n *html.Node, text *strings.Builder) {
	if n.Type == html.TextNode {
		text.WriteString(n.Data)
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		p.extractText(c, text)
	}
}
