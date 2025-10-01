package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/redis/go-redis/v9"
	"gitlab-master.nvidia.com/kratos-dataverse-observability/go-ratelimit/pkg/config"
	"gitlab-master.nvidia.com/kratos-dataverse-observability/go-ratelimit/pkg/limiter"
	"gitlab-master.nvidia.com/kratos-dataverse-observability/go-ratelimit/pkg/middleware"
	"gitlab-master.nvidia.com/kratos-dataverse-observability/go-ratelimit/pkg/utils"
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
		fmt.Printf("Redis connection failed: %v. Make sure Redis is running at 127.0.0.1:6379\n", err)
		return
	}
	fmt.Println("Successfully connected to Redis")

	// Create a custom key function that uses client ID from query parameter
	keyByClientID := func(r *http.Request) string {
		clientID := r.URL.Query().Get("client_id")
		if clientID == "" {
			// Fallback to IP if no client ID provided
			return "ip:" + utils.ClientIP(r)
		}
		return "client:" + clientID
	}

	// Create a dynamic rate limiter with per-client configurations
	// Default policy: 10 requests and 10KB per minute
	dynamicLimiter := limiter.NewDynamicLimiter(
		rdb,
		"rl",
		10,    // Default: 10 requests per minute
		10240, // Default: 10KB per minute
		time.Minute,
		keyByClientID,
	)

	// Create API routes
	mux := http.NewServeMux()

	// Regular API endpoints
	mux.HandleFunc("/v1/thing", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "ok")
	})

	mux.HandleFunc("/v1/heavy", func(w http.ResponseWriter, r *http.Request) {
		// Generate a larger response (about 5KB)
		largeResponse := make([]byte, 5120)
		for i := range largeResponse {
			largeResponse[i] = byte('A' + (i % 26))
		}
		w.Write(largeResponse)
	})

	// Add example endpoint to show current client limits
	mux.HandleFunc("/v1/my-limits", func(w http.ResponseWriter, r *http.Request) {
		clientID := r.URL.Query().Get("client_id")
		if clientID == "" {
			http.Error(w, "client_id parameter required", http.StatusBadRequest)
			return
		}

		cfg, err := dynamicLimiter.GetClientConfig(r.Context(), clientID)
		if err != nil {
			http.Error(w, "Error retrieving limits: "+err.Error(), http.StatusInternalServerError)
			return
		}

		fmt.Fprintf(w, "Your rate limits:\n")
		fmt.Fprintf(w, "- Client ID: %s\n", clientID)
		fmt.Fprintf(w, "- Request limit: %d requests per %d seconds\n", cfg.RequestLimit, cfg.WindowSecs)
		fmt.Fprintf(w, "- Byte limit: %d bytes per %d seconds\n", cfg.ByteLimit, cfg.WindowSecs)
	})

	// Add endpoint to update client limits dynamically
	mux.HandleFunc("/admin/set-limits", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		clientID := r.URL.Query().Get("client_id")
		if clientID == "" {
			http.Error(w, "client_id parameter required", http.StatusBadRequest)
			return
		}

		// Parse request body
		var newConfig config.ClientConfig
		if err := json.NewDecoder(r.Body).Decode(&newConfig); err != nil {
			http.Error(w, "Invalid request body: "+err.Error(), http.StatusBadRequest)
			return
		}

		// Validate config
		if newConfig.RequestLimit <= 0 {
			http.Error(w, "request_limit must be positive", http.StatusBadRequest)
			return
		}
		if newConfig.ByteLimit <= 0 {
			http.Error(w, "byte_limit must be positive", http.StatusBadRequest)
			return
		}
		if newConfig.WindowSecs <= 0 {
			http.Error(w, "window_secs must be positive", http.StatusBadRequest)
			return
		}

		// Update client config
		if err := dynamicLimiter.SetClientConfig(r.Context(), clientID, newConfig); err != nil {
			http.Error(w, "Failed to update limits: "+err.Error(), http.StatusInternalServerError)
			return
		}

		// Return success
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "Successfully updated limits for client %s:\n", clientID)
		fmt.Fprintf(w, "- Request limit: %d requests per %d seconds\n", newConfig.RequestLimit, newConfig.WindowSecs)
		fmt.Fprintf(w, "- Byte limit: %d bytes per %d seconds\n", newConfig.ByteLimit, newConfig.WindowSecs)
	})

	// Add endpoint to reset client limits to defaults
	mux.HandleFunc("/admin/reset-limits", func(w http.ResponseWriter, r *http.Request) {
		clientID := r.URL.Query().Get("client_id")
		if clientID == "" {
			http.Error(w, "client_id parameter required", http.StatusBadRequest)
			return
		}

		// Reset client config
		if err := dynamicLimiter.ResetClientConfig(r.Context(), clientID); err != nil {
			http.Error(w, "Failed to reset limits: "+err.Error(), http.StatusInternalServerError)
			return
		}

		// Return success
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "Successfully reset limits for client %s to defaults\n", clientID)
	})

	// Apply dynamic rate limiting middleware
	rateLimitHandler := middleware.DynamicRateLimitMiddleware(dynamicLimiter)(mux)

	fmt.Println("Server running on :8085")
	fmt.Println("API endpoints:")
	fmt.Println("- GET /v1/thing?client_id=user123 - Simple endpoint")
	fmt.Println("- GET /v1/heavy?client_id=user123 - Returns ~5KB response")
	fmt.Println("- GET /v1/my-limits?client_id=user123 - Shows your current rate limits")
	fmt.Println("Admin endpoints:")
	fmt.Println("- POST /admin/set-limits?client_id=user123 - Set custom limits")
	fmt.Println("- GET /admin/reset-limits?client_id=user123 - Reset to default limits")
	fmt.Println("")
	fmt.Println("Example to set custom limits:")
	fmt.Println("curl -X POST -H \"Content-Type: application/json\" -d '{\"request_limit\":20,\"byte_limit\":20480,\"window_secs\":60}' http://localhost:8085/admin/set-limits?client_id=user123")

	if err := http.ListenAndServe(":8085", rateLimitHandler); err != nil {
		fmt.Printf("Failed to start server: %v\n", err)
	}
}
