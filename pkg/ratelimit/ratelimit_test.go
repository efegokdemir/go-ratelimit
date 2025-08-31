package ratelimit

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/dheerajk/go-ratelimit/pkg/middleware"
	"github.com/redis/go-redis/v9"
)

func TestDefaultOptions(t *testing.T) {
	// Use a nil Redis client for testing - in real code this would panic
	// but for the test we just want to check the default values
	mockRedis := &redis.Client{}
	options := DefaultOptions(mockRedis)

	if options.Redis == nil {
		t.Error("Redis client not set correctly")
	}

	if options.Prefix != "ratelimit" {
		t.Errorf("Expected prefix 'ratelimit', got '%s'", options.Prefix)
	}

	if options.RequestLimit != 60 {
		t.Errorf("Expected request limit 60, got %d", options.RequestLimit)
	}

	if options.ByteLimit != 1048576 {
		t.Errorf("Expected byte limit 1048576, got %d", options.ByteLimit)
	}

	if options.Window != time.Minute {
		t.Errorf("Expected window 1m0s, got %s", options.Window)
	}

	if options.AdminKey != "change-me-in-production" {
		t.Errorf("Expected admin key 'change-me-in-production', got '%s'", options.AdminKey)
	}

	// Test the key function
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-API-Key", "test-key")
	req.RemoteAddr = "192.0.2.1:1234"

	keyResult := options.KeyFunc(req)
	expectedKey := "apikey:test-key|route:/test"
	if keyResult != expectedKey {
		t.Errorf("Expected key '%s', got '%s'", expectedKey, keyResult)
	}
}

func TestMatcherFunctions(t *testing.T) {
	// Test that the matcher functions are properly exposed
	tests := []struct {
		name     string
		path     string
		matcher  func(string) bool
		expected bool
	}{
		{
			name:     "MatchPathExact - match",
			path:     "/api/users",
			matcher:  MatchPathExact("/api/users"),
			expected: true,
		},
		{
			name:     "MatchPathExact - no match",
			path:     "/api/products",
			matcher:  MatchPathExact("/api/users"),
			expected: false,
		},
		{
			name:     "MatchPathPrefix - match",
			path:     "/api/users/123",
			matcher:  MatchPathPrefix("/api/users"),
			expected: true,
		},
		{
			name:     "MatchPathPrefix - no match",
			path:     "/api/products",
			matcher:  MatchPathPrefix("/api/users"),
			expected: false,
		},
		{
			name:     "MatchPathRegex - match",
			path:     "/api/users/123",
			matcher:  MatchPathRegex("^/api/users/\\d+$"),
			expected: true,
		},
		{
			name:     "MatchPathRegex - no match",
			path:     "/api/users/abc",
			matcher:  MatchPathRegex("^/api/users/\\d+$"),
			expected: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := tc.matcher(tc.path)
			if result != tc.expected {
				t.Errorf("Expected %v, got %v", tc.expected, result)
			}
		})
	}

	// Test MatchAny
	anyMatcher := MatchAny(
		MatchPathExact("/api/users"),
		MatchPathExact("/api/products"),
	)
	if !anyMatcher("/api/users") {
		t.Error("MatchAny should match /api/users")
	}
	if !anyMatcher("/api/products") {
		t.Error("MatchAny should match /api/products")
	}
	if anyMatcher("/api/orders") {
		t.Error("MatchAny should not match /api/orders")
	}

	// Test MatchNone
	noneMatcher := MatchNone(
		MatchPathExact("/api/users"),
		MatchPathExact("/api/products"),
	)
	if noneMatcher("/api/users") {
		t.Error("MatchNone should not match /api/users")
	}
	if noneMatcher("/api/products") {
		t.Error("MatchNone should not match /api/products")
	}
	if !noneMatcher("/api/orders") {
		t.Error("MatchNone should match /api/orders")
	}
}

func TestKeyFunctions(t *testing.T) {
	// Test that the key functions are properly exposed
	req := httptest.NewRequest(http.MethodGet, "/api/users/123", nil)
	req.Header.Set("X-API-Key", "test-key")
	req.RemoteAddr = "192.0.2.1:1234"

	// Test KeyByAPIKey
	apiKeyResult := KeyByAPIKey(req)
	expectedAPIKey := "apikey:test-key"
	if apiKeyResult != expectedAPIKey {
		t.Errorf("Expected '%s', got '%s'", expectedAPIKey, apiKeyResult)
	}

	// Test KeyByAPIKeyAndRoute
	routeResult := KeyByAPIKeyAndRoute(req)
	expectedRoute := "apikey:test-key|route:/api/users"
	if routeResult != expectedRoute {
		t.Errorf("Expected '%s', got '%s'", expectedRoute, routeResult)
	}

	// Test KeyByIP
	ipResult := KeyByIP(req)
	expectedIP := "ip:192.0.2.1"
	if ipResult != expectedIP {
		t.Errorf("Expected '%s', got '%s'", expectedIP, ipResult)
	}
}

func TestWithKeyFunc(t *testing.T) {
	customKeyFunc := func(r *http.Request) string {
		return "custom:" + r.URL.Path
	}

	wrappedFunc := WithKeyFunc(customKeyFunc)

	req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	result := wrappedFunc(req)
	expected := "custom:/api/test"

	if result != expected {
		t.Errorf("Expected '%s', got '%s'", expected, result)
	}
}

// Mock service for testing
type mockService struct {
	middlewareCalled bool
	selectiveCalled  bool
	matcher          middleware.PathMatcher
}

func (s *mockService) Middleware() func(http.Handler) http.Handler {
	s.middlewareCalled = true
	return func(next http.Handler) http.Handler {
		return next
	}
}

func (s *mockService) SelectiveMiddleware(matcher middleware.PathMatcher) func(http.Handler) http.Handler {
	s.selectiveCalled = true
	s.matcher = matcher
	return func(next http.Handler) http.Handler {
		return next
	}
}
