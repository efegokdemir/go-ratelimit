// Package ratelimit provides a flexible and configurable rate limiting solution
// for HTTP services based on Redis.
package ratelimit

import (
	"context"
	"net/http"
	"time"

	"github.com/dheerajk/go-ratelimit/pkg/api"
	"github.com/dheerajk/go-ratelimit/pkg/config"
	"github.com/dheerajk/go-ratelimit/pkg/limiter"
	"github.com/dheerajk/go-ratelimit/pkg/middleware"
	"github.com/dheerajk/go-ratelimit/pkg/utils"
	"github.com/redis/go-redis/v9"
)

// Options configures the rate limiter
type Options struct {
	// Redis client for storing rate limit data
	Redis *redis.Client

	// Prefix for Redis keys
	Prefix string

	// Default request limit per window
	RequestLimit int

	// Default byte limit per window
	ByteLimit int64

	// Time window for rate limiting
	Window time.Duration

	// Function to extract key from request
	KeyFunc func(*http.Request) string

	// Admin key for configuration API
	AdminKey string
}

// DefaultOptions returns sensible default options
func DefaultOptions(redis *redis.Client) Options {
	return Options{
		Redis:        redis,
		Prefix:       "ratelimit",
		RequestLimit: 60,
		ByteLimit:    1048576, // 1MB
		Window:       time.Minute,
		KeyFunc:      utils.KeyByIP,
		AdminKey:     "change-me-in-production",
	}
}

// Service provides rate limiting functionality
type Service struct {
	limiter    *limiter.DynamicLimiter
	configAPI  *api.ConfigHandler
	middleware func(http.Handler) http.Handler
	options    Options
}

// New creates a new rate limiting service
func New(options Options) *Service {
	if options.Redis == nil {
		panic("Redis client is required")
	}

	if options.KeyFunc == nil {
		options.KeyFunc = utils.KeyByIP
	}

	if options.Prefix == "" {
		options.Prefix = "ratelimit"
	}

	// Create the dynamic limiter
	dynamicLimiter := limiter.NewDynamicLimiter(
		options.Redis,
		options.Prefix,
		options.RequestLimit,
		options.ByteLimit,
		options.Window,
		options.KeyFunc,
	)

	// Create the config API handler
	configHandler := api.NewConfigHandler(dynamicLimiter)
	configHandler.SetAdminKey(options.AdminKey)

	// Create the middleware
	middlewareFunc := middleware.DynamicRateLimitMiddleware(dynamicLimiter)

	return &Service{
		limiter:    dynamicLimiter,
		configAPI:  configHandler,
		middleware: middlewareFunc,
		options:    options,
	}
}

// NewFromLimiter creates a new service from an existing limiter
func NewFromLimiter(dynamicLimiter *limiter.DynamicLimiter) *Service {
	// Create the config API handler
	configHandler := api.NewConfigHandler(dynamicLimiter)

	// Create the middleware
	middlewareFunc := middleware.DynamicRateLimitMiddleware(dynamicLimiter)

	return &Service{
		limiter:    dynamicLimiter,
		configAPI:  configHandler,
		middleware: middlewareFunc,
		options:    Options{}, // Empty options since we're using an existing limiter
	}
}

// Middleware returns an http.Handler middleware that applies rate limiting to all paths
func (s *Service) Middleware() func(http.Handler) http.Handler {
	return s.middleware
}

// SelectiveMiddleware returns a middleware that only applies rate limiting to paths matching the criteria
func (s *Service) SelectiveMiddleware(matcher middleware.PathMatcher) func(http.Handler) http.Handler {
	return middleware.SelectiveRateLimitMiddleware(s.limiter, matcher)
}

// MatchPathPrefix returns a matcher that matches paths with a specific prefix
func MatchPathPrefix(prefix string) middleware.PathMatcher {
	return middleware.MatchPathPrefix(prefix)
}

// MatchPathExact returns a matcher that matches paths exactly
func MatchPathExact(exactPath string) middleware.PathMatcher {
	return middleware.MatchPathExact(exactPath)
}

// MatchPathRegex returns a matcher that matches paths using a regular expression
func MatchPathRegex(pattern string) middleware.PathMatcher {
	return middleware.MatchPathRegex(pattern)
}

// MatchAny returns a matcher that matches if any of the provided matchers match
func MatchAny(matchers ...middleware.PathMatcher) middleware.PathMatcher {
	return middleware.MatchAny(matchers...)
}

// MatchNone returns a matcher that matches if none of the provided matchers match
func MatchNone(matchers ...middleware.PathMatcher) middleware.PathMatcher {
	return middleware.MatchNone(matchers...)
}

// RegisterConfigAPI registers the configuration API endpoints on the provided mux
func (s *Service) RegisterConfigAPI(mux *http.ServeMux) {
	s.configAPI.RegisterRoutes(mux)
}

// GetLimiter returns the underlying dynamic limiter
func (s *Service) GetLimiter() *limiter.DynamicLimiter {
	return s.limiter
}

// SetClientConfig sets rate limit configuration for a specific client
func (s *Service) SetClientConfig(clientID string, requestLimit int, byteLimit int64, windowSecs int) error {
	cfg := config.ClientConfig{
		RequestLimit: requestLimit,
		ByteLimit:    byteLimit,
		WindowSecs:   windowSecs,
	}
	return s.limiter.SetClientConfig(context.TODO(), clientID, cfg)
}

// ResetClientConfig resets a client to default configuration
func (s *Service) ResetClientConfig(clientID string) error {
	return s.limiter.ResetClientConfig(context.TODO(), clientID)
}

// WithKeyFunc returns a key extraction function that can be used in options
func WithKeyFunc(keyFunc func(*http.Request) string) func(*http.Request) string {
	return keyFunc
}

// KeyByIP extracts key based on client IP
func KeyByIP(r *http.Request) string {
	return utils.KeyByIP(r)
}

// KeyByClientID returns a function that extracts key based on client ID
// The client ID should be extracted by the API implementation after authentication
func KeyByClientID(clientID string) func(*http.Request) string {
	return func(r *http.Request) string {
		return utils.KeyByClientID(r, clientID)
	}
}

// KeyByClientIDAndRoute returns a function that extracts key based on client ID and route
// The client ID should be extracted by the API implementation after authentication
func KeyByClientIDAndRoute(clientID string) func(*http.Request) string {
	return func(r *http.Request) string {
		return utils.KeyByClientIDAndRoute(r, clientID)
	}
}
