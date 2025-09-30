package middleware

import (
	"net/http"
	"regexp"
	"strings"

	"gitlab-master.nvidia.com/kratos-dataverse-observability/go-ratelimit/pkg/limiter"
)

// PathMatcher defines a function to determine if a path should be rate limited
type PathMatcher func(path string) bool

// SelectiveRateLimitMiddleware applies rate limiting only to paths that match specific criteria
func SelectiveRateLimitMiddleware(dl *limiter.DynamicLimiter, matcher PathMatcher) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Check if this path should be rate limited
			if !matcher(r.URL.Path) {
				// Skip rate limiting for this path
				next.ServeHTTP(w, r)
				return
			}

			// Apply normal rate limiting
			dynamicHandler := DynamicRateLimitMiddleware(dl)(next)
			dynamicHandler.ServeHTTP(w, r)
		})
	}
}

// MatchPathPrefix returns a matcher that matches paths with a specific prefix
func MatchPathPrefix(prefix string) PathMatcher {
	return func(path string) bool {
		return strings.HasPrefix(path, prefix)
	}
}

// MatchPathExact returns a matcher that matches paths exactly
func MatchPathExact(exactPath string) PathMatcher {
	return func(path string) bool {
		return path == exactPath
	}
}

// MatchPathRegex returns a matcher that matches paths using a regular expression
func MatchPathRegex(pattern string) PathMatcher {
	regex := regexp.MustCompile(pattern)
	return func(path string) bool {
		return regex.MatchString(path)
	}
}

// MatchAny returns a matcher that matches if any of the provided matchers match
func MatchAny(matchers ...PathMatcher) PathMatcher {
	return func(path string) bool {
		for _, m := range matchers {
			if m(path) {
				return true
			}
		}
		return false
	}
}

// MatchNone returns a matcher that matches if none of the provided matchers match
func MatchNone(matchers ...PathMatcher) PathMatcher {
	return func(path string) bool {
		for _, m := range matchers {
			if m(path) {
				return false
			}
		}
		return true
	}
}
