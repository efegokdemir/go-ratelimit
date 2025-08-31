package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/dheerajk/go-ratelimit/pkg/api"
	"github.com/dheerajk/go-ratelimit/pkg/limiter"
	"github.com/dheerajk/go-ratelimit/pkg/middleware"
	"github.com/dheerajk/go-ratelimit/pkg/ratelimit"
	"github.com/dheerajk/go-ratelimit/pkg/utils"
	"github.com/redis/go-redis/v9"
)

func main() {
	// Use a Redis Cluster or a single node; tune PoolSize for high QPS.
	rdb := redis.NewClient(&redis.Options{
		Addr:         "127.0.0.1:6379",
		ReadTimeout:  200 * time.Millisecond,
		WriteTimeout: 200 * time.Millisecond,
		PoolSize:     64,
	})

	// Health check Redis up-front
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := rdb.Ping(ctx).Err(); err != nil {
		log.Fatalf("Redis connection failed: %v. Make sure Redis is running at 127.0.0.1:6379", err)
	}
	log.Println("Successfully connected to Redis")

	// Create a dynamic rate limiter with per-client configurations
	// Default policy: 10 requests and 1MB per minute
	dynamicLimiter := limiter.NewDynamicLimiter(
		rdb,
		"rl",
		10,      // Default: 10 requests per minute
		1048576, // Default: 1MB (1048576 bytes) per minute
		time.Minute,
		utils.KeyByAPIKeyAndRoute,
	)

	// Create API routes
	mux := http.NewServeMux()

	// Regular API endpoints
	mux.HandleFunc("/v1/thing", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "ok")
	})
	mux.HandleFunc("/v1/heavy", func(w http.ResponseWriter, r *http.Request) {
		// Generate a larger response (about 10KB)
		largeResponse := make([]byte, 10240)
		for i := range largeResponse {
			largeResponse[i] = byte('A' + (i % 26))
		}
		w.Write(largeResponse)
	})
	mux.HandleFunc("/v1/veryheavy", func(w http.ResponseWriter, r *http.Request) {
		// Generate a very large response (about 500KB)
		largeResponse := make([]byte, 512000)
		for i := range largeResponse {
			largeResponse[i] = byte('A' + (i % 26))
		}
		w.Write(largeResponse)
	})

	// Add client configuration endpoint
	configHandler := api.NewConfigHandler(dynamicLimiter)
	configHandler.RegisterRoutes(mux)

	// Add example endpoint to show current client limits
	mux.HandleFunc("/v1/my-limits", func(w http.ResponseWriter, r *http.Request) {
		apiKey := r.Header.Get("X-API-Key")
		if apiKey == "" {
			http.Error(w, "API key required", http.StatusBadRequest)
			return
		}

		cfg, err := dynamicLimiter.GetClientConfig(r.Context(), apiKey)
		if err != nil {
			http.Error(w, "Error retrieving limits: "+err.Error(), http.StatusInternalServerError)
			return
		}

		fmt.Fprintf(w, "Your rate limits:\n")
		fmt.Fprintf(w, "- Request limit: %d requests per %d seconds\n", cfg.RequestLimit, cfg.WindowSecs)
		fmt.Fprintf(w, "- Byte limit: %d bytes per %d seconds\n", cfg.ByteLimit, cfg.WindowSecs)
	})

	// Create a service to use the new API
	service := ratelimit.NewFromLimiter(dynamicLimiter)

	// Apply selective rate limiting middleware - only rate limit heavy endpoints
	heavyPathMatcher := middleware.MatchAny(
		middleware.MatchPathExact("/v1/heavy"),
		middleware.MatchPathExact("/v1/veryheavy"),
	)

	// Apply rate limiting only to specific paths
	rateLimitHandler := service.SelectiveMiddleware(heavyPathMatcher)(mux)

	log.Println("Server running on :8082")
	log.Println("API endpoints:")
	log.Println("- GET /v1/thing - Simple endpoint")
	log.Println("- GET /v1/heavy - Returns ~10KB response")
	log.Println("- GET /v1/veryheavy - Returns ~500KB response")
	log.Println("- GET /v1/my-limits - Shows your current rate limits")
	log.Println("Admin endpoints:")
	log.Println("- GET /admin/config/{clientID} - Get client's rate limit config")
	log.Println("- PUT /admin/config/{clientID} - Update client's rate limit config")
	log.Println("- DELETE /admin/config/{clientID} - Reset client to default config")

	if err := http.ListenAndServe(":8082", rateLimitHandler); err != nil {
		log.Fatal(err)
	}
}
