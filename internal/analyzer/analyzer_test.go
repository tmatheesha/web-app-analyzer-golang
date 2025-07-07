package analyzer

import (
	"WebAppAnalyzer/config/env"
	"WebAppAnalyzer/config/logger"
	"WebAppAnalyzer/internal/models"
	"WebAppAnalyzer/internal/validator"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

// MockResponse represents a mock HTTP response
type MockResponse struct {
	StatusCode int
	Body       string
	Headers    map[string]string
	Error      error
}

// MockTransport implements http.RoundTripper for testing
type MockTransport struct {
	responses map[string]*MockResponse
}

func (m *MockTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	url := req.URL.String()

	if response, exists := m.responses[url]; exists {
		if response.Error != nil {
			return nil, response.Error
		}

		resp := &http.Response{
			StatusCode: response.StatusCode,
			Body:       io.NopCloser(strings.NewReader(response.Body)),
			Header:     make(http.Header),
			Request:    req,
		}

		for key, value := range response.Headers {
			resp.Header.Set(key, value)
		}

		return resp, nil
	}

	// Default 404 response for unknown URLs
	return &http.Response{
		StatusCode: 404,
		Body:       io.NopCloser(strings.NewReader("Not Found")),
		Request:    req,
	}, nil
}

// createTestAnalyzer creates an analyzer with mock HTTP client for testing
func createTestAnalyzer() (*PageAnalyzer, *MockTransport) {
	config := &env.Config{
		LogLevel:     "debug",
		NumOfWorkers: 5,
	}
	logger := logger.NewLogger(*config)

	mockTransport := &MockTransport{
		responses: make(map[string]*MockResponse),
	}

	analyzer := &PageAnalyzer{
		client: &http.Client{
			Transport: mockTransport,
			Timeout:   30 * time.Second,
		},
		validator: validator.NewURLValidator(),
		logger:    logger,
		config:    config,
	}

	return analyzer, mockTransport
}

// TestPageAnalyzer tests the main analyzer functionality
func TestPageAnalyzer(t *testing.T) {
	analyzer, mockTransport := createTestAnalyzer()

	// Setup mock responses
	mockTransport.responses["https://example.com"] = &MockResponse{
		StatusCode: 200,
		Body: `<!DOCTYPE html>
<html>
<head>
    <title>Example Domain</title>
</head>
<body>
    <h1>Example Domain</h1>
    <p>This domain is for use in illustrative examples.</p>
    <a href="https://www.iana.org/domains/example">More information...</a>
	<h2>Subheading</h2>
	<p>Another paragraph.</p>
    <form action="/login" method="post">
		<input type="text" name="username" id="username">
		<input type="password" name="password" id="password">
		<input type="submit" value="Login">
	</form>
</body>
</html>`,
		Headers: map[string]string{
			"Content-Type": "text/html",
		},
	}

	mockTransport.responses["https://example.com/notfound"] = &MockResponse{
		StatusCode: 404,
		Body:       "Not Found",
		Headers: map[string]string{
			"Content-Type": "text/plain",
		},
	}

	// Test cases
	testCases := []struct {
		name           string
		url            string
		expectedError  bool
		expectedStatus int
		checkResult    func(*testing.T, *models.AnalysisResult)
	}{
		{
			name:           "Valid URL with complete HTML",
			url:            "https://example.com",
			expectedError:  false,
			expectedStatus: 200,
			checkResult: func(t *testing.T, result *models.AnalysisResult) {
				if result.PageTitle != "Example Domain" {
					t.Errorf("Expected page title 'Example Domain', got '%s'", result.PageTitle)
				}
				if result.HTMLVersion != "HTML5" {
					t.Errorf("Expected HTML version 'HTML5', got '%s'", result.HTMLVersion)
				}
				if result.Headings["h1"] != 1 {
					t.Errorf("Expected 1 h1 heading, got %d", result.Headings["h1"])
				}
				if result.Headings["h2"] != 1 {
					t.Errorf("Expected 1 h2 headings, got %d", result.Headings["h2"])
				}
				if result.InternalLinks != 0 {
					t.Errorf("Expected 0 internal links, got %d", result.InternalLinks)
				}
				if result.ExternalLinks != 1 {
					t.Errorf("Expected 1 external link, got %d", result.ExternalLinks)
				}
			},
		},
		{
			name:           "Invalid URL",
			url:            "not-a-url",
			expectedError:  true,
			expectedStatus: 400,
			checkResult: func(t *testing.T, result *models.AnalysisResult) {
				if result.Error == "" {
					t.Error("Expected error message for invalid URL")
				}
			},
		},
		{
			name:           "HTTP 404 Error",
			url:            "https://example.com/notfound",
			expectedError:  true,
			expectedStatus: 404,
			checkResult: func(t *testing.T, result *models.AnalysisResult) {
				if result.HTTPStatusCode != 404 {
					t.Errorf("Expected status code 404, got %d", result.HTTPStatusCode)
				}
			},
		},
		{
			name:           "Empty URL",
			url:            "",
			expectedError:  true,
			expectedStatus: 400,
			checkResult: func(t *testing.T, result *models.AnalysisResult) {
				if result.Error == "" {
					t.Error("Expected error message for empty URL")
				}
			},
		},
		{
			name:           "URL without scheme",
			url:            "example.com",
			expectedError:  false,
			expectedStatus: 200,
			checkResult: func(t *testing.T, result *models.AnalysisResult) {
				if result.Error != "" {
					t.Errorf("Unexpected error: %s", result.Error)
				}
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			result := analyzer.Analyze(ctx, tc.url)

			if tc.expectedError {
				if result.Error == "" {
					t.Error("Expected error but got none")
				}
			} else {
				if result.Error != "" {
					t.Errorf("Unexpected error: %s", result.Error)
				}
			}

			if tc.expectedStatus != 0 && result.HTTPStatusCode != tc.expectedStatus {
				t.Errorf("Expected status code %d, got %d", tc.expectedStatus, result.HTTPStatusCode)
			}

			if tc.checkResult != nil {
				tc.checkResult(t, result)
			}
		})
	}
}

// TestHTMLParsing tests HTML parsing functionality
func TestHTMLParsing(t *testing.T) {
	analyzer, mockTransport := createTestAnalyzer()

	// Test HTML with various elements
	htmlContent := `<!DOCTYPE html>
<html>
<head>
    <title>Test Page</title>
</head>
<body>
    <h1>Main Heading</h1>
    <h2>Sub Heading 1</h2>
    <h2>Sub Heading 2</h2>
    <h3>Sub Sub Heading</h3>
    <a href="/internal1">Internal Link 1</a>
    <a href="/internal2">Internal Link 2</a>
    <a href="https://external.com">External Link</a>
    <form action="/login" method="post">
        <input type="text" name="username" id="username">
        <input type="password" name="password" id="password">
        <input type="submit" value="Login">
    </form>
</body>
</html>`

	// Setup mock response
	mockTransport.responses["https://testpage.com"] = &MockResponse{
		StatusCode: 200,
		Body:       htmlContent,
		Headers: map[string]string{
			"Content-Type": "text/html",
		},
	}

	// Test the analyzer
	ctx := context.Background()
	result := analyzer.Analyze(ctx, "https://testpage.com")

	// Verify results
	if result.Error != "" {
		t.Errorf("Unexpected error: %s", result.Error)
	}

	if result.PageTitle != "Test Page" {
		t.Errorf("Expected page title 'Test Page', got '%s'", result.PageTitle)
	}

	if result.HTMLVersion != "HTML5" {
		t.Errorf("Expected HTML version 'HTML5', got '%s'", result.HTMLVersion)
	}

	// Check headings
	expectedHeadings := map[string]int{
		"h1": 1,
		"h2": 2,
		"h3": 1,
	}

	for heading, expectedCount := range expectedHeadings {
		if result.Headings[heading] != expectedCount {
			t.Errorf("Expected %d %s headings, got %d", expectedCount, heading, result.Headings[heading])
		}
	}

	// Check links
	if result.InternalLinks != 2 {
		t.Errorf("Expected 2 internal links, got %d", result.InternalLinks)
	}

	if result.ExternalLinks != 1 {
		t.Errorf("Expected 1 external link, got %d", result.ExternalLinks)
	}

	// Check login form detection
	if !result.HasLoginForm {
		t.Error("Expected login form to be detected")
	}
}

// TestFormDetection tests form detection functionality
func TestFormDetection(t *testing.T) {
	analyzer, mockTransport := createTestAnalyzer()

	testCases := []struct {
		name          string
		html          string
		expectedLogin bool
		description   string
	}{
		{
			name: "Login form with password and username",
			html: `<form>
				<input type="text" name="username">
				<input type="password" name="password">
			</form>`,
			expectedLogin: true,
			description:   "Form with username and password fields",
		},
		{
			name: "Login form with password and email",
			html: `<form>
				<input type="email" name="email">
				<input type="password" name="password">
			</form>`,
			expectedLogin: true,
			description:   "Form with email and password fields",
		},
		{
			name: "Login form with action attribute",
			html: `<form action="/login">
				<input type="text" name="user">
				<input type="password" name="pass">
			</form>`,
			expectedLogin: true,
			description:   "Form with login action",
		},
		{
			name: "Login form with class attribute",
			html: `<form class="login-form">
				<input type="text" name="user">
				<input type="password" name="pass">
			</form>`,
			expectedLogin: true,
			description:   "Form with login class",
		},
		{
			name: "Non-login form",
			html: `<form>
				<input type="text" name="search">
				<input type="submit" value="Search">
			</form>`,
			expectedLogin: false,
			description:   "Form without password field",
		},
		{
			name: "Form with only password",
			html: `<form>
				<input type="password" name="password">
			</form>`,
			expectedLogin: false,
			description:   "Form with only password field",
		},
	}

	for i, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create unique URL for each test case
			testURL := fmt.Sprintf("https://test%d.com", i)

			// Setup mock response
			mockTransport.responses[testURL] = &MockResponse{
				StatusCode: 200,
				Body:       fmt.Sprintf(`<!DOCTYPE html><html><head><title>Test</title></head><body>%s</body></html>`, tc.html),
				Headers: map[string]string{
					"Content-Type": "text/html",
				},
			}

			// Test the analyzer
			ctx := context.Background()
			result := analyzer.Analyze(ctx, testURL)

			if result.Error != "" {
				t.Errorf("Unexpected error: %s", result.Error)
			}

			if result.HasLoginForm != tc.expectedLogin {
				t.Errorf("%s: Expected login form detection to be %v, got %v", tc.description, tc.expectedLogin, result.HasLoginForm)
			}
		})
	}
}

// TestLinkAnalysis tests link analysis functionality
func TestLinkAnalysis(t *testing.T) {
	analyzer, mockTransport := createTestAnalyzer()

	// Setup mock responses for main page and internal links
	mockTransport.responses["https://testlinks.com"] = &MockResponse{
		StatusCode: 200,
		Body: `<!DOCTYPE html>
<html>
<head><title>Main Page</title></head>
<body>
    <a href="/internal1">Internal Link 1</a>
    <a href="/internal2">Internal Link 2</a>
    <a href="https://external1.com">External Link 1</a>
    <a href="https://external2.com">External Link 2</a>
    <a href="https://broken-link.com">Broken Link</a>
</body>
</html>`,
		Headers: map[string]string{
			"Content-Type": "text/html",
		},
	}

	mockTransport.responses["https://external1.com"] = &MockResponse{
		StatusCode: 200,
		Body:       "<html><body>External 1</body></html>",
		Headers: map[string]string{
			"Content-Type": "text/html",
		},
	}

	mockTransport.responses["https://external2.com"] = &MockResponse{
		StatusCode: 200,
		Body:       "<html><body>External 2</body></html>",
		Headers: map[string]string{
			"Content-Type": "text/html",
		},
	}

	mockTransport.responses["https://broken-link.com"] = &MockResponse{
		StatusCode: 404,
		Body:       "Not Found",
		Headers: map[string]string{
			"Content-Type": "text/plain",
		},
	}

	// Test the analyzer
	ctx := context.Background()
	result := analyzer.Analyze(ctx, "https://testlinks.com")

	if result.Error != "" {
		t.Errorf("Unexpected error: %s", result.Error)
	}

	// Check link counts
	if result.InternalLinks != 2 {
		t.Errorf("Expected 2 internal links, got %d", result.InternalLinks)
	}

	if result.ExternalLinks != 3 {
		t.Errorf("Expected 3 external links, got %d", result.ExternalLinks)
	}

	// Check inaccessible links (should be 1 - the broken link)
	if result.InaccessibleLinks != 1 {
		t.Errorf("Expected 1 inaccessible link, got %d", result.InaccessibleLinks)
	}
}

// TestConcurrency tests concurrent link checking
func TestConcurrency(t *testing.T) {
	analyzer, mockTransport := createTestAnalyzer()

	// Setup main page with many external links
	mainPageHTML := `<!DOCTYPE html><html><head><title>Test</title></head><body>`
	for i := 0; i < 10; i++ {
		mainPageHTML += fmt.Sprintf(`<a href="https://external%d.com">External %d</a>`, i, i)
	}
	mainPageHTML += `</body></html>`

	mockTransport.responses["https://concurrencytest.com"] = &MockResponse{
		StatusCode: 200,
		Body:       mainPageHTML,
		Headers: map[string]string{
			"Content-Type": "text/html",
		},
	}

	// Setup responses for external links (some accessible, some not)
	for i := 0; i < 10; i++ {
		url := fmt.Sprintf("https://external%d.com", i)
		if i%2 == 0 {
			// Even numbered links are accessible
			mockTransport.responses[url] = &MockResponse{
				StatusCode: 200,
				Body:       fmt.Sprintf("<html><body>External %d</body></html>", i),
				Headers: map[string]string{
					"Content-Type": "text/html",
				},
			}
		} else {
			// Odd numbered links are inaccessible
			mockTransport.responses[url] = &MockResponse{
				StatusCode: 404,
				Body:       "Not Found",
				Headers: map[string]string{
					"Content-Type": "text/plain",
				},
			}
		}
	}

	// Test with timeout context
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	start := time.Now()
	result := analyzer.Analyze(ctx, "https://concurrencytest.com")
	duration := time.Since(start)

	// Verify the analysis completed within reasonable time
	if duration > 3*time.Second {
		t.Errorf("Analysis took too long: %v", duration)
	}

	if result.Error != "" {
		t.Errorf("Unexpected error: %s", result.Error)
	}

	// Should have detected external links
	if result.ExternalLinks != 10 {
		t.Errorf("Expected 10 external links, got %d", result.ExternalLinks)
	}

	// Should have detected inaccessible links
	if result.InaccessibleLinks != 5 {
		t.Errorf("Expected 5 inaccessible links, got %d", result.InaccessibleLinks)
	}
}

// TestPerformance tests analyzer performance with large HTML
func TestPerformance(t *testing.T) {
	analyzer, mockTransport := createTestAnalyzer()

	// Generate large HTML content
	html := `<!DOCTYPE html><html><head><title>Large Page</title></head><body>`

	// Add many headings
	for i := 0; i < 100; i++ {
		html += fmt.Sprintf("<h1>Heading %d</h1>", i)
		html += fmt.Sprintf("<h2>Subheading %d</h2>", i)
	}

	// Add many links
	for i := 0; i < 50; i++ {
		html += fmt.Sprintf(`<a href="/internal%d">Internal %d</a>`, i, i)
		html += fmt.Sprintf(`<a href="https://external%d.com">External %d</a>`, i, i)
	}

	// Add forms
	for i := 0; i < 5; i++ {
		html += fmt.Sprintf(`<form action="/form%d"><input type="text"><input type="password"></form>`, i)
	}

	html += `</body></html>`

	// Setup mock response
	mockTransport.responses["https://largepage.com"] = &MockResponse{
		StatusCode: 200,
		Body:       html,
		Headers: map[string]string{
			"Content-Type": "text/html",
		},
	}

	// Setup responses for external links
	for i := 0; i < 50; i++ {
		url := fmt.Sprintf("https://external%d.com", i)
		mockTransport.responses[url] = &MockResponse{
			StatusCode: 200,
			Body:       fmt.Sprintf("<html><body>External %d</body></html>", i),
			Headers: map[string]string{
				"Content-Type": "text/html",
			},
		}
	}

	// Test performance
	ctx := context.Background()
	start := time.Now()
	result := analyzer.Analyze(ctx, "https://largepage.com")
	duration := time.Since(start)

	// Verify performance (should complete within 2 seconds)
	if duration > 2*time.Second {
		t.Errorf("Analysis took too long: %v", duration)
	}

	if result.Error != "" {
		t.Errorf("Unexpected error: %s", result.Error)
	}

	// Verify results
	if result.Headings["h1"] != 100 {
		t.Errorf("Expected 100 h1 headings, got %d", result.Headings["h1"])
	}

	if result.Headings["h2"] != 100 {
		t.Errorf("Expected 100 h2 headings, got %d", result.Headings["h2"])
	}

	if result.InternalLinks != 50 {
		t.Errorf("Expected 50 internal links, got %d", result.InternalLinks)
	}

	if result.ExternalLinks != 50 {
		t.Errorf("Expected 50 external links, got %d", result.ExternalLinks)
	}

	//if !result.HasLoginForm {
	//	t.Error("Expected login form to be detected")
	//}
}

// BenchmarkAnalyzer benchmarks the analyzer performance
func BenchmarkAnalyzer(b *testing.B) {
	analyzer, mockTransport := createTestAnalyzer()

	// Setup mock response with sample HTML
	mockTransport.responses["https://benchmark.com"] = &MockResponse{
		StatusCode: 200,
		Body: `<!DOCTYPE html>
<html>
<head><title>Benchmark Test</title></head>
<body>
    <h1>Main Heading</h1>
    <h2>Sub Heading</h2>
    <a href="/internal">Internal Link</a>
    <a href="https://external.com">External Link</a>
    <form><input type="text"><input type="password"></form>
</body>
</html>`,
		Headers: map[string]string{
			"Content-Type": "text/html",
		},
	}

	// Setup external link response
	mockTransport.responses["https://external.com"] = &MockResponse{
		StatusCode: 200,
		Body:       "<html><body>External</body></html>",
		Headers: map[string]string{
			"Content-Type": "text/html",
		},
	}

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		analyzer.Analyze(ctx, "https://benchmark.com")
	}
}
