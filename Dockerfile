# Build stage
FROM golang:1.24-alpine AS builder

# Set working directory
WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o web-analyzer ./cmd/web-analyzer

# Final stage
FROM alpine:latest

# Install ca-certificates for HTTPS requests
RUN apk --no-cache add ca-certificates

# Set working directory
WORKDIR /root/

# Copy the binary from builder stage
COPY --from=builder /app/web-analyzer .
# Copy app.env into the container
COPY --from=builder /app/cmd/web-analyzer/app.env .
# Copy templates into the image
COPY --from=builder /app/web/templates ./web/templates
# Expose port
EXPOSE 8080

# Run the application
CMD ["./web-analyzer"] 