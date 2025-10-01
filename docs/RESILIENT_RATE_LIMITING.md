# Resilient Rate Limiting

This document describes the resilient rate limiting mechanism implemented in this library, which provides fallback to in-memory rate limiting when Redis is unavailable.

## Overview

The resilient rate limiter combines two rate limiting strategies:

1. **Redis-based rate limiting**: The primary strategy, which provides distributed rate limiting across multiple instances of your application.
2. **In-memory rate limiting**: A fallback strategy that maintains rate limits locally when Redis is unavailable.

This approach ensures that your application can continue to enforce rate limits even during Redis outages, preventing potential abuse or overload situations.

## How It Works

### Normal Operation (Redis Available)

When Redis is available, the resilient limiter:

1. Uses Redis for all rate limiting operations
2. Stores client configurations in Redis
3. Updates in-memory store when client configurations change

### Fallback Operation (Redis Unavailable)

When Redis becomes unavailable:

1. The limiter automatically detects the Redis failure
2. All rate limiting operations switch to using the in-memory store
3. Client configurations are served from the in-memory cache
4. Rate limits continue to be enforced locally

### Redis Recovery

When Redis becomes available again:

1. The limiter detects that Redis is back online
2. Operations switch back to using Redis
3. Client-specific configurations remain consistent as they're maintained in both systems

## Usage

### Creating a Resilient Limiter

```go
// Create a resilient rate limiter
resilientLimiter := limiter.NewResilientLimiter(
    rdb,                // Redis client
    "rl",               // Key prefix
    10,                 // Default request limit
    1048576,            // Default byte limit (1MB)
    time.Minute,        // Window duration
    keyBuilderFunction, // Function to extract key from request
)

// Use it with the dynamic rate limiting middleware
handler := middleware.DynamicRateLimitMiddleware(resilientLimiter)(nextHandler)
```

The resilient limiter implements the `RateLimiter` interface, so it can be used with any middleware that accepts this interface.

### Configuration Management

The resilient limiter provides the same configuration management interface as the dynamic limiter, plus methods to access and update default configurations:

```go
// Get a client's config
cfg, err := resilientLimiter.GetClientConfig(ctx, clientID)

// Set a custom config for a client
err := resilientLimiter.SetClientConfig(ctx, clientID, config.ClientConfig{
    RequestLimit: 20,
    ByteLimit:    2097152,
    WindowSecs:   120,
})

// Reset a client to default config
err := resilientLimiter.ResetClientConfig(ctx, clientID)

// Get the current default configuration
defaultCfg := resilientLimiter.GetDefaultConfig()

// Update the default configuration
err := resilientLimiter.UpdateDefaultConfig(ctx, 30, 3145728, 120)
```

## Implementation Details

### Redis Health Checking

The resilient limiter periodically checks Redis availability:

- Default check interval: 5 seconds
- Short timeout for health checks: 500ms
- Automatic fallback to memory when checks fail
- Automatic recovery when Redis becomes available

### Memory Limiter

The in-memory limiter:

- Uses sliding window algorithm similar to the Redis implementation
- Stores request timestamps and byte records in memory
- Periodically cleans up expired entries
- Provides the same interface as the Redis-based limiter

### Configuration Management

Client configurations are:

- Primarily stored in Redis when available
- Maintained in memory for fallback purposes
- Always updated in both Redis and memory when changes are made
- Consistent across both systems without requiring synchronization

## Examples

- `examples/resilient_limiter/main.go` - Basic example of resilient rate limiting
- `examples/dynamic_resilient/main.go` - Example showing dynamic rate limiting with resilient fallback

## Considerations

### Memory Usage

The in-memory limiter stores rate limit data in memory, which can grow if you have many clients or high request volumes. Consider:

- Setting appropriate cleanup intervals
- Monitoring memory usage
- Adjusting window sizes for high-volume applications

### Consistency in Clustered Environments

When running multiple instances of your application:

- During normal operation (Redis available), rate limits are consistent across instances
- During Redis outages, each instance maintains its own local rate limits
- This may allow more total requests across all instances during an outage
- When Redis recovers, consistent distributed rate limiting resumes

### Configuration

The resilient limiter can be configured with:

- Redis check interval (default: 5 seconds)
- Redis check timeout (default: 500ms)
- Memory cleanup interval (controlled by the memory limiter)

Adjust these settings based on your application's needs and Redis infrastructure reliability.
