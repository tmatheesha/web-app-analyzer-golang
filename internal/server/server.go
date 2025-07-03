package server

import (
	"WebAppAnalyzer/config/env"
	"WebAppAnalyzer/config/logger"
	"WebAppAnalyzer/internal/analyzer"
	"WebAppAnalyzer/internal/handlers"
	"context"
	"fmt"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"net/http"
	"time"
)

type Server struct {
	engine  *gin.Engine
	handler *handlers.Handler
	logger  *logger.Logger
	config  *env.Config
}

func NewServer(logger *logger.Logger, c *env.Config) *Server {
	pageAnalyzer := analyzer.NewPageAnalyzer(logger, c)

	if logger.Logger.GetLevel() == logrus.InfoLevel {
		gin.SetMode(gin.ReleaseMode)
	} else {
		gin.SetMode(gin.DebugMode)
	}

	engine := gin.New()

	handler := handlers.NewHandler(pageAnalyzer, logger, c)

	server := &Server{
		engine:  engine,
		handler: handler,
		logger:  logger,
		config:  c,
	}

	// Setup middleware and routes
	server.setupMiddleware()
	server.setupRoutes()

	return server
}

func (s *Server) ListenAndServe(port *string) error {
	s.logger.Logger.Info("Starting server")
	portAddr := fmt.Sprintf(":%s", *port)
	return s.engine.Run(portAddr)
}

func (s *Server) Shutdown(ctx context.Context) error {
	s.logger.Logger.Info("Shutting down server")
	port := fmt.Sprintf(":%s", s.config.Port)

	srv := &http.Server{
		Addr: port,
	}
	return srv.Shutdown(ctx)
}

func (s Server) setupMiddleware() {
	s.engine.Use(gin.Recovery())
	s.engine.Use(s.loggingMiddleware())

	s.engine.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	s.engine.Use(gin.Recovery())
	s.engine.Use(s.securityHeadersMiddleware())
}

func (s Server) securityHeadersMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-XSS-Protection", "1; mode=block")
		c.Header("X-Frame-Options", "DENY")
		c.Header("Content-Security-Policy", "default-src 'self'")
		c.Next()
	}
}

func (s Server) loggingMiddleware() gin.HandlerFunc {
	return gin.LoggerWithFormatter(func(param gin.LogFormatterParams) string {
		s.logger.Logger.WithFields(logrus.Fields{
			"method":     param.Method,
			"path":       param.Path,
			"status":     param.StatusCode,
			"latency":    param.Latency,
			"client_ip":  param.ClientIP,
			"user_agent": param.Request.UserAgent(),
			"error":      param.ErrorMessage,
			"timestamp":  param.TimeStamp.Format(time.RFC3339),
		}).Info("Request: %s %s", param.Method, param.Path)
		return ""
	})
}

func (s Server) setupRoutes() {

	s.engine.LoadHTMLGlob("../../web/templates/*.html")
	s.engine.Static("../../static", "web/static")
	s.engine.GET("/health", s.handler.HealthCheck)

	api := s.engine.Group("/api/v1")
	{
		api.POST("/analyze", s.handler.AnalyzePage)
		api.GET("/analyze", s.handler.AnalyzePage)
	}

	//Web routes
	s.engine.GET("/", s.handler.Index)
	s.engine.POST("/analyze", s.handler.AnalyzePageForm)

	s.engine.NoRoute(s.handler.NotFound)
	s.engine.NoMethod(s.handler.MethodNotAllowed)

}
