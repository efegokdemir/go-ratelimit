package middleware

import (
	"fmt"
	"net/http"

	"github.com/dheerajk/go-ratelimit/pkg/logger"

	"github.com/dheerajk/go-ratelimit/pkg/limiter"
)

// DynamicRateLimitMiddleware enforces both request count and byte-based rate limiting with dynamic configs
func DynamicRateLimitMiddleware(dl *limiter.DynamicLimiter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// First check if the request is allowed based on request count
			allowed, remaining, reset, err := dl.AllowRequest(r.Context(), r)
			if err != nil {
				http.Error(w, "rate limiter error", http.StatusInternalServerError)
				return
			}

			// Get client ID for retrieving config
			clientID := extractClientID(r)
			cfg, _ := dl.GetClientConfig(r.Context(), clientID)

			// Set request count rate limit headers
			logger.Log.Info(fmt.Sprintf("Setting request count rate limit headers for client %s", clientID))
			w.Header().Set("RateLimit-Request-Limit", fmt.Sprintf("%d", cfg.RequestLimit))
			w.Header().Set("RateLimit-Request-Remaining", fmt.Sprintf("%d", remaining))
			w.Header().Set("RateLimit-Request-Reset", fmt.Sprintf("%d", reset))

			if !allowed {
				logger.Log.Info(fmt.Sprintf("Request rate limit exceeded for client %s", clientID))
				w.Header().Set("Retry-After", fmt.Sprintf("%d", reset))
				http.Error(w, "request rate limit exceeded", http.StatusTooManyRequests)
				return
			}

			// Create a byte counter wrapper for the response writer
			bcw := NewByteCounterWriter(w)

			// Call the next handler with our wrapped response writer
			next.ServeHTTP(bcw, r)

			// Only count successful responses
			if bcw.StatusCode < 400 {
				logger.Log.Info(fmt.Sprintf("Recording bytes used for client %s", clientID))
				dl.RecordBytes(r.Context(), r, bcw.BytesWritten)
			}
		})
	}
}

// extractClientID extracts a client ID from the request for config lookup
// This is a placeholder implementation. In a real application, this would extract
// the client ID from the authenticated context after proper authentication and authorization.
func extractClientID(r *http.Request) string {
	// For demo purposes, we'll use a query parameter
	clientID := r.URL.Query().Get("client_id")
	if clientID != "" {
		return clientID
	}
	return ""
}
