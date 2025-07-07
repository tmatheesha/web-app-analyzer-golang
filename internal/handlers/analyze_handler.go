package handlers

import (
	"github.com/gin-gonic/gin"
	"net/http"
	"time"
)

// Handler struct contains dependencies for handling requests
// @Summary: Handler struct for web application
// @Description: Contains methods for handling requests, including health checks and page analysis
// @Tags: handlers
// @Accept: json
// @Produce: json
// @Param: url query string true "URL to analyze"
// @Success: 200 {object} models.AnalysisResult
// @Failure: 400 {object} APIError "Bad Request"
// @Failure: 500 {object} APIError "Internal Server Error"
// @Router: /analyze [get]
// @Router: /analyze [post]
func (h *Handler) AnalyzePage(c *gin.Context) {
	startTime := time.Now()

	clientIP := c.ClientIP()
	userAgent := c.GetHeader("User-Agent")

	h.logger.WithRequest(c.Request.Method, c.Request.URL.Path, clientIP).
		WithField("user_agent", userAgent).
		Info("Analysis request received")

	url := c.Query("url")
	if url == "" {
		h.logger.Error(c, http.StatusBadRequest, "URL parameter is required")
		c.HTML(http.StatusBadRequest, "index.html", gin.H{
			"title": h.config.WebAppTitle,
			"error": "URL parameter is required",
		})
		return
	}

	// Perform analysis
	ctx := c.Request.Context()
	result := h.analyzer.Analyze(ctx, url)

	// Set analysis time
	result.AnalysisTime = time.Since(startTime).String()

	c.JSON(http.StatusOK, result)

	h.logger.WithField("duration", time.Since(startTime)).
		Info("Analysis completed")

}
