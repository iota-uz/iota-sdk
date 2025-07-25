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

	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/configuration"
)

// RateLimitConfig holds configuration for rate limiting
type RateLimitConfig struct {
	// RequestsPerPeriod defines the number of requests allowed per time period
	RequestsPerPeriod int
	// BurstSize defines the maximum burst size (optional, defaults to RequestsPerPeriod if not set)
	BurstSize int
	// Period defines the time window for rate limiting (optional, defaults to 1 second)
	Period time.Duration
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
	// Try to get session from context first
	ctx := r.Context()
	sess, err := composables.UseSession(ctx)
	if err != nil {
		// No authenticated session, fall back to IP-based limiting
		return DefaultKeyFunc(r)
	}

	// Use user ID for rate limiting if session is available
	return fmt.Sprintf("user:%d", sess.UserID)
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
	_, _ = w.Write([]byte(`{"error":"Rate limit exceeded","message":"Too many requests"}`))
}

// NewMemoryStore creates a new in-memory store for rate limiting
func NewMemoryStore() limiter.Store {
	return memory.NewStore()
}

// NewRedisStore creates a new Redis store for rate limiting
func NewRedisStore(redisURL string) (limiter.Store, error) {
	if redisURL == "" {
		return nil, fmt.Errorf("redis URL cannot be empty")
	}

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
		config.BurstSize = config.RequestsPerPeriod
	}
	if config.Period == 0 {
		config.Period = time.Second
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

	// Create rate limit with configurable period
	rate := limiter.Rate{
		Period: config.Period,
		Limit:  int64(config.RequestsPerPeriod),
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
			w.Header().Set("X-Ratelimit-Limit", strconv.FormatInt(context.Limit, 10))
			w.Header().Set("X-Ratelimit-Remaining", strconv.FormatInt(context.Remaining, 10))
			w.Header().Set("X-Ratelimit-Reset", strconv.FormatInt(context.Reset, 10))

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

// RateLimitPeriod creates a rate limiting middleware with custom time period
func RateLimitPeriod(requests int, period time.Duration, keyFunc func(r *http.Request) string) mux.MiddlewareFunc {
	return RateLimit(RateLimitConfig{
		RequestsPerPeriod: requests,
		Period:            period,
		KeyFunc:           keyFunc,
	})
}

// GlobalRateLimitPeriod creates a global rate limiting middleware with custom time period
func GlobalRateLimitPeriod(requests int, period time.Duration) mux.MiddlewareFunc {
	return RateLimitPeriod(requests, period, func(r *http.Request) string {
		return "global"
	})
}

// IPRateLimitPeriod creates an IP-based rate limiting middleware with custom time period
func IPRateLimitPeriod(requests int, period time.Duration) mux.MiddlewareFunc {
	return RateLimitPeriod(requests, period, DefaultKeyFunc)
}

// UserRateLimitPeriod creates a user-based rate limiting middleware with custom time period
func UserRateLimitPeriod(requests int, period time.Duration) mux.MiddlewareFunc {
	return RateLimitPeriod(requests, period, UserKeyFunc)
}
