# Build stage
FROM golang:1.21-alpine AS builder

# Install git and ca-certificates
RUN apk add --no-cache git ca-certificates

# Set working directory
WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the admin server
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o ratelimit-server ./cmd/server

# Final stage
FROM alpine:latest

# Install ca-certificates for HTTPS
RUN apk --no-cache add ca-certificates

WORKDIR /root/

# Copy the binary from builder
COPY --from=builder /app/ratelimit-server .

# Copy default config
COPY --from=builder /app/configs/default.yaml ./configs/

# Expose ports
# 8080 - gRPC port
# 8081 - HTTP/REST gateway port  
# 8082 - Admin/metrics port
EXPOSE 8080 8081 8082

# Default environment variables
ENV RATELIMIT_REDIS_ADDR="redis:6379" \
    RATELIMIT_REDIS_DB="0" \
    RATELIMIT_DEFAULT_REQUEST_LIMIT="100" \
    RATELIMIT_DEFAULT_BYTE_LIMIT="1048576" \
    RATELIMIT_WINDOW_SECONDS="60" \
    RATELIMIT_LOG_LEVEL="info"

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8082/health || exit 1

# Run the server
ENTRYPOINT ["./ratelimit-server"]
CMD ["server", "--config", "/root/configs/default.yaml"]
