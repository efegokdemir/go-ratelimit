package middleware

import (
	"fmt"
	"net/http"

	"gitlab-master.nvidia.com/kratos-dataverse-observability/go-ratelimit/pkg/limiter"
)

// DualRateLimitMiddleware enforces both request count and byte-based rate limiting
func DualRateLimitMiddleware(dl *limiter.DualLimiter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// First check if the request is allowed based on request count
			allowed, remaining, reset, err := dl.AllowRequest(r.Context(), r)
			if err != nil {
				http.Error(w, "rate limiter error", http.StatusInternalServerError)
				return
			}

			// Set request count rate limit headers
			w.Header().Set("RateLimit-Request-Limit", fmt.Sprintf("%d", dl.GetRequestLimit()))
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
				// Headers must be set before the body is written
			}
		})
	}
}
