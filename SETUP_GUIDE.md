# Go-Ratelimit Setup Guide

## Prerequisites

### 1. Install Redis

The go-ratelimit library requires Redis to store rate limit data. You have several options:

#### Option A: Install Redis Locally (Windows)

1. **Using WSL (Windows Subsystem for Linux) - Recommended**:
   ```bash
   # In WSL terminal
   sudo apt update
   sudo apt install redis-server
   sudo service redis-server start
   ```

2. **Using Docker**:
   ```bash
   docker run -d -p 6379:6379 --name redis redis:latest
   ```

3. **Using Memurai (Windows Redis alternative)**:
   - Download from: https://www.memurai.com/
   - Install and run as a Windows service

#### Option B: Install Redis Locally (Mac/Linux)

```bash
# Mac
brew install redis
brew services start redis

# Ubuntu/Debian
sudo apt update
sudo apt install redis-server
sudo systemctl start redis

# CentOS/RHEL
sudo yum install redis
sudo systemctl start redis
```

### 2. Verify Redis is Running

```bash
# Test Redis connection
redis-cli ping
# Should return: PONG
```

## Using the Library

### Method 1: Run the Existing Server Example

```bash
cd go-ratelimit

# Run the server (it includes rate limiting)
go run cmd/server/main.go
```

The server will start on port 8082 with these endpoints:
- `GET /v1/thing?client_id=user123` - Simple endpoint
- `GET /v1/heavy?client_id=user123` - Returns ~10KB response
- `GET /v1/veryheavy?client_id=user123` - Returns ~500KB response
- `GET /v1/my-limits?client_id=user123` - Shows current rate limits

### Method 2: Run the Simple Example

```bash
# Run the example we created
go run example_usage.go
```

Then test it:
```bash
# Make multiple requests to test rate limiting
for i in {1..15}; do curl http://localhost:8080/api/hello; done
```

### Method 3: Integrate into Your Project

1. **Add to your project**:
   ```bash
   go get gitlab-master.nvidia.com/kratos-dataverse-observability/go-ratelimit
   ```

2. **Basic Integration**:
   ```go
   import (
       "gitlab-master.nvidia.com/kratos-dataverse-observability/go-ratelimit/pkg/ratelimit"
       "github.com/redis/go-redis/v9"
   )

   // Create Redis client
   rdb := redis.NewClient(&redis.Options{
       Addr: "localhost:6379",
   })

   // Use default options
   options := ratelimit.DefaultOptions(rdb)
   service := ratelimit.New(options)

   // Apply to your router
   handler := service.Middleware()(yourMux)
   ```

3. **Custom Configuration**:
   ```go
   options := ratelimit.Options{
       Redis:        rdb,
       Prefix:       "myapp",
       RequestLimit: 100,                    // 100 requests
       ByteLimit:    10 * 1024 * 1024,      // 10MB
       Window:       5 * time.Minute,        // per 5 minutes
       KeyFunc:      ratelimit.KeyByClientIDAndRoute("default"),
   }
   ```

## Testing Rate Limiting

### 1. Test Request Count Limiting

```bash
# Make 11 requests quickly (limit is 10)
for i in {1..11}; do 
    echo "Request $i:"
    curl -w "\nStatus: %{http_code}\n" http://localhost:8080/api/hello
    echo "---"
done
```

The 11th request should return `429 Too Many Requests`.

### 2. Test Byte Limit

```bash
# Make requests to the data endpoint (100KB each)
# With 1MB limit, should fail after ~10 requests
for i in {1..12}; do 
    echo "Request $i:"
    curl -s -w "\nStatus: %{http_code}, Size: %{size_download} bytes\n" \
         http://localhost:8080/api/data > /dev/null
    echo "---"
done
```

### 3. Check Rate Limit Headers

```bash
curl -v http://localhost:8080/api/hello 2>&1 | grep -i "x-ratelimit"
```

You should see headers like:
- `X-RateLimit-Limit`: Your limit
- `X-RateLimit-Remaining`: Requests remaining
- `X-RateLimit-Reset`: When the window resets

## Advanced Features

### 1. Per-Client Configuration

```go
// Set custom limits for a specific client
service.SetClientConfig("premium-client", 
    1000,               // 1000 requests
    50 * 1024 * 1024,   // 50MB
    300)                // per 5 minutes

// Reset to defaults
service.ResetClientConfig("client-id")
```

### 2. Selective Rate Limiting

```go
// Only rate limit specific paths
matcher := ratelimit.MatchAny(
    ratelimit.MatchPathPrefix("/api/expensive/"),
    ratelimit.MatchPathExact("/api/heavy"),
)

handler := service.SelectiveMiddleware(matcher)(mux)
```

### 3. Custom Key Functions

```go
// Rate limit by API key header
customKeyFunc := func(r *http.Request) string {
    apiKey := r.Header.Get("X-API-Key")
    if apiKey == "" {
        return "anonymous"
    }
    return "apikey:" + apiKey
}

options.KeyFunc = customKeyFunc
```

## Troubleshooting

### Redis Connection Issues

1. **Check Redis is running**:
   ```bash
   redis-cli ping
   ```

2. **Check Redis logs**:
   ```bash
   # Linux/Mac
   tail -f /var/log/redis/redis-server.log
   
   # Docker
   docker logs redis
   ```

3. **Test connection in Go**:
   ```go
   ctx := context.Background()
   err := rdb.Ping(ctx).Err()
   if err != nil {
       log.Fatal("Redis connection failed:", err)
   }
   ```

### Rate Limit Not Working

1. **Check Redis keys**:
   ```bash
   redis-cli
   > KEYS ratelimit:*
   > ZRANGE ratelimit:req:ip:127.0.0.1 0 -1 WITHSCORES
   ```

2. **Enable debug logging**:
   ```go
   // Add logging to see what's happening
   fmt.Printf("Key: %s, Allowed: %v, Remaining: %d\n", 
              key, allowed, remaining)
   ```

### Performance Tuning

For high QPS applications:

1. **Increase Redis connection pool**:
   ```go
   rdb := redis.NewClient(&redis.Options{
       Addr:         "localhost:6379",
       PoolSize:     64,
       MinIdleConns: 16,
       ReadTimeout:  200 * time.Millisecond,
       WriteTimeout: 200 * time.Millisecond,
   })
   ```

2. **Use Redis Cluster** for distributed setup

3. **Consider using Redis Sentinel** for high availability

## Production Considerations

1. **Monitor Redis memory usage** - Rate limit data expires but can accumulate
2. **Set up Redis persistence** if you need rate limits to survive restarts
3. **Use TLS** for Redis connections in production
4. **Monitor rate limit metrics** - Track 429 responses
5. **Set appropriate limits** based on your infrastructure capacity

## Additional Resources

- Redis Documentation: https://redis.io/docs/
- Go Redis Client: https://github.com/redis/go-redis
- HTTP Rate Limiting Best Practices: https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/X-RateLimit-Limit 