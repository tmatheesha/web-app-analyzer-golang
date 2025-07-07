package handlers

import (
	"WebAppAnalyzer/config/env"
	"WebAppAnalyzer/config/logger"
	"WebAppAnalyzer/internal/models"
	"context"
	"github.com/gin-gonic/gin"
	"net/http"
	"time"
)

// PageAnalyzerInterface defines the interface for page analysis
type PageAnalyzerInterface interface {
	Analyze(ctx context.Context, url string) *models.AnalysisResult
}

type Handler struct {
	analyzer PageAnalyzerInterface
	logger   *logger.Logger
	config   *env.Config
}

type APIError struct {
	Error   string `json:"error"`
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// NewHandler creates a new handler instance
func NewHandler(pageAnalyzer PageAnalyzerInterface, logger *logger.Logger, c *env.Config) *Handler {
	return &Handler{
		analyzer: pageAnalyzer,
		logger:   logger,
		config:   c,
	}
}

// HealthCheck handles health check requests
func (h *Handler) HealthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":    "healthy",
		"timestamp": time.Now().UTC(),
		"service":   "web-analyzer",
		"version":   "1.0.0",
	})
}

func (h *Handler) Index(c *gin.Context) {
	c.HTML(http.StatusOK, "index.html", gin.H{
		"title": h.config.WebAppTitle,
	})
}

func (h *Handler) NotFound(c *gin.Context) {
	h.logger.WithRequest(c.Request.Method, c.Request.URL.Path, c.ClientIP()).
		Warn("404 - Page not found")

	c.HTML(http.StatusNotFound, "404.html", gin.H{
		"title": "Page Not Found",
		"path":  c.Request.URL.Path,
	})
}

// MethodNotAllowed handles 405 errors
func (h *Handler) MethodNotAllowed(c *gin.Context) {
	h.logger.WithRequest(c.Request.Method, c.Request.URL.Path, c.ClientIP()).
		Warn("405 - Method not allowed")

	c.JSON(http.StatusMethodNotAllowed, APIError{
		Error:   "Method Not Allowed",
		Code:    http.StatusMethodNotAllowed,
		Message: "The requested method is not allowed for this endpoint",
	})
}

// AnalyzePageForm handles form submissions for page analysis
// @Summary Analyze a web page from form submission
// @Description Analyzes a web page based on the URL provided in the form submission
// @Tags Analysis
// @Accept  application/x-www-form-urlencoded
// @Produce  html
// @Param url formData string true "URL of the web page to analyze"
// @Success 200 {object} models.AnalysisResult
// @Failure 400 {object} APIError "Bad Request"
// @Failure 500 {object} APIError "Internal Server Error"
// @Router /analyze [post]
func (h *Handler) AnalyzePageForm(c *gin.Context) {
	clientIP := c.ClientIP()

	h.logger.WithRequest(c.Request.Method, c.Request.URL.Path, clientIP).
		Info("Form analysis request received")

	// Get URL from form
	url := c.PostForm("url")
	if url == "" {
		h.logger.Error("Empty URL provided in form")
		c.HTML(http.StatusOK, "index.html", gin.H{
			"title": "Web Page Analyzer",
			"error": "URL is required",
		})
		return
	}

	ctx := c.Request.Context()
	startTime := time.Now()
	result := h.analyzer.Analyze(ctx, url)

	result.AnalysisTime = time.Since(startTime).String()

	h.logger.WithField("success", result.IsSuccessful()).
		Info("Form analysis completed")

	c.HTML(http.StatusOK, "index.html", gin.H{
		"title":  "Web Page Analyzer",
		"result": result,
	})
}
