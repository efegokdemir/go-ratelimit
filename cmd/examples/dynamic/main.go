package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/dheerajk/go-ratelimit/pkg/logger"
	"github.com/dheerajk/go-ratelimit/pkg/ratelimit"
	"github.com/dheerajk/go-ratelimit/pkg/utils"
	"github.com/redis/go-redis/v9"
	"github.com/uptrace/opentelemetry-go-extra/otelzap"
	"go.uber.org/zap"
)

func main() {
	// Initialize logger with default configuration
	zapLogger, err := zap.NewDevelopment()
	if err != nil {
		log.Fatalf("Failed to create logger: %v", err)
	}
	logger.Log = otelzap.New(zapLogger)

	// Create a Redis client
	rdb := redis.NewClient(&redis.Options{
		Addr: "127.0.0.1:6379",
	})

	// Verify Redis connection
	if err := rdb.Ping(context.Background()).Err(); err != nil {
		logger.Log.Error(fmt.Sprintf("Redis connection failed: %v", err))
	} else {
		logger.Log.Info("Connected to Redis successfully")
	}

	// Create a custom key function that would normally be implemented by the API
	// In a real implementation, this would extract client ID from auth context
	// after proper authentication and authorization
	keyByClientID := func(r *http.Request) string {
		// This is just a placeholder - in production, the API would:
		// 1. Perform authentication (JWT, OAuth, API key, etc.)
		// 2. Perform authorization
		// 3. Extract the authenticated client ID from the auth context

		// For demo purposes, we're still using the query param
		clientID := r.URL.Query().Get("client_id")
		if clientID == "" {
			// Fallback to IP if no client ID provided
			return "ip:" + utils.ClientIP(r)
		}
		return "client:" + clientID
	}

	// Create rate limiting service with custom options
	options := ratelimit.Options{
		Redis:        rdb,
		Prefix:       "example",
		RequestLimit: 5,          // 5 requests per minute by default
		ByteLimit:    512 * 1024, // 512KB per minute by default
		Window:       time.Minute,
		KeyFunc:      keyByClientID,
	}

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

	// Endpoint to view your current limits
	mux.HandleFunc("/api/my-limits", func(w http.ResponseWriter, r *http.Request) {
		clientID := r.URL.Query().Get("client_id")
		if clientID == "" {
			http.Error(w, "client_id parameter required", http.StatusBadRequest)
			return
		}

		cfg, err := service.GetLimiter().GetClientConfig(r.Context(), clientID)
		if err != nil {
			http.Error(w, "Error retrieving limits: "+err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"client_id":     clientID,
			"request_limit": cfg.RequestLimit,
			"byte_limit":    cfg.ByteLimit,
			"window_secs":   cfg.WindowSecs,
		})
	})

	// Register the configuration API
	service.RegisterConfigAPI(mux)

	// Set a custom configuration for a premium client
	service.SetClientConfig("premium-client", 100, 5*1024*1024, 60)

	// Apply rate limiting middleware
	handler := service.Middleware()(mux)

	// Start server
	logger.Log.Info("Server running on :8082")
	logger.Log.Info("Try these endpoints:")
	logger.Log.Info("- GET /api/hello?client_id=user123 - Simple response")
	logger.Log.Info("- GET /api/large?client_id=user123 - Large response (100KB)")
	logger.Log.Info("- GET /api/my-limits?client_id=user123 - View your rate limits")
	logger.Log.Info("- GET /admin/config/{clientID} - Get client's rate limit config")
	logger.Log.Info("- PUT /admin/config/{clientID} - Update client's rate limit config")
	logger.Log.Info("- DELETE /admin/config/{clientID} - Reset client to default config")
	logger.Log.Info("Try with client_id=premium-client to see higher limits")

	if err := http.ListenAndServe(":8082", handler); err != nil {
		logger.Log.Error(fmt.Sprintf("Failed to start server: %v", err))
	}
}
