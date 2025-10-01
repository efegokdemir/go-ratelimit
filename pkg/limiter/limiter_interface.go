package limiter

import (
	"context"
	"net/http"

	"gitlab-master.nvidia.com/kratos-dataverse-observability/go-ratelimit/pkg/config"
)

// RateLimiter defines the interface for rate limiters
type RateLimiter interface {
	// AllowRequest checks if a new request is allowed
	AllowRequest(ctx context.Context, r *http.Request) (bool, int64, int64, error)

	// RecordBytes records bytes used and checks if byte limit is exceeded
	RecordBytes(ctx context.Context, r *http.Request, bytes int64) (bool, int64, int64, error)

	// GetClientConfig returns the current config for a client
	GetClientConfig(ctx context.Context, clientID string) (config.ClientConfig, error)

	// GetCurrentUsage returns the current byte usage for a client
	GetCurrentUsage(ctx context.Context, r *http.Request) (int64, error)
}
