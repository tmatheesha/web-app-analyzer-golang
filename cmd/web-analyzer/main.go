package main

import (
	"WebAppAnalyzer/config/env"
	"WebAppAnalyzer/config/logger"
	"WebAppAnalyzer/internal/server"
	"context"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	// Import docs for swagger initialization
	_ "WebAppAnalyzer/internal/docs"
)

// @title           Web Page Analyzer API
// @version         1.0
// @description     A web service that analyzes web pages and provides detailed information about their structure, links, and forms.
// @host      localhost:8080
// @BasePath  /api/v1
func main() {
	configPaths := []string{
		"cmd/web-analyzer/app.env",
		"./app.env",
	}
	correctConfigPaths := "."

	for _, path := range configPaths {
		matches, err := filepath.Glob(path)
		if err == nil && len(matches) > 0 {
			//correctConfigPaths = matches[0]
			correctConfigPaths = filepath.Dir(matches[0])
			break
		} else {
			log.Println("Config not found at path:", path)
		}
	}
	config, err := env.LoadConfig(correctConfigPaths)
	if err != nil {
		panic("Failed to load environment variables: " + err.Error())
	}
	log := logger.NewLogger(config)
	log.Infof("Starting web analyzer")

	srv := server.NewServer(log, &config)

	go func() {
		log.Infof("Starting web srv on port 8080")
		if err := srv.ListenAndServe(&config.Port); err != nil {
			log.Fatalf("Failed to start srv: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Infof("Shutting down server...")

	contextTimeOutConfigured, _ := strconv.Atoi(config.Port)

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(contextTimeOutConfigured)*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("Failed to shutdown server: %v", err)
	}
	log.Infof("Server exited gracefully")
}
