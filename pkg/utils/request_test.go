package utils

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestKeyByAPIKey(t *testing.T) {
	tests := []struct {
		name     string
		apiKey   string
		expected string
	}{
		{
			name:     "with api key",
			apiKey:   "test-key-123",
			expected: "apikey:test-key-123",
		},
		{
			name:     "without api key",
			apiKey:   "",
			expected: "ip:192.0.2.1", // IP from the test request
		},
		{
			name:     "with whitespace api key",
			apiKey:   "  ",
			expected: "ip:192.0.2.1", // IP from the test request
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			req.RemoteAddr = "192.0.2.1:1234" // Test IP
			if tc.apiKey != "" {
				req.Header.Set("X-API-Key", tc.apiKey)
			}

			result := KeyByAPIKey(req)
			if result != tc.expected {
				t.Errorf("KeyByAPIKey() = %v, want %v", result, tc.expected)
			}
		})
	}
}

func TestKeyByIP(t *testing.T) {
	tests := []struct {
		name     string
		ip       string
		xff      string
		expected string
	}{
		{
			name:     "direct connection",
			ip:       "192.0.2.1:1234",
			xff:      "",
			expected: "ip:192.0.2.1",
		},
		{
			name:     "with X-Forwarded-For",
			ip:       "10.0.0.1:1234",
			xff:      "203.0.113.1, 192.168.1.1",
			expected: "ip:203.0.113.1",
		},
		{
			name:     "invalid RemoteAddr format",
			ip:       "invalid-format",
			xff:      "",
			expected: "ip:invalid-format",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			req.RemoteAddr = tc.ip
			if tc.xff != "" {
				req.Header.Set("X-Forwarded-For", tc.xff)
			}

			result := KeyByIP(req)
			if result != tc.expected {
				t.Errorf("KeyByIP() = %v, want %v", result, tc.expected)
			}
		})
	}
}

func TestKeyByAPIKeyAndRoute(t *testing.T) {
	tests := []struct {
		name     string
		apiKey   string
		path     string
		ip       string
		expected string
	}{
		{
			name:     "with api key and path",
			apiKey:   "test-key-123",
			path:     "/v1/users",
			ip:       "192.0.2.1:1234",
			expected: "apikey:test-key-123|route:/v1/users",
		},
		{
			name:     "without api key",
			apiKey:   "",
			path:     "/v1/products",
			ip:       "192.0.2.1:1234",
			expected: "ip:192.0.2.1|route:/v1/products",
		},
		{
			name:     "with nested path",
			apiKey:   "test-key-123",
			path:     "/v1/users/123/profile",
			ip:       "192.0.2.1:1234",
			expected: "apikey:test-key-123|route:/v1/users",
		},
		{
			name:     "with root path",
			apiKey:   "test-key-123",
			path:     "/",
			ip:       "192.0.2.1:1234",
			expected: "apikey:test-key-123|route:/",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tc.path, nil)
			req.RemoteAddr = tc.ip
			if tc.apiKey != "" {
				req.Header.Set("X-API-Key", tc.apiKey)
			}

			result := KeyByAPIKeyAndRoute(req)
			if result != tc.expected {
				t.Errorf("KeyByAPIKeyAndRoute() = %v, want %v", result, tc.expected)
			}
		})
	}
}

func TestRouteBucket(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		expected string
	}{
		{
			name:     "root path",
			path:     "/",
			expected: "/",
		},
		{
			name:     "single segment",
			path:     "/users",
			expected: "/users",
		},
		{
			name:     "two segments",
			path:     "/v1/users",
			expected: "/v1/users",
		},
		{
			name:     "multiple segments",
			path:     "/v1/users/123/profile",
			expected: "/v1/users",
		},
		{
			name:     "trailing slash",
			path:     "/v1/users/",
			expected: "/v1/users",
		},
		{
			name:     "empty path",
			path:     "",
			expected: "/",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := RouteBucket(tc.path)
			if result != tc.expected {
				t.Errorf("RouteBucket() = %v, want %v", result, tc.expected)
			}
		})
	}
}

func TestClientIP(t *testing.T) {
	tests := []struct {
		name       string
		remoteAddr string
		xff        string
		expected   string
	}{
		{
			name:       "standard format",
			remoteAddr: "192.0.2.1:1234",
			xff:        "",
			expected:   "192.0.2.1",
		},
		{
			name:       "with X-Forwarded-For",
			remoteAddr: "10.0.0.1:1234",
			xff:        "203.0.113.1, 192.168.1.1",
			expected:   "203.0.113.1",
		},
		{
			name:       "invalid format",
			remoteAddr: "invalid-format",
			xff:        "",
			expected:   "invalid-format",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			req.RemoteAddr = tc.remoteAddr
			if tc.xff != "" {
				req.Header.Set("X-Forwarded-For", tc.xff)
			}

			result := ClientIP(req)
			if result != tc.expected {
				t.Errorf("ClientIP() = %v, want %v", result, tc.expected)
			}
		})
	}
}
