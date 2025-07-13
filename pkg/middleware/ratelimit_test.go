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
	router.Use(IPRateLimitPeriod(5, time.Second))
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
