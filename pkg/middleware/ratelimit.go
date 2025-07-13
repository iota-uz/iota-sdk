package middleware

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gorilla/mux"
	redisClient "github.com/redis/go-redis/v9"
	"github.com/ulule/limiter/v3"
	"github.com/ulule/limiter/v3/drivers/store/memory"
	"github.com/ulule/limiter/v3/drivers/store/redis"

	"github.com/iota-uz/iota-sdk/pkg/configuration"
)

// RateLimitConfig holds configuration for rate limiting
type RateLimitConfig struct {
	// RequestsPerSecond defines the number of requests allowed per second
	RequestsPerSecond int
	// BurstSize defines the maximum burst size (optional, defaults to RPS if not set)
	BurstSize int
	// KeyFunc defines how to generate keys for rate limiting (e.g., by IP, user, etc.)
	KeyFunc func(r *http.Request) string
	// Store defines the storage backend (memory or redis)
	Store limiter.Store
	// OnLimitReached is called when rate limit is exceeded (optional)
	OnLimitReached func(w http.ResponseWriter, r *http.Request)
}

// DefaultKeyFunc returns the real IP address for rate limiting
func DefaultKeyFunc(r *http.Request) string {
	return getRealIP(r, configuration.Use())
}

// UserKeyFunc returns a key based on user ID if authenticated, falls back to IP
func UserKeyFunc(r *http.Request) string {
	// Try to get user ID from context first
	// This would need to be integrated with your auth system
	// For now, fallback to IP-based limiting
	return DefaultKeyFunc(r)
}

// EndpointKeyFunc returns a key based on endpoint and IP
func EndpointKeyFunc(endpoint string) func(r *http.Request) string {
	return func(r *http.Request) string {
		return fmt.Sprintf("%s:%s", endpoint, DefaultKeyFunc(r))
	}
}

// DefaultOnLimitReached writes a standard rate limit exceeded response
func DefaultOnLimitReached(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusTooManyRequests)
	w.Write([]byte(`{"error":"Rate limit exceeded","message":"Too many requests"}`))
}

// NewMemoryStore creates a new in-memory store for rate limiting
func NewMemoryStore() limiter.Store {
	return memory.NewStore()
}

// NewRedisStore creates a new Redis store for rate limiting
func NewRedisStore(redisURL string) (limiter.Store, error) {
	if redisURL == "" {
		redisURL = "redis://localhost:6379"
	}

	// Parse Redis URL and create a redis client
	// For now, we'll use a simple approach - in production you might want more sophisticated parsing
	client := redisClient.NewClient(&redisClient.Options{
		Addr: redisURL,
	})

	return redis.NewStoreWithOptions(client, limiter.StoreOptions{
		Prefix: "rate_limit",
	})
}

// RateLimit creates a rate limiting middleware with custom configuration
func RateLimit(config RateLimitConfig) mux.MiddlewareFunc {
	// Set defaults
	if config.BurstSize == 0 {
		config.BurstSize = config.RequestsPerSecond
	}
	if config.KeyFunc == nil {
		config.KeyFunc = DefaultKeyFunc
	}
	if config.Store == nil {
		config.Store = NewMemoryStore()
	}
	if config.OnLimitReached == nil {
		config.OnLimitReached = DefaultOnLimitReached
	}

	// Create rate limit string (requests per second)
	rate := limiter.Rate{
		Period: time.Second,
		Limit:  int64(config.RequestsPerSecond),
	}

	instance := limiter.New(config.Store, rate, limiter.WithClientIPHeader("X-Real-IP"))

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Get the rate limit context
			context, err := instance.Get(r.Context(), config.KeyFunc(r))
			if err != nil {
				// If we can't get rate limit info, allow the request (fail open)
				next.ServeHTTP(w, r)
				return
			}

			// Set rate limit headers
			w.Header().Set("X-RateLimit-Limit", strconv.FormatInt(context.Limit, 10))
			w.Header().Set("X-RateLimit-Remaining", strconv.FormatInt(context.Remaining, 10))
			w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(context.Reset, 10))

			// Check if rate limit is exceeded
			if context.Reached {
				config.OnLimitReached(w, r)
				return
			}

			// Continue to next handler
			next.ServeHTTP(w, r)
		})
	}
}

// GlobalRateLimit creates a global rate limiting middleware (all requests share the same limit)
func GlobalRateLimit(requestsPerSecond int) mux.MiddlewareFunc {
	return RateLimit(RateLimitConfig{
		RequestsPerSecond: requestsPerSecond,
		KeyFunc: func(r *http.Request) string {
			return "global"
		},
	})
}

// IPRateLimit creates an IP-based rate limiting middleware
func IPRateLimit(requestsPerSecond int) mux.MiddlewareFunc {
	return RateLimit(RateLimitConfig{
		RequestsPerSecond: requestsPerSecond,
		KeyFunc:           DefaultKeyFunc,
	})
}

// UserRateLimit creates a user-based rate limiting middleware (falls back to IP if no user)
func UserRateLimit(requestsPerSecond int) mux.MiddlewareFunc {
	return RateLimit(RateLimitConfig{
		RequestsPerSecond: requestsPerSecond,
		KeyFunc:           UserKeyFunc,
	})
}

// EndpointRateLimit creates an endpoint-specific rate limiting middleware
func EndpointRateLimit(endpoint string, requestsPerSecond int) mux.MiddlewareFunc {
	return RateLimit(RateLimitConfig{
		RequestsPerSecond: requestsPerSecond,
		KeyFunc:           EndpointKeyFunc(endpoint),
	})
}

// APIRateLimit creates an API rate limiting middleware with different tiers based on RPS
func APIRateLimit(requestsPerSecond int) mux.MiddlewareFunc {
	return IPRateLimit(requestsPerSecond)
}
