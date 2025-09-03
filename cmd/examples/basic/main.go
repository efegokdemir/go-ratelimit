package main

import (
	"context"
	"fmt"
	"net/http"

	"github.com/dheerajk/go-ratelimit/pkg/logger"

	"github.com/dheerajk/go-ratelimit/pkg/ratelimit"
	"github.com/redis/go-redis/v9"
)

func main() {
	// Create a Redis client
	rdb := redis.NewClient(&redis.Options{
		Addr: "127.0.0.1:6379",
	})

	// Verify Redis connection
	if err := rdb.Ping(context.Background()).Err(); err != nil {
		logger.Log.Error(fmt.Sprintf("Redis connection failed: %v", err))
	}

	// Create rate limiting service with default options
	options := ratelimit.DefaultOptions(rdb)
	options.RequestLimit = 10       // 10 requests per minute
	options.ByteLimit = 1024 * 1024 // 1MB per minute

	service := ratelimit.New(options)

	// Create HTTP handlers
	mux := http.NewServeMux()

	// Regular API endpoint
	mux.HandleFunc("/api/hello", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "Hello, World!")
	})

	// Large response endpoint
	mux.HandleFunc("/api/large", func(w http.ResponseWriter, r *http.Request) {
		// Generate a 100KB response
		data := make([]byte, 100*1024)
		for i := range data {
			data[i] = byte('A' + (i % 26))
		}
		w.Write(data)
	})

	// Apply rate limiting middleware
	handler := service.Middleware()(mux)

	// Start server
	logger.Log.Info("Server running on :8080")
	logger.Log.Info("Try these endpoints:")
	logger.Log.Info("- GET /api/hello?client_id=user123 - Simple response")
	logger.Log.Info("- GET /api/large?client_id=user123 - Large response (100KB)")
	logger.Log.Info("Add client_id query parameter to identify clients")

	if err := http.ListenAndServe(":8080", handler); err != nil {
		logger.Log.Error(fmt.Sprintf("Failed to start server: %v", err))
	}
}
