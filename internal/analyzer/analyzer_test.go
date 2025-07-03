package analyzer

import (
	"WebAppAnalyzer/config/env"
	"WebAppAnalyzer/config/logger"
	"WebAppAnalyzer/internal/models"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// TestPageAnalyzer tests the main analyzer functionality
func TestPageAnalyzer(t *testing.T) {
	// Setup test environment
	config := &env.Config{
		LogLevel: "debug",
	}
	logger := logger.NewLogger(*config)

	// Create analyzer
	analyzer := NewPageAnalyzer(logger, config)

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
					t.Errorf("Expected page title 'Test Page', got '%s'", result.PageTitle)
				}
				if result.HTMLVersion != "HTML5" {
					t.Errorf("Expected HTML version 'HTML5', got '%s'", result.HTMLVersion)
				}
				if result.Headings["h1"] != 1 {
					t.Errorf("Expected 1 h1 heading, got %d", result.Headings["h1"])
				}
				if result.Headings["h2"] != 0 {
					t.Errorf("Expected 2 h2 headings, got %d", result.Headings["h2"])
				}
				if result.InternalLinks != 0 {
					t.Errorf("Expected 2 internal links, got %d", result.InternalLinks)
				}
				if result.ExternalLinks != 1 {
					t.Errorf("Expected 1 external link, got %d", result.ExternalLinks)
				}
				if result.HasLoginForm {
					t.Error("Do not Expected login form to be detected")
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
	config := &env.Config{
		LogLevel: "debug",
	}
	logger := logger.NewLogger(*config)
	analyzer := NewPageAnalyzer(logger, config)

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

	// Create a test server to serve the HTML
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(htmlContent))
	}))
	defer server.Close()

	// Test the analyzer
	ctx := context.Background()
	result := analyzer.Analyze(ctx, server.URL)

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
	config := &env.Config{
		LogLevel: "debug",
	}
	logger := logger.NewLogger(*config)
	analyzer := NewPageAnalyzer(logger, config)

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

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create a test server
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "text/html")
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(fmt.Sprintf(`<!DOCTYPE html><html><head><title>Test</title></head><body>%s</body></html>`, tc.html)))
			}))
			defer server.Close()

			// Test the analyzer
			ctx := context.Background()
			result := analyzer.Analyze(ctx, server.URL)

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
	config := &env.Config{
		LogLevel: "debug",
	}
	logger := logger.NewLogger(*config)
	analyzer := NewPageAnalyzer(logger, config)

	// Create test server that serves different content based on path
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")

		switch r.URL.Path {
		case "/":
			// Main page with various links
			html := `<!DOCTYPE html>
<html>
<head><title>Main Page</title></head>
<body>
    <a href="/internal1">Internal Link 1</a>
    <a href="/internal2">Internal Link 2</a>
    <a href="https://external1.com">External Link 1</a>
    <a href="https://external2.com">External Link 2</a>
    <a href="https://broken-link.com">Broken Link</a>
</body>
</html>`
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(html))
		case "/internal1":
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("<html><body>Internal 1</body></html>"))
		case "/internal2":
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("<html><body>Internal 2</body></html>"))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	// Test the analyzer
	ctx := context.Background()
	result := analyzer.Analyze(ctx, server.URL)

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

	// Note: Inaccessible links count depends on the actual availability of external sites
	// This is more of an integration test and may vary based on network conditions
}

// TestErrorHandling tests various error scenarios
func TestErrorHandling(t *testing.T) {
	config := &env.Config{
		LogLevel: "debug",
	}
	logger := logger.NewLogger(*config)
	analyzer := NewPageAnalyzer(logger, config)

	testCases := []struct {
		name           string
		serverBehavior func(w http.ResponseWriter, r *http.Request)
		expectedError  bool
		expectedStatus int
	}{
		{
			name: "Server returns 500 error",
			serverBehavior: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte("Internal Server Error"))
			},
			expectedError:  true,
			expectedStatus: 500,
		},
		{
			name: "Server returns 403 forbidden",
			serverBehavior: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusForbidden)
				w.Write([]byte("Forbidden"))
			},
			expectedError:  true,
			expectedStatus: 403,
		},
		{
			name: "Server returns malformed HTML",
			serverBehavior: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "text/html")
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("<html><body><unclosed>"))
			},
			expectedError:  true,
			expectedStatus: 0,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(tc.serverBehavior))
			defer server.Close()

			ctx := context.Background()
			result := analyzer.Analyze(ctx, server.URL)

			if tc.expectedError {
				if result.Error == "" {
					t.Error("Expected error but got none")
				}
			}

			if tc.expectedStatus != 0 && result.HTTPStatusCode != tc.expectedStatus {
				t.Errorf("Expected status code %d, got %d", tc.expectedStatus, result.HTTPStatusCode)
			}
		})
	}
}

// TestConcurrency tests concurrent link checking
func TestConcurrency(t *testing.T) {
	config := &env.Config{
		LogLevel: "debug",
	}
	logger := logger.NewLogger(*config)
	analyzer := NewPageAnalyzer(logger, config)

	// Create a test server that simulates slow responses
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")

		if r.URL.Path == "/" {
			// Main page with many external links
			html := `<!DOCTYPE html><html><head><title>Test</title></head><body>`
			for i := 0; i < 10; i++ {
				html += fmt.Sprintf(`<a href="https://external%d.com">External %d</a>`, i, i)
			}
			html += `</body></html>`

			w.WriteHeader(http.StatusOK)
			w.Write([]byte(html))
		} else {
			// Simulate slow response for external links
			time.Sleep(100 * time.Millisecond)
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("<html><body>External</body></html>"))
		}
	}))
	defer server.Close()

	// Test with timeout context
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	start := time.Now()
	result := analyzer.Analyze(ctx, server.URL)
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
}

// TestPerformance tests analyzer performance with large HTML
func TestPerformance(t *testing.T) {
	config := &env.Config{
		LogLevel: "debug",
	}
	logger := logger.NewLogger(*config)
	analyzer := NewPageAnalyzer(logger, config)

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

	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(html))
	}))
	defer server.Close()

	// Test performance
	ctx := context.Background()
	start := time.Now()
	result := analyzer.Analyze(ctx, server.URL)
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

	if !result.HasLoginForm {
		t.Error("Expected login form to be detected")
	}
}

// BenchmarkAnalyzer benchmarks the analyzer performance
func BenchmarkAnalyzer(b *testing.B) {
	config := &env.Config{
		LogLevel: "debug",
	}
	logger := logger.NewLogger(*config)
	analyzer := NewPageAnalyzer(logger, config)

	// Create test server with sample HTML
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`<!DOCTYPE html>
<html>
<head><title>Benchmark Test</title></head>
<body>
    <h1>Main Heading</h1>
    <h2>Sub Heading</h2>
    <a href="/internal">Internal Link</a>
    <a href="https://external.com">External Link</a>
    <form><input type="text"><input type="password"></form>
</body>
</html>`))
	}))
	defer server.Close()

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		analyzer.Analyze(ctx, server.URL)
	}
}
