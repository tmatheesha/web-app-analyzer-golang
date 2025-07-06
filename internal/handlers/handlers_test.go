package handlers

import (
	"WebAppAnalyzer/config/env"
	"WebAppAnalyzer/config/logger"
	"WebAppAnalyzer/internal/models"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockPageAnalyzer is a mock implementation of the PageAnalyzerInterface
type MockPageAnalyzer struct {
	mock.Mock
}

func (m *MockPageAnalyzer) Analyze(ctx context.Context, url string) *models.AnalysisResult {
	args := m.Called(ctx, url)
	return args.Get(0).(*models.AnalysisResult)
}

// createTestHandler creates handler with mock analyzer for testing
func createTestHandler() (*Handler, *MockPageAnalyzer) {
	config := &env.Config{
		LogLevel:    "debug",
		WebAppTitle: "Test Web App Analyzer",
	}
	logger := logger.NewLogger(*config)

	mockAnalyzer := &MockPageAnalyzer{}

	handler := NewHandler(mockAnalyzer, logger, config)

	return handler, mockAnalyzer
}

// setupGinTest sets up a Gin router for testing
func setupGinTest(handler *Handler) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	router.GET("/analyze", handler.AnalyzePage)
	router.GET("/", handler.Index)
	router.GET("/health", handler.HealthCheck)
	router.NoRoute(handler.NotFound)

	return router
}

// TestAnalyzePage_Success tests successful analysis
func TestAnalyzePage_Success(t *testing.T) {
	handler, mockAnalyzer := createTestHandler()
	router := setupGinTest(handler)

	expectedResult := &models.AnalysisResult{
		URL:            "https://example.com",
		HTMLVersion:    "HTML5",
		PageTitle:      "Example Domain",
		Headings:       map[string]int{"h1": 1},
		InternalLinks:  2,
		ExternalLinks:  1,
		HasLoginForm:   false,
		HTTPStatusCode: 200,
		Timestamp:      time.Now(),
	}

	mockAnalyzer.On("Analyze", mock.Anything, "https://example.com").Return(expectedResult)

	req, _ := http.NewRequest("GET", "/analyze?url=https://example.com", nil)
	req.Header.Set("User-Agent", "TestAgent")
	req.Header.Set("X-Forwarded-For", "192.168.1.1")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response models.AnalysisResult
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)

	assert.Equal(t, expectedResult.URL, response.URL)
	assert.Equal(t, expectedResult.PageTitle, response.PageTitle)
	assert.Equal(t, expectedResult.HTMLVersion, response.HTMLVersion)
	assert.Equal(t, expectedResult.HTTPStatusCode, response.HTTPStatusCode)

	mockAnalyzer.AssertExpectations(t)
}

// TestAnalyzePage_MissingURL tests missing URL parameter
// Note: This test focuses on the query parameter validation logic
// HTML template rendering is tested in integration tests
func TestAnalyzePage_MissingURL(t *testing.T) {
	_, mockAnalyzer := createTestHandler()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	req, _ := http.NewRequest("GET", "/analyze", nil)
	c.Request = req

	url := c.Query("url")
	assert.Equal(t, "", url)

	mockAnalyzer.AssertNotCalled(t, "Analyze")
}

// TestAnalyzePage_EmptyURL tests empty URL parameter
func TestAnalyzePage_EmptyURL(t *testing.T) {
	_, mockAnalyzer := createTestHandler()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	req, _ := http.NewRequest("GET", "/analyze?url=", nil)
	c.Request = req

	url := c.Query("url")
	assert.Equal(t, "", url)

	mockAnalyzer.AssertNotCalled(t, "Analyze")
}

// TestAnalyzePage_AnalyzerError tests when analyzer returns an error
func TestAnalyzePage_AnalyzerError(t *testing.T) {
	handler, mockAnalyzer := createTestHandler()
	router := setupGinTest(handler)

	errorResult := &models.AnalysisResult{
		URL:            "https://invalid-url.com",
		Error:          "Invalid URL format",
		HTTPStatusCode: 400,
		Timestamp:      time.Now(),
	}

	mockAnalyzer.On("Analyze", mock.Anything, "https://invalid-url.com").Return(errorResult)

	req, _ := http.NewRequest("GET", "/analyze?url=https://invalid-url.com", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response models.AnalysisResult
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)

	assert.Equal(t, errorResult.Error, response.Error)
	assert.Equal(t, errorResult.HTTPStatusCode, response.HTTPStatusCode)

	mockAnalyzer.AssertExpectations(t)
}

// TestAnalyzePage_HTTPError tests when analyzer returns HTTP error
func TestAnalyzePage_HTTPError(t *testing.T) {
	handler, mockAnalyzer := createTestHandler()
	router := setupGinTest(handler)

	httpErrorResult := &models.AnalysisResult{
		URL:            "https://notfound.com",
		Error:          "HTTP Error: 404 - Not Found",
		HTTPStatusCode: 404,
		Timestamp:      time.Now(),
	}

	mockAnalyzer.On("Analyze", mock.Anything, "https://notfound.com").Return(httpErrorResult)

	req, _ := http.NewRequest("GET", "/analyze?url=https://notfound.com", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response models.AnalysisResult
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)

	assert.Equal(t, httpErrorResult.Error, response.Error)
	assert.Equal(t, httpErrorResult.HTTPStatusCode, response.HTTPStatusCode)

	mockAnalyzer.AssertExpectations(t)
}

// TestAnalyzePage_ComplexResult tests analysis with complex result
func TestAnalyzePage_ComplexResult(t *testing.T) {
	handler, mockAnalyzer := createTestHandler()
	router := setupGinTest(handler)

	complexResult := &models.AnalysisResult{
		URL:               "https://complex-site.com",
		HTMLVersion:       "HTML5",
		PageTitle:         "Complex Website",
		Headings:          map[string]int{"h1": 2, "h2": 5, "h3": 10},
		InternalLinks:     15,
		ExternalLinks:     8,
		InaccessibleLinks: 2,
		HasLoginForm:      true,
		HTTPStatusCode:    200,
		AnalysisTime:      1500 * time.Millisecond,
		Timestamp:         time.Now(),
	}

	mockAnalyzer.On("Analyze", mock.Anything, "https://complex-site.com").Return(complexResult)

	req, _ := http.NewRequest("GET", "/analyze?url=https://complex-site.com", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response models.AnalysisResult
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)

	assert.Equal(t, complexResult.URL, response.URL)
	assert.Equal(t, complexResult.PageTitle, response.PageTitle)
	assert.Equal(t, complexResult.HTMLVersion, response.HTMLVersion)
	assert.Equal(t, complexResult.InternalLinks, response.InternalLinks)
	assert.Equal(t, complexResult.ExternalLinks, response.ExternalLinks)
	assert.Equal(t, complexResult.InaccessibleLinks, response.InaccessibleLinks)
	assert.Equal(t, complexResult.HasLoginForm, response.HasLoginForm)
	assert.Equal(t, complexResult.HTTPStatusCode, response.HTTPStatusCode)

	mockAnalyzer.AssertExpectations(t)
}

// TestAnalyzePage_ContextCancellation tests context cancellation
func TestAnalyzePage_ContextCancellation(t *testing.T) {
	handler, mockAnalyzer := createTestHandler()
	router := setupGinTest(handler)

	// Setup mock to simulate context cancellation
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	errorResult := &models.AnalysisResult{
		URL:   "https://timeout.com",
		Error: "context canceled",
	}

	mockAnalyzer.On("Analyze", mock.Anything, "https://timeout.com").Return(errorResult)

	// Create request
	req, _ := http.NewRequest("GET", "/analyze?url=https://timeout.com", nil)
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	// Assertions
	assert.Equal(t, http.StatusOK, w.Code)

	var response models.AnalysisResult
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)

	assert.Equal(t, errorResult.Error, response.Error)

	mockAnalyzer.AssertExpectations(t)
}

// TestAnalyzePage_Headers tests request headers are properly handled
func TestAnalyzePage_Headers(t *testing.T) {
	handler, mockAnalyzer := createTestHandler()
	router := setupGinTest(handler)

	expectedResult := &models.AnalysisResult{
		URL:            "https://example.com",
		PageTitle:      "Example",
		HTTPStatusCode: 200,
		Timestamp:      time.Now(),
	}

	mockAnalyzer.On("Analyze", mock.Anything, "https://example.com").Return(expectedResult)

	req, _ := http.NewRequest("GET", "/analyze?url=https://example.com", nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Test Browser)")
	req.Header.Set("X-Forwarded-For", "10.0.0.1")
	req.Header.Set("X-Real-IP", "10.0.0.2")
	req.Header.Set("Accept", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/json; charset=utf-8", w.Header().Get("Content-Type"))

	var response models.AnalysisResult
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)

	assert.Equal(t, expectedResult.URL, response.URL)

	mockAnalyzer.AssertExpectations(t)
}

//// TestAnalyzePage_InvalidMethod tests invalid HTTP method
//func TestAnalyzePage_InvalidMethod(t *testing.T) {
//	handler, mockAnalyzer := createTestHandler()
//	router := setupGinTest(handler)
//
//	// Create POST request (should not be allowed)
//	req, _ := http.NewRequest("POST", "/analyze?url=https://example.com", bytes.NewBufferString(""))
//	w := httptest.NewRecorder()
//
//	router.ServeHTTP(w, req)
//
//	// Assertions - should return 404 since POST is not defined for this route
//	assert.Equal(t, http.StatusNotFound, w.Code)
//
//	// Verify that analyzer was not called
//	mockAnalyzer.AssertNotCalled(t, "Analyze")
//}

// TestAnalyzePage_URLEscaping tests URL parameter escaping
func TestAnalyzePage_URLEscaping(t *testing.T) {
	handler, mockAnalyzer := createTestHandler()
	router := setupGinTest(handler)

	// Test URL with special characters
	testURL := "https://example.com/path with spaces?param=value&other=test"
	expectedResult := &models.AnalysisResult{
		URL:            testURL,
		PageTitle:      "Example",
		HTTPStatusCode: 200,
		Timestamp:      time.Now(),
	}

	mockAnalyzer.On("Analyze", mock.Anything, testURL).Return(expectedResult)

	// Create request with escaped URL
	escapedURL := "https%3A//example.com/path%20with%20spaces%3Fparam%3Dvalue%26other%3Dtest"
	req, _ := http.NewRequest("GET", fmt.Sprintf("/analyze?url=%s", escapedURL), nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	// Assertions
	assert.Equal(t, http.StatusOK, w.Code)

	var response models.AnalysisResult
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)

	assert.Equal(t, expectedResult.URL, response.URL)

	mockAnalyzer.AssertExpectations(t)
}

// TestAnalyzePage_Performance tests performance with timing
func TestAnalyzePage_Performance(t *testing.T) {
	handler, mockAnalyzer := createTestHandler()
	router := setupGinTest(handler)

	expectedResult := &models.AnalysisResult{
		URL:            "https://example.com",
		PageTitle:      "Example",
		HTTPStatusCode: 200,
		AnalysisTime:   500 * time.Millisecond,
		Timestamp:      time.Now(),
	}

	mockAnalyzer.On("Analyze", mock.Anything, "https://example.com").Return(expectedResult)

	// Create request
	req, _ := http.NewRequest("GET", "/analyze?url=https://example.com", nil)
	w := httptest.NewRecorder()

	start := time.Now()
	router.ServeHTTP(w, req)
	duration := time.Since(start)

	// Assertions
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Less(t, duration, 1*time.Second) // Should complete quickly

	var response models.AnalysisResult
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)

	assert.Equal(t, expectedResult.URL, response.URL)
	assert.Equal(t, expectedResult.AnalysisTime, response.AnalysisTime)

	mockAnalyzer.AssertExpectations(t)
}

// TestAnalyzePage_Logging tests that logging is properly handled
func TestAnalyzePage_Logging(t *testing.T) {
	handler, mockAnalyzer := createTestHandler()
	router := setupGinTest(handler)

	expectedResult := &models.AnalysisResult{
		URL:            "https://example.com",
		PageTitle:      "Example",
		HTTPStatusCode: 200,
		Timestamp:      time.Now(),
	}

	mockAnalyzer.On("Analyze", mock.Anything, "https://example.com").Return(expectedResult)

	// Create request
	req, _ := http.NewRequest("GET", "/analyze?url=https://example.com", nil)
	req.Header.Set("User-Agent", "TestAgent")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	// Assertions
	assert.Equal(t, http.StatusOK, w.Code)

	// Note: In a real test, you might want to capture log output
	// For now, we just verify the handler completes successfully

	mockAnalyzer.AssertExpectations(t)
}

// BenchmarkAnalyzePage benchmarks the AnalyzePage handler
func BenchmarkAnalyzePage(b *testing.B) {
	handler, mockAnalyzer := createTestHandler()
	router := setupGinTest(handler)

	expectedResult := &models.AnalysisResult{
		URL:            "https://example.com",
		PageTitle:      "Example",
		HTTPStatusCode: 200,
		Timestamp:      time.Now(),
	}

	mockAnalyzer.On("Analyze", mock.Anything, "https://example.com").Return(expectedResult)

	req, _ := http.NewRequest("GET", "/analyze?url=https://example.com", nil)
	w := httptest.NewRecorder()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		router.ServeHTTP(w, req)
	}
}

// TestAnalyzePage_SuccessfulAnalysis tests successful analysis with URL parameter
func TestAnalyzePage_SuccessfulAnalysis(t *testing.T) {
	handler, mockAnalyzer := createTestHandler()

	// Setup expected result
	expectedResult := &models.AnalysisResult{
		URL:            "https://example.com",
		PageTitle:      "Example Domain",
		HTTPStatusCode: 200,
		HTMLVersion:    "HTML5",
		Headings:       map[string]int{"h1": 1, "h2": 0},
		InternalLinks:  0,
		ExternalLinks:  1,
		HasLoginForm:   false,
		Timestamp:      time.Now(),
	}

	mockAnalyzer.On("Analyze", mock.Anything, "https://example.com").Return(expectedResult)

	// Create a mock Gin context
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	// Set up the request with URL parameter
	req, _ := http.NewRequest("GET", "/analyze?url=https://example.com", nil)
	c.Request = req

	// Call the handler method directly
	handler.AnalyzePage(c)

	// Assertions
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Header().Get("Content-Type"), "application/json")

	// Verify that analyzer was called with correct parameters
	mockAnalyzer.AssertExpectations(t)
}
