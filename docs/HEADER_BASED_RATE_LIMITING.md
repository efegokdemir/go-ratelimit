# Header-Based Rate Limiting

This guide explains how to implement rate limiting based on HTTP headers using the go-ratelimit library. This feature is particularly useful for multi-tenant applications, API key-based systems, and scenarios where you need to identify and limit clients based on custom headers.

## Table of Contents
- [Overview](#overview)
- [Basic Usage](#basic-usage)
- [Advanced Options](#advanced-options)
- [Use Cases](#use-cases)
- [Best Practices](#best-practices)
- [API Reference](#api-reference)

## Overview

Header-based rate limiting allows you to control access to your API based on values in HTTP headers. This is more flexible than IP-based limiting and enables:

- **Multi-tenant isolation**: Different limits per tenant
- **API key management**: Control usage per API key
- **Customer tier differentiation**: Different limits for different subscription levels
- **Service identification**: Rate limit by service or collector ID

## Basic Usage

### Simple Header-Based Rate Limiting

The simplest way to rate limit by a header is using `KeyByHeader`:

```go
import (
    "gitlab-master.nvidia.com/kratos-dataverse-observability/go-ratelimit/pkg/ratelimit"
    "github.com/redis/go-redis/v9"
)

// Create Redis client
rdb := redis.NewClient(&redis.Options{
    Addr: "localhost:6379",
})

// Rate limit by API key
options := ratelimit.Options{
    Redis:        rdb,
    Prefix:       "myapp",
    RequestLimit: 100,              // 100 requests
    ByteLimit:    10 * 1024 * 1024, // 10 MB
    Window:       time.Hour,        // per hour
    KeyFunc:      ratelimit.KeyByHeader("X-API-Key"),
}

service := ratelimit.New(options)

// Apply to your HTTP handler
handler := service.Middleware()(yourMux)
```

### Common Header Examples

```go
// Rate limit by tenant ID
KeyFunc: ratelimit.KeyByHeader("X-Tenant-ID")

// Rate limit by collector ID (logging systems)
KeyFunc: ratelimit.KeyByHeader("collector-id")

// Rate limit by customer ID
KeyFunc: ratelimit.KeyByHeader("X-Customer-ID")

// Rate limit by API version
KeyFunc: ratelimit.KeyByHeader("X-API-Version")
```

## Advanced Options

### Header with Options

For more control, use `KeyByHeaderWithOptions`:

```go
options := ratelimit.Options{
    Redis:        rdb,
    Prefix:       "myapp",
    RequestLimit: 1000,
    ByteLimit:    100 * 1024 * 1024, // 100 MB
    Window:       time.Hour,
    KeyFunc: ratelimit.KeyByHeaderWithOptions(utils.HeaderKeyOptions{
        HeaderName:    "X-Tenant-ID",
        Prefix:        "tenant",           // Redis keys will be "tenant:value"
        DefaultValue:  "anonymous",        // Used when header is missing
        CaseSensitive: false,              // Case-insensitive matching
        AllowEmpty:    false,              // Don't allow empty header values
        Sanitizer:     utils.AlphanumericSanitizer, // Remove special characters
    }),
}
```

### Available Options

| Option | Type | Description | Default |
|--------|------|-------------|---------|
| `HeaderName` | string | The HTTP header name to extract | Required |
| `Prefix` | string | Prefix for Redis keys | Empty |
| `DefaultValue` | string | Value when header is missing | Falls back to IP |
| `CaseSensitive` | bool | Whether header values are case-sensitive | false |
| `AllowEmpty` | bool | Allow empty header values | false |
| `Sanitizer` | func(string) string | Function to clean/validate values | None |

### Built-in Sanitizers

```go
// Remove all non-alphanumeric characters
Sanitizer: utils.AlphanumericSanitizer

// Validate and normalize UUIDs
Sanitizer: utils.UUIDSanitizer

// Limit value length
Sanitizer: utils.MaxLengthSanitizer(32)

// Custom sanitizer
Sanitizer: func(value string) string {
    // Your custom logic
    return strings.ToLower(strings.TrimSpace(value))
}
```

### Multiple Headers (Composite Keys)

Rate limit based on multiple headers combined:

```go
// Rate limit by tenant AND region
KeyFunc: ratelimit.KeyByMultipleHeaders(
    []string{"X-Tenant-ID", "X-Region"}, 
    ":", // separator
)
// Creates keys like: "tenant123:us-west"

// Rate limit by customer AND API version
KeyFunc: ratelimit.KeyByMultipleHeaders(
    []string{"X-Customer-ID", "X-API-Version"}, 
    "-",
)
// Creates keys like: "cust456-v2"
```

### Header Fallback Chain

Try multiple headers in order until one is found:

```go
// Try these headers in order: API key, then Authorization, then session ID
KeyFunc: ratelimit.KeyByHeaderWithFallback([]string{
    "X-API-Key",
    "Authorization",
    "X-Session-ID",
})
```

## Use Cases

### 1. Multi-Tenant SaaS Application

```go
// Different limits for different tenant tiers
basicTenantLimiter := ratelimit.New(ratelimit.Options{
    Redis:        rdb,
    Prefix:       "basic",
    RequestLimit: 1000,
    ByteLimit:    10 * 1024 * 1024, // 10 MB/hour
    Window:       time.Hour,
    KeyFunc: ratelimit.KeyByHeaderWithOptions(utils.HeaderKeyOptions{
        HeaderName: "X-Tenant-ID",
        Prefix:     "basic-tenant",
    }),
})

premiumTenantLimiter := ratelimit.New(ratelimit.Options{
    Redis:        rdb,
    Prefix:       "premium",
    RequestLimit: 10000,
    ByteLimit:    1024 * 1024 * 1024, // 1 GB/hour
    Window:       time.Hour,
    KeyFunc: ratelimit.KeyByHeaderWithOptions(utils.HeaderKeyOptions{
        HeaderName: "X-Tenant-ID",
        Prefix:     "premium-tenant",
    }),
})
```

### 2. API Gateway with Multiple Auth Methods

```go
// Support multiple authentication methods
options := ratelimit.Options{
    Redis:        rdb,
    Prefix:       "api-gateway",
    RequestLimit: 100,
    ByteLimit:    5 * 1024 * 1024,
    Window:       time.Minute,
    KeyFunc: ratelimit.KeyByHeaderWithFallback([]string{
        "X-API-Key",           // API key auth
        "Authorization",       // Bearer token auth
        "X-Client-ID",        // Client certificate auth
    }),
}
```

### 3. Microservices with Service Identification

```go
// Rate limit by calling service
options := ratelimit.Options{
    Redis:        rdb,
    Prefix:       "internal-api",
    RequestLimit: 10000,
    ByteLimit:    100 * 1024 * 1024,
    Window:       time.Minute,
    KeyFunc: ratelimit.KeyByHeaderWithOptions(utils.HeaderKeyOptions{
        HeaderName:   "X-Service-Name",
        Prefix:       "service",
        DefaultValue: "unknown-service",
    }),
}
```

### 4. Logging Infrastructure (collector-based)

```go
// Rate limit log ingestion by collector
options := ratelimit.Options{
    Redis:        rdb,
    Prefix:       "log-ingestion",
    RequestLimit: 0,                    // No request limit
    ByteLimit:    600 * 1024 * 1024,    // 600 MB/s per collector
    Window:       time.Second,
    KeyFunc: ratelimit.KeyByHeaderWithOptions(utils.HeaderKeyOptions{
        HeaderName:   "collector-id",
        Prefix:       "collector",
        DefaultValue: "unknown-collector",
        Sanitizer:    utils.AlphanumericSanitizer,
    }),
}
```

## Best Practices

### 1. Always Handle Missing Headers

```go
// Good: Provide a sensible default or fallback
KeyFunc: ratelimit.KeyByHeaderWithOptions(utils.HeaderKeyOptions{
    HeaderName:   "X-Tenant-ID",
    DefaultValue: "anonymous", // or fall back to IP
})

// Bad: May cause issues with missing headers
KeyFunc: ratelimit.KeyByHeader("X-Tenant-ID") // Falls back to IP automatically
```

### 2. Sanitize Header Values

Headers can contain unexpected values. Always sanitize when needed:

```go
KeyFunc: ratelimit.KeyByHeaderWithOptions(utils.HeaderKeyOptions{
    HeaderName: "X-API-Key",
    Sanitizer: func(value string) string {
        // Remove potential injection attempts
        value = strings.TrimSpace(value)
        if len(value) > 64 {
            value = value[:64]
        }
        // Remove any Redis special characters
        value = strings.ReplaceAll(value, ":", "-")
        return value
    },
})
```

### 3. Use Meaningful Prefixes

Prefixes help organize keys in Redis and prevent collisions:

```go
// Good: Clear namespace
Prefix: "tenant"        // Keys: "tenant:12345"
Prefix: "api-v2"        // Keys: "api-v2:key123"
Prefix: "collector"     // Keys: "collector:web-01"

// Bad: No context
Prefix: ""              // Keys: "12345" (unclear)
```

### 4. Consider Composite Keys for Fine-Grained Control

```go
// Rate limit by tenant AND endpoint
KeyFunc: ratelimit.KeyByMultipleHeaders(
    []string{"X-Tenant-ID", "X-Endpoint"}, 
    ":",
)
// Allows different limits for different endpoints per tenant
```

### 5. Monitor and Alert

Always monitor rate limit metrics:

```go
// Check rate limit headers in responses
// RateLimit-Limit: Maximum requests allowed
// RateLimit-Remaining: Requests remaining
// RateLimit-Reset: Time until limit resets

// Log when limits are hit
if resp.StatusCode == 429 {
    log.Printf("Rate limit hit for header: %s", headerValue)
}
```

## API Reference

### Functions

#### `KeyByHeader(headerName string) func(*http.Request) string`
Simple header-based key extraction.

#### `KeyByHeaderWithOptions(opts HeaderKeyOptions) func(*http.Request) string`
Header-based key extraction with advanced options.

#### `KeyByMultipleHeaders(headerNames []string, separator string) func(*http.Request) string`
Combine multiple headers into a composite key.

#### `KeyByHeaderWithFallback(headerNames []string) func(*http.Request) string`
Try multiple headers in order until one is found.

### Types

#### `HeaderKeyOptions`
```go
type HeaderKeyOptions struct {
    HeaderName    string
    Prefix        string
    DefaultValue  string
    CaseSensitive bool
    AllowEmpty    bool
    Sanitizer     func(string) string
}
```

### Sanitizers

- `AlphanumericSanitizer`: Removes all non-alphanumeric characters
- `UUIDSanitizer`: Validates and normalizes UUID format
- `MaxLengthSanitizer(n)`: Returns a sanitizer that truncates to n characters

## Troubleshooting

### Headers Not Being Read

1. Check header name spelling (case-sensitive in Go)
2. Verify the header is being sent by the client
3. Check if a proxy is stripping headers

### Rate Limits Not Working

1. Verify Redis connection
2. Check if header values are being sanitized unexpectedly
3. Monitor Redis keys: `redis-cli KEYS "yourprefix:*"`

### Performance Issues

1. Use simple sanitizers (avoid regex when possible)
2. Consider caching header extraction for repeated calls
3. Monitor Redis latency

## Migration Guide

If you're migrating from IP-based to header-based rate limiting:

```go
// Before: IP-based
KeyFunc: ratelimit.KeyByIP

// After: Header-based with IP fallback
KeyFunc: ratelimit.KeyByHeaderWithOptions(utils.HeaderKeyOptions{
    HeaderName:   "X-API-Key",
    DefaultValue: "", // Empty default will fall back to IP
})
```

This ensures backward compatibility while adding header support.
