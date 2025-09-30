package middleware

import (
	"fmt"
	"net/http"

	"gitlab-master.nvidia.com/kratos-dataverse-observability/go-ratelimit/pkg/limiter"
)

// DynamicRateLimitMiddleware enforces both request count and byte-based rate limiting with dynamic configs
func DynamicRateLimitMiddleware(dl *limiter.DynamicLimiter) func(http.Handler) http.Handler {
	return DynamicRateLimitMiddlewareWithLogger(dl, nil)
}

// DynamicRateLimitMiddlewareWithLogger enforces rate limiting with a custom logger
func DynamicRateLimitMiddlewareWithLogger(dl *limiter.DynamicLimiter, log Logger) func(http.Handler) http.Handler {
	// Use a no-op logger if none provided
	if log == nil {
		log = NoOpLogger{}
	}

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
			log.Info(fmt.Sprintf("Setting rate limit headers for client %s", clientID))
			if cfg.RequestLimit < 0 {
				// Request limiting is disabled
				w.Header().Set("RateLimit-Request-Limit", "unlimited")
				w.Header().Set("RateLimit-Request-Remaining", "unlimited")
			} else {
				w.Header().Set("RateLimit-Request-Limit", fmt.Sprintf("%d", cfg.RequestLimit))
				w.Header().Set("RateLimit-Request-Remaining", fmt.Sprintf("%d", remaining))
			}
			w.Header().Set("RateLimit-Request-Reset", fmt.Sprintf("%d", reset))

			// Get current byte usage to calculate bytes remaining
			currentUsage, _ := dl.GetCurrentUsage(r.Context(), r)
			bytesRemaining := cfg.ByteLimit - currentUsage
			if bytesRemaining < 0 {
				bytesRemaining = 0
			}

			// Also set byte rate limit headers (these are always active)
			w.Header().Set("RateLimit-Bytes-Limit", fmt.Sprintf("%d", cfg.ByteLimit))
			w.Header().Set("RateLimit-Bytes-Remaining", fmt.Sprintf("%d", bytesRemaining))
			w.Header().Set("RateLimit-Bytes-Window", fmt.Sprintf("%d", cfg.WindowSecs))

			if !allowed {
				log.Info(fmt.Sprintf("Request rate limit exceeded for client %s", clientID))
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
				log.Info(fmt.Sprintf("Recording bytes used for client %s: %d bytes", clientID, bcw.BytesWritten))

				// Record bytes and check if byte limit would be exceeded
				allowed, remaining, reset, err := dl.RecordBytes(r.Context(), r, bcw.BytesWritten)

				// Set byte rate limit headers
				w.Header().Set("RateLimit-Bytes-Limit", fmt.Sprintf("%d", cfg.ByteLimit))
				w.Header().Set("RateLimit-Bytes-Remaining", fmt.Sprintf("%d", remaining))
				w.Header().Set("RateLimit-Bytes-Reset", fmt.Sprintf("%d", reset))

				if !allowed && err == nil {
					log.Error(fmt.Sprintf("Byte rate limit exceeded for client %s", clientID))
					// Note: Response is already sent at this point, so we can't return 429
					// This is logged for monitoring purposes
				}
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
