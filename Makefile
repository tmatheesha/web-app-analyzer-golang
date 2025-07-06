.PHONY: build run test test-coverage clean docker-build docker-run docker-compose help deps fmt lint security

# Default target
help:
	@echo "Available commands:"
	@echo "  build         - Build the application"
	@echo "  run           - Run the application"
	@echo "  test          - Run tests"
	@echo "  test-coverage - Run tests with coverage report"
	@echo "  clean         - Clean build artifacts"
	@echo "  docker-build  - Build Docker image"
	@echo "  docker-run    - Run with Docker"
	@echo "  docker-compose- Run with docker-compose"
	@echo "  deps          - Install dependencies"
	@echo "  fmt           - Format code"
	@echo "  lint          - Lint code"
	@echo "  security      - Security check"

# Install dependencies
deps:
	go mod download
	go mod tidy

# Build the application
build: deps
	go build -o bin/web-analyzer cmd/web-analyzer/main.go

# Run the application
run: deps
	go run cmd/web-analyzer/main.go

# Run tests
test: deps
	go test -v ./...

# Run tests with coverage (targeting 70%+ coverage)
test-coverage: deps
	@echo "Running tests with coverage..."
	go test -v -coverprofile=coverage.out ./...
	@echo "Coverage report:"
	go tool cover -func=coverage.out
	@echo "Coverage percentage:"
	@go tool cover -func=coverage.out | tail -1 | awk '{print $$3}' | sed 's/%//' | awk '{if($$1 < 70) {print "WARNING: Coverage is below 70%: " $$1 "%"} else {print "Coverage is good: " $$1 "%"}}'
	@echo "Generating HTML coverage report..."
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report saved to coverage.html"

# Clean build artifacts
clean:
	rm -rf bin/
	rm -f coverage.out coverage.html
	rm -f *.test

# Build Docker image
docker-build:
	docker build -t web-analyzer .

# Run with Docker
docker-run:
	docker run -p 8080:8080 web-analyzer

# Run with docker-compose
docker-compose:
	docker-compose up --build

# Stop docker-compose
docker-compose-down:
	docker-compose down

# Format code
fmt:
	go fmt ./...

# Lint code
lint:
	@echo "Running golangci-lint..."
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run; \
	else \
		echo "golangci-lint not found. Install with: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest"; \
	fi

# Check for security vulnerabilities
security:
	@echo "Running security check..."
	@if command -v gosec >/dev/null 2>&1; then \
		gosec ./...; \
	else \
		echo "gosec not found. Install with: go install github.com/securecodewarrior/gosec/v2/cmd/gosec@latest"; \
	fi

# Run benchmarks
bench: deps
	go test -bench=. -benchmem ./...

# Run race detector
race: deps
	go test -race ./...

# Generate API documentation
docs:
	@echo "Generating API documentation..."
	@if command -v swag >/dev/null 2>&1; then \
		swag init -g cmd/web-analyzer/main.go; \
	else \
		echo "swag not found. Install with: go install github.com/swaggo/swag/cmd/swag@latest"; \
	fi

# Install development tools
install-tools:
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	go install github.com/securecodewarrior/gosec/v2/cmd/gosec@latest
	go install github.com/swaggo/swag/cmd/swag@latest

# Check code quality
quality: fmt lint security test-coverage

# Full build and test pipeline
pipeline: deps fmt lint security test-coverage build

# Development mode
dev: deps
	@echo "Starting development server..."
	@echo "Access the application at: http://localhost:8080"
	@echo "Health check: http://localhost:8080/health"
	@echo "Metrics: http://localhost:8080/metrics"
	go run cmd/web-analyzer/main.go

# Production build
prod: deps
	CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o bin/web-analyzer cmd/web-analyzer/main.go

# Show project info
info:
	@echo "Web Page Analyzer - Project Information"
	@echo "======================================"
	@echo "Go version: $(shell go version)"
	@echo "Module: $(shell go list -m)"
	@echo "Dependencies:"
	@go list -m all | head -10
	@echo ""
	@echo "Available endpoints:"
	@echo "  - Web UI: http://localhost:8080"
	@echo "  - Health: http://localhost:8080/health"
	@echo "  - Metrics: http://localhost:8080/metrics"
	@echo "  - API: http://localhost:8080/api/v1/analyze" 