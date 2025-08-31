package middleware

import (
	"fmt"
	"net/http"

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
			w.Header().Set("RateLimit-Request-Limit", fmt.Sprintf("%d", cfg.RequestLimit))
			w.Header().Set("RateLimit-Request-Remaining", fmt.Sprintf("%d", remaining))
			w.Header().Set("RateLimit-Request-Reset", fmt.Sprintf("%d", reset))

			if !allowed {
				w.Header().Set("Retry-After", fmt.Sprintf("%d", reset))
				http.Error(w, "request rate limit exceeded", http.StatusTooManyRequests)
				return
			}

			// Create a byte counter wrapper for the response writer
			bcw := NewByteCounterWriter(w)

			// Call the next handler with our wrapped response writer
			next.ServeHTTP(bcw, r)

			// After the handler has completed, record the bytes used
			// This is post-facto recording - we can't prevent the bytes from being sent
			// but we record it for future rate limiting decisions
			if bcw.StatusCode < 400 { // Only count successful responses
				dl.RecordBytes(r.Context(), r, bcw.BytesWritten)
				// Note: We can't set headers here as they won't be sent after the body
			}
		})
	}
}

// extractClientID extracts a client ID from the request for config lookup
func extractClientID(r *http.Request) string {
	apiKey := r.Header.Get("X-API-Key")
	if apiKey != "" {
		return apiKey
	}
	return ""
}
