package middleware

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultKeyFunc(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "192.168.1.1:12345"

	key := DefaultKeyFunc(req)
	assert.Equal(t, "192.168.1.1:12345", key)
}

func TestDefaultKeyFuncWithRealIPHeader(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "192.168.1.1:12345"
	req.Header.Set("X-Real-IP", "10.0.0.1")

	key := DefaultKeyFunc(req)
	assert.Equal(t, "10.0.0.1", key)
}

func TestEndpointKeyFunc(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/users", nil)
	req.RemoteAddr = "192.168.1.1:12345"

	keyFunc := EndpointKeyFunc("/api/users")
	key := keyFunc(req)
	assert.Equal(t, "/api/users:192.168.1.1:12345", key)
}

func TestNewMemoryStore(t *testing.T) {
	store := NewMemoryStore()
	assert.NotNil(t, store)
}

func TestIPRateLimit(t *testing.T) {
	// Create a test handler
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	})

	// Create router with rate limiting middleware (5 requests per second)
	router := mux.NewRouter()
	router.Use(IPRateLimit(5))
	router.HandleFunc("/test", handler)

	// Test that first few requests succeed
	for i := 0; i < 5; i++ {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.RemoteAddr = "192.168.1.1:12345"

		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Equal(t, "OK", rr.Body.String())

		// Check rate limit headers
		assert.Equal(t, "5", rr.Header().Get("X-Ratelimit-Limit"))
		remaining, err := strconv.Atoi(rr.Header().Get("X-Ratelimit-Remaining"))
		require.NoError(t, err)
		assert.GreaterOrEqual(t, remaining, 0)
	}

	// Test that subsequent request is rate limited
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.RemoteAddr = "192.168.1.1:12345"

	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusTooManyRequests, rr.Code)
	assert.Contains(t, rr.Body.String(), "Rate limit exceeded")
}

func TestIPRateLimitDifferentIPs(t *testing.T) {
	// Create a test handler
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	})

	// Create router with rate limiting middleware (2 requests per second)
	router := mux.NewRouter()
	router.Use(IPRateLimit(2))
	router.HandleFunc("/test", handler)

	// Test requests from different IPs can each make requests up to the limit
	ips := []string{"192.168.1.1:12345", "192.168.1.2:12346", "192.168.1.3:12347"}

	for _, ip := range ips {
		for i := 0; i < 2; i++ {
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			req.RemoteAddr = ip

			rr := httptest.NewRecorder()
			router.ServeHTTP(rr, req)

			assert.Equal(t, http.StatusOK, rr.Code, "IP %s request %d should succeed", ip, i+1)
		}

		// Third request from same IP should be rate limited
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.RemoteAddr = ip

		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusTooManyRequests, rr.Code, "IP %s third request should be rate limited", ip)
	}
}

func TestGlobalRateLimit(t *testing.T) {
	// Create a test handler
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	})

	// Create router with global rate limiting middleware (3 requests per second total)
	router := mux.NewRouter()
	router.Use(GlobalRateLimit(3))
	router.HandleFunc("/test", handler)

	// Test that first 3 requests succeed from different IPs
	ips := []string{"192.168.1.1:12345", "192.168.1.2:12346", "192.168.1.3:12347"}

	for i, ip := range ips {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.RemoteAddr = ip

		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code, "Request %d from IP %s should succeed", i+1, ip)
	}

	// Fourth request should be rate limited even from a new IP
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.RemoteAddr = "192.168.1.4:12348"

	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusTooManyRequests, rr.Code, "Fourth request should be globally rate limited")
}

func TestEndpointRateLimit(t *testing.T) {
	// Create test handlers
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	})

	// Create router with endpoint-specific rate limiting
	router := mux.NewRouter()
	apiRouter := router.PathPrefix("/api").Subrouter()
	apiRouter.Use(EndpointRateLimit("/api", 2))
	apiRouter.HandleFunc("/users", handler)

	authRouter := router.PathPrefix("/auth").Subrouter()
	authRouter.Use(EndpointRateLimit("/auth", 1))
	authRouter.HandleFunc("/login", handler)

	ip := "192.168.1.1:12345"

	// Test /api endpoint allows 2 requests
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodGet, "/api/users", nil)
		req.RemoteAddr = ip

		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code, "API request %d should succeed", i+1)
	}

	// Third API request should be rate limited
	req := httptest.NewRequest(http.MethodGet, "/api/users", nil)
	req.RemoteAddr = ip

	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusTooManyRequests, rr.Code, "Third API request should be rate limited")

	// But /auth endpoint should still work (separate limit)
	req = httptest.NewRequest(http.MethodPost, "/auth/login", nil)
	req.RemoteAddr = ip

	rr = httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code, "Auth request should succeed")
}

func TestAPIRateLimit(t *testing.T) {
	// Create a test handler
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	})

	// Test different RPS values
	testCases := []struct {
		name string
		rps  int
	}{
		{"Low RPS", 1},
		{"Medium RPS", 10},
		{"High RPS", 100},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			router := mux.NewRouter()
			router.Use(APIRateLimit(tc.rps))
			router.HandleFunc("/test", handler)

			// Test that requests up to the limit succeed
			for i := 0; i < tc.rps; i++ {
				req := httptest.NewRequest(http.MethodGet, "/test", nil)
				req.RemoteAddr = "192.168.1.1:12345"

				rr := httptest.NewRecorder()
				router.ServeHTTP(rr, req)

				assert.Equal(t, http.StatusOK, rr.Code, "Request %d should succeed for %d RPS", i+1, tc.rps)
				assert.Equal(t, strconv.Itoa(tc.rps), rr.Header().Get("X-Ratelimit-Limit"))
			}

			// Test that subsequent request is rate limited
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			req.RemoteAddr = "192.168.1.1:12345"

			rr := httptest.NewRecorder()
			router.ServeHTTP(rr, req)

			assert.Equal(t, http.StatusTooManyRequests, rr.Code, "Request beyond limit should be rate limited for %d RPS", tc.rps)
		})
	}
}

func TestCustomRateLimitConfig(t *testing.T) {
	// Create a test handler
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	})

	// Custom on limit reached handler
	customLimitReached := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte("Custom rate limit message"))
	}

	// Custom key function (always returns same key for testing)
	customKeyFunc := func(r *http.Request) string {
		return "test-key"
	}

	// Create router with custom rate limiting configuration
	router := mux.NewRouter()
	router.Use(RateLimit(RateLimitConfig{
		RequestsPerSecond: 2,
		BurstSize:         3,
		KeyFunc:           customKeyFunc,
		OnLimitReached:    customLimitReached,
	}))
	router.HandleFunc("/test", handler)

	// Test that first 2 requests succeed
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.RemoteAddr = "192.168.1.1:12345"

		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
	}

	// Third request should trigger custom rate limit response
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.RemoteAddr = "192.168.1.1:12345"

	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusTooManyRequests, rr.Code)
	assert.Equal(t, "text/plain", rr.Header().Get("Content-Type"))
	assert.Equal(t, "Custom rate limit message", rr.Body.String())
}

func TestRateLimitHeaders(t *testing.T) {
	// Create a test handler
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	})

	// Create router with rate limiting middleware
	router := mux.NewRouter()
	router.Use(IPRateLimit(5))
	router.HandleFunc("/test", handler)

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.RemoteAddr = "192.168.1.1:12345"

	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	// Check that rate limit headers are present
	assert.Equal(t, "5", rr.Header().Get("X-Ratelimit-Limit"))

	remaining := rr.Header().Get("X-Ratelimit-Remaining")
	assert.NotEmpty(t, remaining)
	remainingInt, err := strconv.Atoi(remaining)
	require.NoError(t, err)
	assert.True(t, remainingInt >= 0 && remainingInt <= 5)

	reset := rr.Header().Get("X-Ratelimit-Reset")
	assert.NotEmpty(t, reset)
	resetInt, err := strconv.ParseInt(reset, 10, 64)
	require.NoError(t, err)
	assert.Greater(t, resetInt, time.Now().Unix())
}

func TestUserRateLimit(t *testing.T) {
	// Create a test handler
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	})

	// Create router with user rate limiting middleware
	router := mux.NewRouter()
	router.Use(UserRateLimit(3))
	router.HandleFunc("/test", handler)

	// Since UserRateLimit currently falls back to IP-based limiting,
	// this should behave the same as IPRateLimit
	ip := "192.168.1.1:12345"

	// Test that first 3 requests succeed
	for i := 0; i < 3; i++ {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.RemoteAddr = ip

		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code, "Request %d should succeed", i+1)
	}

	// Fourth request should be rate limited
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.RemoteAddr = ip

	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusTooManyRequests, rr.Code, "Fourth request should be rate limited")
}

func TestRateLimitWithCustomPeriod(t *testing.T) {
	// Create a test handler
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	})

	// Create router with custom period rate limiting (1000 requests per 30 minutes)
	router := mux.NewRouter()
	router.Use(IPRateLimitPeriod(1000, 30*time.Minute))
	router.HandleFunc("/test", handler)

	ip := "192.168.1.1:12345"

	// Test that requests succeed initially
	for i := 0; i < 5; i++ {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.RemoteAddr = ip

		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code, "Request %d should succeed", i+1)

		// Check that rate limit is set correctly (1000 for the 30-minute window)
		assert.Equal(t, "1000", rr.Header().Get("X-Ratelimit-Limit"))
	}
}

func TestGlobalRateLimitWithCustomPeriod(t *testing.T) {
	// Create a test handler
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	})

	// Create router with custom period global rate limiting (100 requests per 10 minutes)
	router := mux.NewRouter()
	router.Use(GlobalRateLimitPeriod(100, 10*time.Minute))
	router.HandleFunc("/test", handler)

	// Test requests from different IPs share the same global limit
	ips := []string{"192.168.1.1:12345", "192.168.1.2:12346", "192.168.1.3:12347"}

	for i, ip := range ips {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.RemoteAddr = ip

		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code, "Request %d from IP %s should succeed", i+1, ip)

		// Check that global rate limit is set correctly
		assert.Equal(t, "100", rr.Header().Get("X-Ratelimit-Limit"))
	}
}

func TestEndpointRateLimitWithCustomPeriod(t *testing.T) {
	// Create a test handler
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	})

	// Create router with custom period endpoint rate limiting (500 requests per 15 minutes)
	router := mux.NewRouter()
	apiRouter := router.PathPrefix("/api").Subrouter()
	apiRouter.Use(EndpointRateLimitPeriod("/api", 500, 15*time.Minute))
	apiRouter.HandleFunc("/users", handler)

	ip := "192.168.1.1:12345"

	// Test that requests succeed and have correct limit
	for i := 0; i < 3; i++ {
		req := httptest.NewRequest(http.MethodGet, "/api/users", nil)
		req.RemoteAddr = ip

		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code, "Request %d should succeed", i+1)

		// Check that endpoint rate limit is set correctly
		assert.Equal(t, "500", rr.Header().Get("X-Ratelimit-Limit"))
	}
}
