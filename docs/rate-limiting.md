# Rate Limiting Middleware

The IOTA SDK provides comprehensive rate limiting middleware using the `ulule/limiter` package. This middleware protects your API endpoints from abuse and ensures fair resource allocation.

## Features

- **Multiple strategies**: Global, IP-based, User-based, and Endpoint-specific rate limiting
- **Storage backends**: Memory (for development) and Redis (for production)
- **Integer RPS configuration**: Simple and intuitive configuration using requests per second
- **Standard headers**: Includes `X-RateLimit-Limit`, `X-RateLimit-Remaining`, and `X-RateLimit-Reset` headers
- **Graceful error handling**: Fails open if rate limiter encounters errors
- **Configurable responses**: Custom rate limit exceeded responses

## Configuration

Add the following environment variables to configure rate limiting:

```bash
# Enable/disable rate limiting
RATE_LIMIT_ENABLED=true

# Rate limits (requests per second)
RATE_LIMIT_GLOBAL_RPS=1000      # Global rate limit for all requests
RATE_LIMIT_API_RPS=100          # API-specific rate limit  
RATE_LIMIT_AUTH_RPS=10          # Authentication endpoints rate limit

# Storage backend (memory or redis)
RATE_LIMIT_STORAGE=memory       # Use 'redis' for production
RATE_LIMIT_REDIS_URL=redis://localhost:6379

# Trusted proxies (comma-separated list)
RATE_LIMIT_TRUSTED_PROXIES=10.0.0.0/8,172.16.0.0/12,192.168.0.0/16
```

## Usage Examples

### Basic IP-based Rate Limiting

```go
import "github.com/iota-uz/iota-sdk/pkg/middleware"

// Limit to 100 requests per second per IP
router.Use(middleware.IPRateLimit(100))
```

### Global Rate Limiting

```go
// Limit to 1000 requests per second globally (shared across all clients)
router.Use(middleware.GlobalRateLimit(1000))
```

### Endpoint-specific Rate Limiting

```go
// Different limits for different endpoints
apiRouter := router.PathPrefix("/api").Subrouter()
apiRouter.Use(middleware.EndpointRateLimit("/api", 50))

authRouter := router.PathPrefix("/auth").Subrouter()
authRouter.Use(middleware.EndpointRateLimit("/auth", 5))
```

### API Rate Limiting with Integer RPS

```go
// Simple integer-based RPS configuration
router.Use(middleware.APIRateLimit(100))  // 100 requests per second
router.Use(middleware.APIRateLimit(50))   // 50 requests per second
router.Use(middleware.APIRateLimit(10))   // 10 requests per second
```

### Custom Rate Limiting Configuration

```go
import (
    "github.com/iota-uz/iota-sdk/pkg/middleware"
)

// Custom configuration with Redis store
store, err := middleware.NewRedisStore("redis://localhost:6379")
if err != nil {
    // Handle error
}

customMiddleware := middleware.RateLimit(middleware.RateLimitConfig{
    RequestsPerSecond: 50,
    BurstSize:        100,  // Allow bursts up to 100 requests
    Store:           store,
    KeyFunc: func(r *http.Request) string {
        // Custom key function - e.g., by user ID
        return "user:" + getUserID(r)
    },
    OnLimitReached: func(w http.ResponseWriter, r *http.Request) {
        // Custom response when rate limit is exceeded
        w.Header().Set("Content-Type", "application/json")
        w.WriteHeader(http.StatusTooManyRequests)
        w.Write([]byte(`{"error":"Rate limit exceeded","retry_after":60}`))
    },
})

router.Use(customMiddleware)
```

## Storage Backends

### Memory Store (Development)

The memory store is suitable for development and single-instance deployments:

```go
store := middleware.NewMemoryStore()
```

### Redis Store (Production)

For production deployments with multiple instances, use Redis:

```go
store, err := middleware.NewRedisStore("redis://localhost:6379")
if err != nil {
    log.Fatal(err)
}
```

## Rate Limit Headers

The middleware automatically adds standard rate limiting headers to responses:

- `X-RateLimit-Limit`: The rate limit ceiling for the endpoint
- `X-RateLimit-Remaining`: Number of requests remaining in the current window
- `X-RateLimit-Reset`: UTC timestamp when the rate limit window resets

## Server Integration

The rate limiting middleware is automatically integrated into the server when enabled:

```go
// In internal/server/default.go
if options.Configuration.RateLimit.Enabled {
    // Rate limiting is automatically configured based on environment variables
}
```

## Error Handling

The middleware follows a "fail open" policy - if the rate limiter encounters an error (e.g., Redis connection failure), it allows the request to proceed rather than blocking it. This ensures your application remains available even if the rate limiting infrastructure has issues.

## Testing

The middleware includes comprehensive tests covering:

- IP-based rate limiting
- Global rate limiting  
- Endpoint-specific rate limiting
- Custom configurations
- Rate limit headers
- Different RPS values

Run the tests with:

```bash
go test -v ./pkg/middleware -run TestRateLimit
```

## Performance Considerations

- **Memory store**: Fast but limited to single instance deployments
- **Redis store**: Slight latency but supports distributed deployments
- **Key selection**: Choose efficient key functions to minimize overhead
- **Rate limit values**: Set appropriate limits based on your application's capacity

## Migration from String-based Levels

This implementation uses integer RPS values instead of predefined string levels like "moderate" or "strict". This provides more flexibility and clearer configuration:

```go
// Old approach (not supported)
// middleware.APIRateLimit("moderate")  // ❌

// New approach (supported)
middleware.APIRateLimit(100)  // ✅ 100 requests per second
middleware.APIRateLimit(10)   // ✅ 10 requests per second
middleware.APIRateLimit(1000) // ✅ 1000 requests per second
```

This change makes the API more intuitive and allows for precise rate limiting configuration based on your specific requirements.