package utils

import (
	"net"
	"net/http"
	"strings"
)

// KeyByAPIKey returns a key based on API key only
func KeyByAPIKey(r *http.Request) string {
	if k := strings.TrimSpace(r.Header.Get("X-API-Key")); k != "" {
		return "apikey:" + k
	}
	// Fallback to per-IP
	ip := ClientIP(r)
	return "ip:" + ip
}

// KeyByIP returns a key based only on client IP
func KeyByIP(r *http.Request) string {
	return "ip:" + ClientIP(r)
}

// KeyByAPIKeyOrIP returns a key based on API key or IP address (alias for KeyByAPIKey)
func KeyByAPIKeyOrIP(r *http.Request) string {
	return KeyByAPIKey(r)
}

// KeyByAPIKeyAndRoute returns a key based on API key and route
func KeyByAPIKeyAndRoute(r *http.Request) string {
	seg := RouteBucket(r.URL.Path)
	if k := strings.TrimSpace(r.Header.Get("X-API-Key")); k != "" {
		return "apikey:" + k + "|route:" + seg
	}
	return "ip:" + ClientIP(r) + "|route:" + seg
}

// RouteBucket normalizes a path to a small set of buckets
func RouteBucket(path string) string {
	// normalize to a small set of buckets like /v1/users, /v1/orders, etc.
	p := strings.Trim(path, "/")
	parts := strings.Split(p, "/")
	if len(parts) == 0 || parts[0] == "" {
		return "/"
	}
	if len(parts) >= 2 {
		return "/" + parts[0] + "/" + parts[1]
	}
	return "/" + parts[0]
}

// ClientIP extracts the client IP address from an HTTP request
func ClientIP(r *http.Request) string {
	// Honor X-Forwarded-For (trust only if set by your proxy/load balancer)
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		return strings.TrimSpace(parts[0])
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
