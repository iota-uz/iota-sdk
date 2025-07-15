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

# Storage backend (memory or redis)
RATE_LIMIT_STORAGE=memory       # Use 'redis' for production
RATE_LIMIT_REDIS_URL=redis://localhost:6379
```

## Usage Examples

### API Protection Use Cases

#### General API Rate Limiting
Protect your API endpoints from excessive usage:

```go
import "github.com/iota-uz/iota-sdk/pkg/middleware"

// General API protection: 100 requests per minute per IP
router.Use(middleware.IPRateLimitPeriod(100, time.Minute))

// High-traffic API: 1000 requests per hour per IP
router.Use(middleware.IPRateLimitPeriod(1000, time.Hour))
```

#### Authentication & Security Use Cases
Protect sensitive endpoints like login, registration, and password reset:

```go
// Login protection: 10 attempts per minute per IP
loginRouter := r.PathPrefix("/login").Subrouter()
loginRouter.Use(middleware.IPRateLimitPeriod(10, time.Minute))

// Registration: 5 accounts per hour per IP
regRouter := r.PathPrefix("/register").Subrouter()
regRouter.Use(middleware.IPRateLimitPeriod(5, time.Hour))

// Password reset: 3 attempts per hour per IP
resetRouter := r.PathPrefix("/reset-password").Subrouter()
resetRouter.Use(middleware.IPRateLimitPeriod(3, time.Hour))
```

#### User-based Rate Limiting
Rate limit authenticated users by their user ID instead of IP:

```go
// Authenticated API endpoints: 1000 requests per hour per user
apiRouter := r.PathPrefix("/api").Subrouter()
apiRouter.Use(middleware.Authorize())  // Ensure user is authenticated first
apiRouter.Use(middleware.UserRateLimitPeriod(1000, time.Hour))

// Premium user endpoints: 5000 requests per hour per user
premiumRouter := r.PathPrefix("/api/premium").Subrouter()
premiumRouter.Use(middleware.Authorize())
premiumRouter.Use(middleware.UserRateLimitPeriod(5000, time.Hour))
```

#### Global Protection Use Cases
Protect your entire application from overload:

```go
// Global protection: 10,000 requests per hour across all clients
router.Use(middleware.GlobalRateLimitPeriod(10000, time.Hour))

// DDoS protection: 50,000 requests per minute globally
router.Use(middleware.GlobalRateLimitPeriod(50000, time.Minute))
```

### Advanced Configuration

#### Custom Rate Limiting with Redis
For production deployments with multiple instances:

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
    RequestsPerPeriod: 50,
    Period:           time.Minute,
    BurstSize:        100,  // Allow bursts up to 100 requests
    Store:           store,
    KeyFunc: func(r *http.Request) string {
        // Custom key function - e.g., by API key
        return "api_key:" + r.Header.Get("X-API-Key")
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

## Configuration Pattern

This implementation uses integer values with custom time periods instead of fixed per-second rates. This provides more flexibility and clearer configuration:

```go
// Time-based rate limiting patterns
middleware.IPRateLimitPeriod(100, time.Minute)     // ✅ 100 requests per minute per IP
middleware.IPRateLimitPeriod(10, time.Second)      // ✅ 10 requests per second per IP
middleware.GlobalRateLimitPeriod(1000, time.Hour)  // ✅ 1000 requests per hour globally

// Use case specific examples
middleware.IPRateLimitPeriod(10, time.Minute)      // ✅ Login protection
middleware.UserRateLimitPeriod(1000, time.Hour)    // ✅ Authenticated user limits
middleware.GlobalRateLimitPeriod(50000, time.Minute) // ✅ DDoS protection
```

This pattern makes the API more intuitive and allows for precise rate limiting configuration based on your specific requirements and time windows. The middleware automatically handles authentication context for user-based rate limiting and falls back to IP-based limiting for unauthenticated requests.