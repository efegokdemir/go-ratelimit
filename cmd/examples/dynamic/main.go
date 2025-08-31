package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

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
		log.Fatalf("Redis connection failed: %v", err)
	}

	// Create rate limiting service with custom options
	options := ratelimit.Options{
		Redis:        rdb,
		Prefix:       "example",
		RequestLimit: 5,          // 5 requests per minute by default
		ByteLimit:    512 * 1024, // 512KB per minute by default
		Window:       time.Minute,
		KeyFunc:      ratelimit.KeyByAPIKeyAndRoute,
		AdminKey:     "my-admin-key",
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
		apiKey := r.Header.Get("X-API-Key")
		if apiKey == "" {
			http.Error(w, "API key required", http.StatusBadRequest)
			return
		}

		cfg, err := service.GetLimiter().GetClientConfig(r.Context(), apiKey)
		if err != nil {
			http.Error(w, "Error retrieving limits: "+err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
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
	log.Println("Server running on :8080")
	log.Println("Try these endpoints:")
	log.Println("- GET /api/hello - Simple response")
	log.Println("- GET /api/large - Large response (100KB)")
	log.Println("- GET /api/my-limits - View your rate limits")
	log.Println("Admin endpoints (use X-Admin-Key: my-admin-key):")
	log.Println("- GET /admin/config/{clientID} - Get client's rate limit config")
	log.Println("- PUT /admin/config/{clientID} - Update client's rate limit config")
	log.Println("- DELETE /admin/config/{clientID} - Reset client to default config")
	log.Println("Try with X-API-Key: premium-client to see higher limits")

	if err := http.ListenAndServe(":8080", handler); err != nil {
		log.Fatal(err)
	}
}
