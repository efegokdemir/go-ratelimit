# Software Design Document: Go Rate Limiting Library

## 1. Introduction

### 1.1 Purpose
This document describes the design of a comprehensive, Redis-based rate limiting library for Go HTTP services. The library provides flexible and configurable rate limiting capabilities that can be easily integrated into any Go HTTP service.

### 1.2 Scope
The library provides:
- Request count and response size (bytes) rate limiting
- Sliding window algorithm for accurate rate limiting
- Redis-based storage for distributed environments
- Dynamic per-client configuration
- Customizable key generation strategies
- Selective rate limiting for specific endpoints

## 2. System Architecture

### 2.1 High-Level Architecture
The library is designed as a set of modular components that can be used independently or together:

```
pkg/
├── ratelimit/      # Main package API and facade
├── limiter/        # Core rate limiting logic
├── middleware/     # HTTP middleware components
├── config/         # Configuration management
├── api/            # API endpoints for configuration
└── utils/          # Helper utilities
```

### 2.2 Component Diagram

```
┌─────────────────────────────┐      ┌─────────────────────┐
│                             │      │                     │
│    HTTP Service             │      │     Redis Server    │
│    ┌─────────────────────┐  │      │                     │
│    │                     │  │      │  ┌───────────────┐  │
│    │  Application Logic  │  │      │  │ Rate Limit    │  │
│    │                     │  │      │  │ Data          │  │
│    └─────────────────────┘  │      │  │               │  │
│              ▲              │      │  └───────────────┘  │
│              │              │      │  ┌───────────────┐  │
│    ┌─────────────────────┐  │      │  │ Client        │  │
│    │  Rate Limit         │  │      │  │ Configurations│  │
│    │  Middleware         ├──┼──────┼─►│               │  │
│    └─────────────────────┘  │      │  └───────────────┘  │
│                             │      │                     │
└─────────────────────────────┘      └─────────────────────┘
```

## 3. Component Design

### 3.1 Rate Limiting Service (`pkg/ratelimit`)

The `ratelimit` package provides the main API for the library. It serves as a facade for the underlying components.

#### 3.1.1 Key Responsibilities
- Initialize rate limiting components
- Provide middleware functions
- Expose configuration methods
- Provide key generation utilities

#### 3.1.2 Key Functions
- `New(options)`: Create a new rate limiting service
- `Middleware()`: Get HTTP middleware for rate limiting
- `SelectiveMiddleware(matcher)`: Get middleware that only applies to specific paths
- `SetClientConfig(clientID, requestLimit, byteLimit, windowSecs)`: Set custom limits for a client
- `ResetClientConfig(clientID)`: Reset client to default configuration
- `RegisterConfigAPI(mux)`: Register configuration API endpoints

### 3.2 Limiters (`pkg/limiter`)

The `limiter` package contains the core rate limiting logic.

#### 3.2.1 Components
- `redis.go`: Basic Redis-based rate limiter
- `dual_limiter.go`: Combined request count and byte limiter
- `dynamic_limiter.go`: Limiter with dynamic per-client configuration

#### 3.2.2 Key Functions
- `AllowRequest(ctx, req)`: Check if a request is allowed
- `RecordBytes(ctx, req, bytes)`: Record bytes transferred
- `GetClientConfig(ctx, clientID)`: Get client's configuration
- `SetClientConfig(ctx, clientID, cfg)`: Set client's configuration
- `ResetClientConfig(ctx, clientID)`: Reset client to default configuration

### 3.3 Middleware (`pkg/middleware`)

The `middleware` package contains HTTP middleware components.

#### 3.3.1 Components
- `ratelimit.go`: Basic rate limiting middleware
- `dual_ratelimit.go`: Middleware for both request count and byte limiting
- `dynamic_ratelimit.go`: Middleware with dynamic client configuration
- `selective_ratelimit.go`: Middleware that only applies to specific paths
- `bytecounter.go`: Response writer wrapper to count bytes

#### 3.3.2 Key Functions
- `DynamicRateLimitMiddleware(limiter)`: Create middleware for rate limiting
- `SelectiveRateLimitMiddleware(limiter, matcher)`: Create selective middleware
- `MatchPathExact(path)`: Match exact path
- `MatchPathPrefix(prefix)`: Match path prefix
- `MatchPathRegex(pattern)`: Match path using regex

### 3.4 Configuration (`pkg/config`)

The `config` package manages per-client configurations.

#### 3.4.1 Components
- `manager.go`: Configuration manager

#### 3.4.2 Key Functions
- `GetConfig(ctx, clientID)`: Get client's configuration
- `SetConfig(ctx, clientID, cfg)`: Set client's configuration
- `DeleteConfig(ctx, clientID)`: Delete client's configuration

### 3.5 API (`pkg/api`)

The `api` package provides HTTP endpoints for configuration management.

#### 3.5.1 Components
- `config_handler.go`: HTTP handlers for configuration API

#### 3.5.2 Key Functions
- `RegisterRoutes(mux)`: Register API routes
- `handleGetConfig(w, r)`: Handle GET requests for client configuration
- `handleSetConfig(w, r)`: Handle PUT requests to update configuration
- `handleDeleteConfig(w, r)`: Handle DELETE requests to reset configuration

### 3.6 Utilities (`pkg/utils`)

The `utils` package provides helper functions.

#### 3.6.1 Components
- `request.go`: Request-related utilities

#### 3.6.2 Key Functions
- `KeyByClientID(r, clientID)`: Generate key based on client ID
- `KeyByClientIDAndRoute(r, clientID)`: Generate key based on client ID and route
- `KeyByIP(r)`: Generate key based on IP address
- `RouteBucket(path)`: Normalize path to a bucket
- `ClientIP(r)`: Extract client IP address

## 4. Data Flow

### 4.1 Request Flow

1. Client sends HTTP request
2. Request passes through rate limiting middleware
3. Middleware extracts client ID (provided by the API implementation)
4. Middleware checks if request is allowed
   - If not allowed, returns 429 Too Many Requests
   - If allowed, proceeds to next handler
5. Response is wrapped with byte counter
6. After response is sent, bytes used are recorded

### 4.2 Configuration Flow

1. Admin sends configuration request
2. Configuration API validates admin key
3. Configuration is stored in Redis
4. Configuration is cached in memory
5. New requests use updated configuration

## 5. Rate Limiting Algorithm

### 5.1 Sliding Window Algorithm

The library uses a sliding window algorithm implemented with Redis sorted sets:

1. Current time is divided into fixed-size buckets (e.g., 1 second)
2. Each request is recorded in the current bucket
3. When checking limits, the algorithm:
   - Removes expired buckets (older than the window)
   - Sums the counts from remaining buckets
   - Compares the sum to the limit

This approach:
- Prevents traffic spikes at window boundaries
- Provides more accurate rate limiting
- Works well in distributed environments

### 5.2 Redis Implementation

Redis sorted sets (ZSET) are used to store:
- Request counts per bucket
- Byte counts per bucket

Redis hash maps (HASH) are used to store:
- Client configurations

## 6. Client Identification

### 6.1 Client ID Extraction

The library expects client IDs to be extracted by the API implementation after proper authentication and authorization. This design choice:

- Separates authentication concerns from rate limiting
- Allows flexible integration with any authentication system
- Enables rate limiting based on authenticated entities

### 6.2 Key Generation

The library provides several key generation strategies:

- `KeyByClientID`: Limit by client ID only
- `KeyByClientIDAndRoute`: Limit by client ID and route
- `KeyByIP`: Limit by IP address only
- Custom key functions: Implement custom identification logic

## 7. Configuration Management

### 7.1 Default Configuration

Default rate limits are specified when creating the rate limiting service:

```go
options := ratelimit.Options{
    RequestLimit: 100,              // 100 requests per window
    ByteLimit:    5 * 1024 * 1024,  // 5MB per window
    Window:       time.Minute,      // 1 minute window
}
```

### 7.2 Per-Client Configuration

Custom limits can be set for specific clients:

```go
service.SetClientConfig("client123", 1000, 10*1024*1024, 60)
```

### 7.3 Configuration API

The library provides a REST API for configuration management:

- `GET /admin/config/{clientID}`: Get client's configuration
- `PUT /admin/config/{clientID}`: Update client's configuration
- `DELETE /admin/config/{clientID}`: Reset client to default configuration

## 8. Testing Strategy

### 8.1 Unit Tests

Unit tests cover:
- Key generation and request utilities
- Path matching for selective rate limiting
- Byte counting middleware
- Core rate limiting functionality

### 8.2 Integration Tests

Integration tests verify:
- Redis interaction
- HTTP middleware behavior
- Configuration API

## 9. Examples

The library includes several examples:

### 9.1 Basic Example

Demonstrates simple rate limiting setup.

### 9.2 Dynamic Example

Demonstrates dynamic per-client configuration.

### 9.3 Server Example

Demonstrates a more complex server setup with selective rate limiting.

## 10. Performance Considerations

### 10.1 Redis Connection Pooling

The library uses Redis connection pooling to optimize performance.

### 10.2 Local Caching

Client configurations are cached in memory to reduce Redis queries.

### 10.3 Lua Scripts

Rate limiting operations use Redis Lua scripts to ensure atomicity and reduce network round-trips.

## 11. Conclusion

This Go rate limiting library provides a comprehensive solution for HTTP service rate limiting. Its modular design, flexible configuration, and efficient implementation make it suitable for a wide range of applications.
