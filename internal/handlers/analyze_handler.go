package handlers

import (
	"github.com/gin-gonic/gin"
	"net/http"
	"time"
)

func (h *Handler) AnalyzePage(c *gin.Context) {
	startTime := time.Now()

	// Get client IP for logging
	clientIP := c.ClientIP()
	userAgent := c.GetHeader("User-Agent")

	// Log request
	h.logger.WithRequest(c.Request.Method, c.Request.URL.Path, clientIP).
		WithField("user_agent", userAgent).
		Info("Analysis request received")

	// Parse request
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

	c.JSON(http.StatusOK, result)

	h.logger.WithField("duration", time.Since(startTime)).
		Info("Analysis completed")

}
