---
layout: default
title: Rate Limiting
parent: Advanced
nav_order: 4
description: "Rate limiting and request throttling in IOTA SDK"
---

# Rate Limiting

The IOTA SDK provides comprehensive rate limiting middleware using the `ulule/limiter` package. This middleware protects your API endpoints from abuse and ensures fair resource allocation across users and clients.

## Overview

Rate limiting provides:

- **Multiple Strategies**: Global, IP-based, User-based, and Custom key-based
- **Storage Backends**: Memory (development) and Redis (production)
- **Standard Headers**: Includes `X-RateLimit-*` headers for client awareness
- **Graceful Degradation**: Fails open if rate limiter has errors
- **Flexible Configuration**: Time period-based configuration
- **Per-Endpoint Control**: Apply different limits to different endpoints

## Core Concepts

### Rate Limit Window
A time period within which a maximum number of requests are allowed.

```go
// 100 requests per minute
middleware.IPRateLimitPeriod(100, time.Minute)

// 10 requests per second
middleware.IPRateLimitPeriod(10, time.Second)

// 1000 requests per hour
middleware.IPRateLimitPeriod(1000, time.Hour)
```

### Storage Backend
The mechanism for tracking requests:
- **Memory**: Single-instance deployments (development)
- **Redis**: Multi-instance deployments (production)

## Rate Limiting Strategies

### IP-Based Rate Limiting

Limit requests per IP address:

```go
import "github.com/iota-uz/iota-sdk/pkg/middleware"

router := mux.NewRouter()

// Limit to 100 requests per minute per IP
router.Use(middleware.IPRateLimitPeriod(100, time.Minute))
```

**Use Cases**:
- Protect public endpoints
- Prevent brute force attacks
- API protection without authentication

### User-Based Rate Limiting

Limit requests per authenticated user:

```go
// Requires user to be authenticated
router.Use(middleware.Authorize())
router.Use(middleware.UserRateLimitPeriod(1000, time.Hour))
```

**Use Cases**:
- Per-user quotas
- Fair allocation among users
- Premium vs free tier limitations

### Global Rate Limiting

Limit total requests across all clients:

```go
// Maximum 10,000 requests per minute across entire system
router.Use(middleware.GlobalRateLimitPeriod(10000, time.Minute))
```

**Use Cases**:
- Protect backend resources
- DDoS protection
- System-wide capacity management

### Custom Key-Based Rate Limiting

Limit by custom criteria:

```go
customMiddleware := middleware.RateLimit(middleware.RateLimitConfig{
    RequestsPerPeriod: 50,
    Period: time.Minute,
    KeyFunc: func(r *http.Request) string {
        // Limit by API key
        return r.Header.Get("X-API-Key")
    },
})

router.Use(customMiddleware)
```

## Configuration

### Environment Variables

```bash
# Enable/disable rate limiting
RATE_LIMIT_ENABLED=true

# Rate limit values (requests per configured period)
RATE_LIMIT_GLOBAL_RPS=1000

# Storage backend
RATE_LIMIT_STORAGE=memory          # or 'redis'
RATE_LIMIT_REDIS_URL=redis://localhost:6379
```

### Programmatic Configuration

```go
import "github.com/iota-uz/iota-sdk/pkg/configuration"

conf := configuration.Use()

// Check if rate limiting is enabled
if conf.RateLimit.Enabled {
    log.Println("Rate limiting is enabled")
}
```

## Usage Patterns

### Basic Protection

```go
func setupRouter() http.Handler {
    router := mux.NewRouter()

    // Protect entire API
    apiRouter := router.PathPrefix("/api").Subrouter()
    apiRouter.Use(middleware.IPRateLimitPeriod(100, time.Minute))

    apiRouter.HandleFunc("/users", listUsers).Methods("GET")
    apiRouter.HandleFunc("/users", createUser).Methods("POST")

    return router
}
```

### Authentication Endpoints

```go
func setupAuthRoutes(router *mux.Router) {
    // Tight limits on auth endpoints
    authRouter := router.PathPrefix("/auth").Subrouter()

    // 10 login attempts per minute per IP
    authRouter.Use(middleware.IPRateLimitPeriod(10, time.Minute))

    authRouter.HandleFunc("/login", login).Methods("POST")
    authRouter.HandleFunc("/register", register).Methods("POST")
    authRouter.HandleFunc("/password-reset", resetPassword).Methods("POST")
}
```

### Tiered User Limits

```go
func setupTieredLimits(router *mux.Router) {
    // Premium users: higher limits
    premiumRouter := router.PathPrefix("/api/premium").Subrouter()
    premiumRouter.Use(middleware.RequirePermission("premium"))
    premiumRouter.Use(middleware.UserRateLimitPeriod(5000, time.Hour))

    // Standard users: standard limits
    apiRouter := router.PathPrefix("/api").Subrouter()
    apiRouter.Use(middleware.Authorize())
    apiRouter.Use(middleware.UserRateLimitPeriod(1000, time.Hour))

    // Public endpoints: IP-based limits
    publicRouter := router.PathPrefix("/public").Subrouter()
    publicRouter.Use(middleware.IPRateLimitPeriod(100, time.Minute))
}
```

### Different Limits per Endpoint

```go
router := mux.NewRouter()

// Strict limit on payment endpoint
paymentRouter := router.PathPrefix("/api/payments").Subrouter()
paymentRouter.Use(middleware.UserRateLimitPeriod(10, time.Minute))
paymentRouter.HandleFunc("/process", processPayment).Methods("POST")

// Moderate limit on search
searchRouter := router.PathPrefix("/api/search").Subrouter()
searchRouter.Use(middleware.UserRateLimitPeriod(100, time.Minute))
searchRouter.HandleFunc("", search).Methods("GET")

// High limit on data retrieval
dataRouter := router.PathPrefix("/api/data").Subrouter()
dataRouter.Use(middleware.UserRateLimitPeriod(1000, time.Hour))
dataRouter.HandleFunc("", getData).Methods("GET")
```

## Response Headers

Rate limit headers are automatically added to all responses:

```
X-RateLimit-Limit: 100              # Maximum requests in window
X-RateLimit-Remaining: 45           # Requests remaining in current window
X-RateLimit-Reset: 1640000000       # Unix timestamp when window resets
X-RateLimit-Retry-After: 60         # Seconds until next request allowed (when limited)
```

**Client Usage**:

```javascript
// Check rate limit headers
const limit = response.headers['X-RateLimit-Limit'];
const remaining = response.headers['X-RateLimit-Remaining'];
const reset = response.headers['X-RateLimit-Reset'];

console.log(`Remaining requests: ${remaining} of ${limit}`);

if (remaining === 0) {
    const resetDate = new Date(reset * 1000);
    console.log(`Rate limited until ${resetDate}`);
}
```

## Storage Backend Configuration

### Memory Storage (Development)

```go
import "github.com/iota-uz/iota-sdk/pkg/middleware"

// Automatically used when RATE_LIMIT_STORAGE=memory
store := middleware.NewMemoryStore()

router.Use(middleware.RateLimit(middleware.RateLimitConfig{
    RequestsPerPeriod: 100,
    Period: time.Minute,
    Store: store,
}))
```

**Characteristics**:
- Fast (no network latency)
- Limited to single instance
- Memory-based tracking
- Suitable for development

### Redis Storage (Production)

```go
import "github.com/iota-uz/iota-sdk/pkg/middleware"

// Configure Redis store
store, err := middleware.NewRedisStore("redis://localhost:6379")
if err != nil {
    log.Fatal(err)
}

router.Use(middleware.RateLimit(middleware.RateLimitConfig{
    RequestsPerPeriod: 100,
    Period: time.Minute,
    Store: store,
}))
```

**Characteristics**:
- Distributed tracking
- Supports multiple instances
- Persistent tracking
- Suitable for production

## Advanced Configuration

### Custom Response Handler

```go
middleware.RateLimit(middleware.RateLimitConfig{
    RequestsPerPeriod: 50,
    Period: time.Minute,
    OnLimitReached: func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Content-Type", "application/json")
        w.WriteHeader(http.StatusTooManyRequests)
        json.NewEncoder(w).Encode(map[string]interface{}{
            "error": "Rate limit exceeded",
            "message": "Too many requests. Please try again later.",
            "retry_after": 60,
        })
    },
})
```

### Burst Allowance

```go
middleware.RateLimit(middleware.RateLimitConfig{
    RequestsPerPeriod: 50,
    Period: time.Minute,
    BurstSize: 100,  // Allow bursts up to 100 requests
    Store: store,
})
```

### Skip Rate Limiting

```go
middleware.RateLimit(middleware.RateLimitConfig{
    RequestsPerPeriod: 100,
    Period: time.Minute,
    SkipFunc: func(r *http.Request) bool {
        // Skip rate limiting for health checks
        if r.URL.Path == "/health" {
            return true
        }
        // Skip for internal requests
        if r.Header.Get("X-Internal-Request") == "true" {
            return true
        }
        return false
    },
})
```

## Error Handling

The rate limiter fails gracefully:

```go
// If rate limiter encounters error (e.g., Redis connection failure)
// the request is allowed to proceed (fail open)

// This ensures your application remains available even if
// rate limiting infrastructure has issues
```

## Testing

### Testing Rate Limits

```go
func TestRateLimit(t *testing.T) {
    suite := controllertest.New(t, module)
    suite.Register(controller)

    // Make requests up to limit
    for i := 0; i < 100; i++ {
        suite.GET("/users").
            Expect(t).
            Status(200)
    }

    // Next request should be rate limited
    response := suite.GET("/users").Expect(t)
    response.Status(429) // Too Many Requests

    // Check rate limit headers
    remaining := response.Header("X-RateLimit-Remaining")
    if remaining != "0" {
        t.Error("Expected 0 remaining requests")
    }
}
```

### Testing with Custom Keys

```go
func TestCustomKeyRateLimit(t *testing.T) {
    suite := controllertest.New(t, module)

    // Request with API key 1
    for i := 0; i < 10; i++ {
        suite.GET("/api/data").
            Header("X-API-Key", "key-1").
            Expect(t).
            Status(200)
    }

    // Request with API key 2 should still work
    suite.GET("/api/data").
        Header("X-API-Key", "key-2").
        Expect(t).
        Status(200)
}
```

## Monitoring

### Metrics Integration

```go
import "prometheus"

// Track rate limit events
rateLimitHits := prometheus.NewCounterVec(
    prometheus.CounterOpts{
        Name: "rate_limit_hits_total",
        Help: "Total number of rate limit hits",
    },
    []string{"endpoint", "limit_type"},
)

rateLimitHits.WithLabelValues("/api/users", "ip").Inc()
```

### Logging

```go
logger := composables.UseLogger(ctx)

logger.WithFields(map[string]interface{}{
    "client_ip": clientIP,
    "endpoint": endpoint,
    "remaining": remainingRequests,
}).Warn("Rate limit approaching")

logger.WithFields(map[string]interface{}{
    "client_ip": clientIP,
    "endpoint": endpoint,
    "reason": "rate_limit_exceeded",
}).Error("Request denied due to rate limit")
```

## Best Practices

1. **Set Appropriate Limits**: Balance protection with usability
   ```go
   // Too strict - users complain
   middleware.IPRateLimitPeriod(1, time.Minute)

   // Appropriate for public API
   middleware.IPRateLimitPeriod(100, time.Minute)

   // Appropriate for authenticated users
   middleware.UserRateLimitPeriod(1000, time.Hour)
   ```

2. **Use Different Limits for Different Endpoints**: Sensitive operations need stricter limits
   ```go
   // Payment endpoint: very strict
   paymentRouter.Use(middleware.UserRateLimitPeriod(10, time.Minute))

   // Search endpoint: moderate
   searchRouter.Use(middleware.UserRateLimitPeriod(100, time.Minute))

   // Data retrieval: generous
   dataRouter.Use(middleware.UserRateLimitPeriod(1000, time.Hour))
   ```

3. **Monitor Rate Limit Hits**: Track and respond to abuse
   ```go
   logger.WithField("ip", clientIP).Warn("Frequent rate limit hits")
   ```

4. **Provide Clear Error Messages**: Help clients understand the limit
   ```go
   "error": "Rate limit exceeded",
   "retry_after": 60,
   "message": "Please wait 60 seconds before trying again"
   ```

5. **Test with Realistic Scenarios**: Verify limits work as expected
   ```bash
   # Test with heavy load
   ab -n 1000 -c 100 http://localhost:8080/api/users
   ```

## Real-World Examples

### SaaS Application

```go
func setupSaaSRateLimits(router *mux.Router) {
    // Free tier: 100 req/hour
    freeRouter := router.PathPrefix("/api/free").Subrouter()
    freeRouter.Use(middleware.UserRateLimitPeriod(100, time.Hour))

    // Pro tier: 10,000 req/hour
    proRouter := router.PathPrefix("/api/pro").Subrouter()
    proRouter.Use(middleware.RequirePermission("pro"))
    proRouter.Use(middleware.UserRateLimitPeriod(10000, time.Hour))

    // Enterprise: unlimited with monitoring
    entRouter := router.PathPrefix("/api/enterprise").Subrouter()
    entRouter.Use(middleware.RequirePermission("enterprise"))
}
```

### Security Hardening

```go
func setupSecurityLimits(router *mux.Router) {
    // Auth endpoints: very restrictive
    authRouter := router.PathPrefix("/auth").Subrouter()
    authRouter.Use(middleware.IPRateLimitPeriod(5, time.Minute))

    // Admin endpoints: per-user limit
    adminRouter := router.PathPrefix("/admin").Subrouter()
    adminRouter.Use(middleware.RequirePermission("admin"))
    adminRouter.Use(middleware.UserRateLimitPeriod(100, time.Minute))

    // Public API: IP-based limit
    apiRouter := router.PathPrefix("/api/public").Subrouter()
    apiRouter.Use(middleware.IPRateLimitPeriod(100, time.Minute))
}
```

---

For more information, see the [Advanced Features Overview](./index.md) or the [Rate Limiting documentation](../rate-limiting.md).
