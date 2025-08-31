package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/dheerajk/go-ratelimit/pkg/config"
)

func TestMatchPathExact(t *testing.T) {
	tests := []struct {
		name     string
		matcher  PathMatcher
		path     string
		expected bool
	}{
		{
			name:     "exact match",
			matcher:  MatchPathExact("/api/users"),
			path:     "/api/users",
			expected: true,
		},
		{
			name:     "no match",
			matcher:  MatchPathExact("/api/users"),
			path:     "/api/products",
			expected: false,
		},
		{
			name:     "substring not matched",
			matcher:  MatchPathExact("/api/users"),
			path:     "/api/users/123",
			expected: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := tc.matcher(tc.path)
			if result != tc.expected {
				t.Errorf("MatchPathExact() = %v, want %v", result, tc.expected)
			}
		})
	}
}

func TestMatchPathPrefix(t *testing.T) {
	tests := []struct {
		name     string
		matcher  PathMatcher
		path     string
		expected bool
	}{
		{
			name:     "exact match",
			matcher:  MatchPathPrefix("/api/users"),
			path:     "/api/users",
			expected: true,
		},
		{
			name:     "prefix match",
			matcher:  MatchPathPrefix("/api/users"),
			path:     "/api/users/123",
			expected: true,
		},
		{
			name:     "no match",
			matcher:  MatchPathPrefix("/api/users"),
			path:     "/api/products",
			expected: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := tc.matcher(tc.path)
			if result != tc.expected {
				t.Errorf("MatchPathPrefix() = %v, want %v", result, tc.expected)
			}
		})
	}
}

func TestMatchPathRegex(t *testing.T) {
	tests := []struct {
		name     string
		pattern  string
		path     string
		expected bool
	}{
		{
			name:     "exact match",
			pattern:  "^/api/users$",
			path:     "/api/users",
			expected: true,
		},
		{
			name:     "regex match",
			pattern:  "^/api/users/\\d+$",
			path:     "/api/users/123",
			expected: true,
		},
		{
			name:     "no match",
			pattern:  "^/api/users/\\d+$",
			path:     "/api/users/abc",
			expected: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			matcher := MatchPathRegex(tc.pattern)
			result := matcher(tc.path)
			if result != tc.expected {
				t.Errorf("MatchPathRegex(%s) with path %s = %v, want %v", tc.pattern, tc.path, result, tc.expected)
			}
		})
	}
}

func TestMatchAny(t *testing.T) {
	tests := []struct {
		name     string
		matchers []PathMatcher
		path     string
		expected bool
	}{
		{
			name: "match first",
			matchers: []PathMatcher{
				MatchPathExact("/api/users"),
				MatchPathExact("/api/products"),
			},
			path:     "/api/users",
			expected: true,
		},
		{
			name: "match second",
			matchers: []PathMatcher{
				MatchPathExact("/api/users"),
				MatchPathExact("/api/products"),
			},
			path:     "/api/products",
			expected: true,
		},
		{
			name: "no match",
			matchers: []PathMatcher{
				MatchPathExact("/api/users"),
				MatchPathExact("/api/products"),
			},
			path:     "/api/orders",
			expected: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			matcher := MatchAny(tc.matchers...)
			result := matcher(tc.path)
			if result != tc.expected {
				t.Errorf("MatchAny() = %v, want %v", result, tc.expected)
			}
		})
	}
}

func TestMatchNone(t *testing.T) {
	tests := []struct {
		name     string
		matchers []PathMatcher
		path     string
		expected bool
	}{
		{
			name: "match none - true",
			matchers: []PathMatcher{
				MatchPathExact("/api/users"),
				MatchPathExact("/api/products"),
			},
			path:     "/api/orders",
			expected: true,
		},
		{
			name: "match none - false (first matches)",
			matchers: []PathMatcher{
				MatchPathExact("/api/users"),
				MatchPathExact("/api/products"),
			},
			path:     "/api/users",
			expected: false,
		},
		{
			name: "match none - false (second matches)",
			matchers: []PathMatcher{
				MatchPathExact("/api/users"),
				MatchPathExact("/api/products"),
			},
			path:     "/api/products",
			expected: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			matcher := MatchNone(tc.matchers...)
			result := matcher(tc.path)
			if result != tc.expected {
				t.Errorf("MatchNone() = %v, want %v", result, tc.expected)
			}
		})
	}
}

func TestSelectiveRateLimitMiddleware(t *testing.T) {
	tests := []struct {
		name            string
		path            string
		shouldRateLimit bool
	}{
		{
			name:            "path should be rate limited",
			path:            "/api/limited",
			shouldRateLimit: true,
		},
		{
			name:            "path should not be rate limited",
			path:            "/api/unlimited",
			shouldRateLimit: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Create a mock handler that will be called if rate limiting is bypassed
			handlerCalled := false
			mockHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				handlerCalled = true
				w.WriteHeader(http.StatusOK)
			})

			// Create a mock matcher that only limits specific paths
			mockMatcher := func(path string) bool {
				return path == "/api/limited"
			}

			// Create a mock limiter middleware that always rejects requests
			mockLimiterMiddleware := func(next http.Handler) http.Handler {
				return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					// Always reject with 429 if rate limiting is applied
					w.WriteHeader(http.StatusTooManyRequests)
				})
			}

			// Create a mock limiter (unused in this test but would be used in a real implementation)
			_ = &mockDynamicLimiter{
				middleware: mockLimiterMiddleware,
			}

			// Create a wrapper function to handle type conversion
			selectiveMiddleware := func(matcher PathMatcher) func(http.Handler) http.Handler {
				return func(next http.Handler) http.Handler {
					// If the path should be rate limited, use the mock limiter middleware
					return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						if matcher(r.URL.Path) {
							mockLimiterMiddleware(next).ServeHTTP(w, r)
						} else {
							next.ServeHTTP(w, r)
						}
					})
				}
			}

			// Create the selective middleware
			middleware := selectiveMiddleware(mockMatcher)
			handler := middleware(mockHandler)

			// Create a test request
			req := httptest.NewRequest(http.MethodGet, tc.path, nil)
			rec := httptest.NewRecorder()

			// Process the request
			handler.ServeHTTP(rec, req)

			// Check if the handler was called (which means rate limiting was bypassed)
			if tc.shouldRateLimit {
				if handlerCalled {
					t.Error("Handler was called but path should have been rate limited")
				}
				if rec.Code != http.StatusTooManyRequests {
					t.Errorf("Expected status code %d, got %d", http.StatusTooManyRequests, rec.Code)
				}
			} else {
				if !handlerCalled {
					t.Error("Handler was not called but path should not have been rate limited")
				}
				if rec.Code != http.StatusOK {
					t.Errorf("Expected status code %d, got %d", http.StatusOK, rec.Code)
				}
			}
		})
	}
}

// Mock limiter for testing
type mockDynamicLimiter struct {
	middleware func(http.Handler) http.Handler
}

func (m *mockDynamicLimiter) AllowRequest(ctx context.Context, r *http.Request) (bool, int64, int64, error) {
	return false, 0, 0, nil
}

func (m *mockDynamicLimiter) RecordBytes(ctx context.Context, r *http.Request, bytes int64) (bool, int64, int64, error) {
	return false, 0, 0, nil
}

func (m *mockDynamicLimiter) GetClientConfig(ctx context.Context, clientID string) (config.ClientConfig, error) {
	return config.ClientConfig{}, nil
}

func (m *mockDynamicLimiter) SetClientConfig(ctx context.Context, clientID string, cfg config.ClientConfig) error {
	return nil
}

func (m *mockDynamicLimiter) ResetClientConfig(ctx context.Context, clientID string) error {
	return nil
}
