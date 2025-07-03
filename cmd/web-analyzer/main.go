package main

import (
	"WebAppAnalyzer/config/env"
	"WebAppAnalyzer/config/logger"
	"WebAppAnalyzer/internal/server"
	"context"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"
)

//TIP <p>To run your code, right-click the code and select <b>Run</b>.</p> <p>Alternatively, click
// the <icon src="AllIcons.Actions.Execute"/> icon in the gutter and select the <b>Run</b> menu item from here.</p>

func main() {
	config, err := env.LoadConfig(".")
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
