package applet

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/csrf"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewStreamWriter_Success tests successful StreamWriter creation
func TestNewStreamWriter_Success(t *testing.T) {
	t.Parallel()

	w := httptest.NewRecorder()
	sw, err := NewStreamWriter(w)

	require.NoError(t, err)
	require.NotNil(t, sw)

	// Verify SSE headers are set
	assert.Equal(t, "text/event-stream", w.Header().Get("Content-Type"))
	assert.Equal(t, "no-cache", w.Header().Get("Cache-Control"))
	assert.Equal(t, "keep-alive", w.Header().Get("Connection"))
	assert.Equal(t, "no", w.Header().Get("X-Accel-Buffering"))
}

// TestNewStreamWriter_NoFlusher tests error when ResponseWriter doesn't support flushing
func TestNewStreamWriter_NoFlusher(t *testing.T) {
	t.Parallel()

	// Custom ResponseWriter that doesn't implement http.Flusher
	type nonFlusher struct {
		http.ResponseWriter
	}

	w := &nonFlusher{ResponseWriter: httptest.NewRecorder()}
	sw, err := NewStreamWriter(w)

	require.Error(t, err)
	assert.Nil(t, sw)
	assert.Contains(t, err.Error(), "does not support flushing")
}

// TestStreamWriter_WriteEvent tests writing SSE events
func TestStreamWriter_WriteEvent(t *testing.T) {
	t.Parallel()

	w := httptest.NewRecorder()
	sw, err := NewStreamWriter(w)
	require.NoError(t, err)

	err = sw.WriteEvent("message", "Hello, World!")
	require.NoError(t, err)

	output := w.Body.String()
	assert.Equal(t, "event: message\ndata: Hello, World!\n\n", output)
}

// TestStreamWriter_WriteJSON tests writing JSON-encoded SSE events
func TestStreamWriter_WriteJSON(t *testing.T) {
	t.Parallel()

	w := httptest.NewRecorder()
	sw, err := NewStreamWriter(w)
	require.NoError(t, err)

	data := map[string]interface{}{
		"status":  "processing",
		"percent": 50,
	}

	err = sw.WriteJSON("update", data)
	require.NoError(t, err)

	output := w.Body.String()
	assert.Contains(t, output, "event: update\n")
	assert.Contains(t, output, "data: ")
	assert.Contains(t, output, `"status":"processing"`)
	assert.Contains(t, output, `"percent":50`)
}

// TestStreamWriter_WriteDone tests writing done event
func TestStreamWriter_WriteDone(t *testing.T) {
	t.Parallel()

	w := httptest.NewRecorder()
	sw, err := NewStreamWriter(w)
	require.NoError(t, err)

	err = sw.WriteDone()
	require.NoError(t, err)

	output := w.Body.String()
	assert.Equal(t, "event: done\ndata: \n\n", output)
}

// TestStreamWriter_WriteError tests writing error event
func TestStreamWriter_WriteError(t *testing.T) {
	t.Parallel()

	w := httptest.NewRecorder()
	sw, err := NewStreamWriter(w)
	require.NoError(t, err)

	err = sw.WriteError("Failed to process request")
	require.NoError(t, err)

	output := w.Body.String()
	assert.Equal(t, "event: error\ndata: Failed to process request\n\n", output)
}

// TestStreamWriter_WriteErrorJSON tests writing JSON-encoded error event
func TestStreamWriter_WriteErrorJSON(t *testing.T) {
	t.Parallel()

	w := httptest.NewRecorder()
	sw, err := NewStreamWriter(w)
	require.NoError(t, err)

	errData := map[string]string{
		"code":    "INVALID_INPUT",
		"message": "Invalid session ID",
	}

	err = sw.WriteErrorJSON(errData)
	require.NoError(t, err)

	output := w.Body.String()
	assert.Contains(t, output, "event: error\n")
	assert.Contains(t, output, `"code":"INVALID_INPUT"`)
	assert.Contains(t, output, `"message":"Invalid session ID"`)
}

// TestStreamWriter_WriteComment tests writing SSE comments
func TestStreamWriter_WriteComment(t *testing.T) {
	t.Parallel()

	w := httptest.NewRecorder()
	sw, err := NewStreamWriter(w)
	require.NoError(t, err)

	err = sw.WriteComment("keep-alive")
	require.NoError(t, err)

	output := w.Body.String()
	assert.Equal(t, ": keep-alive\n", output)
}

// TestStreamWriter_MultipleEvents tests writing multiple events in sequence
func TestStreamWriter_MultipleEvents(t *testing.T) {
	t.Parallel()

	w := httptest.NewRecorder()
	sw, err := NewStreamWriter(w)
	require.NoError(t, err)

	// Write multiple events
	err = sw.WriteEvent("start", "Beginning")
	require.NoError(t, err)

	err = sw.WriteJSON("update", map[string]int{"progress": 50})
	require.NoError(t, err)

	err = sw.WriteEvent("complete", "Finished")
	require.NoError(t, err)

	err = sw.WriteDone()
	require.NoError(t, err)

	output := w.Body.String()
	events := strings.Split(strings.TrimSpace(output), "\n\n")
	assert.Len(t, events, 4) // 4 events
}

// TestStreamContextBuilder_Build_Success tests successful StreamContext building
func TestStreamContextBuilder_Build_Success(t *testing.T) {
	t.Parallel()

	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)
	sessionConfig := DefaultSessionConfig

	config := Config{
		Endpoints: EndpointConfig{
			GraphQL: "/graphql",
			Stream:  "/stream",
		},
	}

	builder := NewStreamContextBuilder(config, sessionConfig, logger)

	ctx := createTestContext(t,
		withUserID(42),
		withPermissions("bichat.access", "finance.read"),
	)

	r := httptest.NewRequest(http.MethodGet, "/stream", nil)

	// Apply CSRF middleware
	csrfMiddleware := csrf.Protect(
		[]byte("32-byte-long-secret-key-for-testing!"),
		csrf.Secure(false),
	)
	handler := csrfMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		streamCtx, err := builder.Build(ctx, r)
		require.NoError(t, err)
		require.NotNil(t, streamCtx)

		// Verify lightweight context
		assert.Equal(t, int64(42), streamCtx.UserID)
		assert.NotEmpty(t, streamCtx.TenantID)
		assert.Contains(t, streamCtx.Permissions, "bichat.access")
		assert.Contains(t, streamCtx.Permissions, "finance.read")
		assert.NotEmpty(t, streamCtx.CSRFToken)

		// Verify session context
		assert.Greater(t, streamCtx.Session.ExpiresAt, time.Now().UnixMilli())
		assert.Equal(t, "/auth/refresh", streamCtx.Session.RefreshURL)

		// Verify NO translations (lightweight)
		assert.Nil(t, streamCtx.Extensions) // No custom context by default
	}))

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)
}

// TestStreamContextBuilder_Build_MissingUser tests error when user is missing
func TestStreamContextBuilder_Build_MissingUser(t *testing.T) {
	t.Parallel()

	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)
	sessionConfig := DefaultSessionConfig

	config := Config{}
	builder := NewStreamContextBuilder(config, sessionConfig, logger)

	// Context without user
	ctx := context.Background()
	r := httptest.NewRequest(http.MethodGet, "/stream", nil)

	streamCtx, err := builder.Build(ctx, r)
	require.Error(t, err)
	assert.Nil(t, streamCtx)
	assert.Contains(t, err.Error(), "user extraction failed")
}

// TestStreamContextBuilder_Build_MissingTenant tests error when tenant is missing
func TestStreamContextBuilder_Build_MissingTenant(t *testing.T) {
	t.Parallel()

	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)
	sessionConfig := DefaultSessionConfig

	config := Config{}
	builder := NewStreamContextBuilder(config, sessionConfig, logger)

	// Create context with user but no tenant (Background so tenant extraction fails)
	ctx := context.Background()
	ctx = composables.WithUser(ctx, &mockUser{
		id:        123,
		firstName: "John",
		lastName:  "Doe",
	})

	r := httptest.NewRequest(http.MethodGet, "/stream", nil)

	streamCtx, err := builder.Build(ctx, r)
	require.Error(t, err)
	assert.Nil(t, streamCtx)
	assert.Contains(t, err.Error(), "tenant extraction failed")
}

// TestStreamContextBuilder_Build_WithCustomContext tests custom context extender
func TestStreamContextBuilder_Build_WithCustomContext(t *testing.T) {
	t.Parallel()

	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)
	sessionConfig := DefaultSessionConfig

	config := Config{
		CustomContext: func(ctx context.Context) (map[string]interface{}, error) {
			return map[string]interface{}{
				"sessionID": "abc123",
				"metadata":  "stream-specific",
			}, nil
		},
	}

	builder := NewStreamContextBuilder(config, sessionConfig, logger)

	ctx := createTestContext(t, withUserID(42))
	r := httptest.NewRequest(http.MethodGet, "/stream", nil)

	// Apply CSRF middleware
	csrfMiddleware := csrf.Protect(
		[]byte("32-byte-long-secret-key-for-testing!"),
		csrf.Secure(false),
	)
	handler := csrfMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		streamCtx, err := builder.Build(ctx, r)
		require.NoError(t, err)
		require.NotNil(t, streamCtx)

		// Verify custom context is included
		assert.NotNil(t, streamCtx.Extensions)
		assert.Equal(t, "abc123", streamCtx.Extensions["sessionID"])
		assert.Equal(t, "stream-specific", streamCtx.Extensions["metadata"])
	}))

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)
}

// TestStreamContextBuilder_Build_Performance tests that stream context builds fast
func TestStreamContextBuilder_Build_Performance(t *testing.T) {
	t.Parallel()

	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)
	sessionConfig := DefaultSessionConfig

	config := Config{}
	builder := NewStreamContextBuilder(config, sessionConfig, logger)

	ctx := createTestContext(t,
		withUserID(42),
		withPermissions("bichat.access", "finance.read"),
	)

	r := httptest.NewRequest(http.MethodGet, "/stream", nil)

	// Apply CSRF middleware
	csrfMiddleware := csrf.Protect(
		[]byte("32-byte-long-secret-key-for-testing!"),
		csrf.Secure(false),
	)
	handler := csrfMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		_, err := builder.Build(ctx, r)
		duration := time.Since(start)

		assert.NoError(t, err)

		// Performance target: <5ms
		assert.Less(t, duration.Milliseconds(), int64(5),
			"Stream context build should complete in <5ms, took %dms", duration.Milliseconds())
	}))

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)
}

// TestStreamContextBuilder_Build_WithCustomContextError tests error handling in custom context
func TestStreamContextBuilder_Build_WithCustomContextError(t *testing.T) {
	t.Parallel()

	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)
	sessionConfig := DefaultSessionConfig

	config := Config{
		CustomContext: func(ctx context.Context) (map[string]interface{}, error) {
			return nil, assert.AnError
		},
	}

	builder := NewStreamContextBuilder(config, sessionConfig, logger)

	ctx := createTestContext(t, withUserID(42))
	r := httptest.NewRequest(http.MethodGet, "/stream", nil)

	// Apply CSRF middleware
	csrfMiddleware := csrf.Protect(
		[]byte("32-byte-long-secret-key-for-testing!"),
		csrf.Secure(false),
	)
	handler := csrfMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Should succeed but log warning (error is swallowed)
		streamCtx, err := builder.Build(ctx, r)
		require.NoError(t, err)
		require.NotNil(t, streamCtx)

		// Extensions should be nil due to error
		assert.Nil(t, streamCtx.Extensions)
	}))

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)
}

// TestStreamContextBuilder_VerifyLightweight tests that stream context excludes heavy fields
func TestStreamContextBuilder_VerifyLightweight(t *testing.T) {
	t.Parallel()

	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)
	sessionConfig := DefaultSessionConfig

	config := Config{}
	builder := NewStreamContextBuilder(config, sessionConfig, logger)

	ctx := createTestContext(t,
		withUserID(42),
		withPermissions("bichat.access"),
	)

	r := httptest.NewRequest(http.MethodGet, "/stream", nil)

	// Apply CSRF middleware
	csrfMiddleware := csrf.Protect(
		[]byte("32-byte-long-secret-key-for-testing!"),
		csrf.Secure(false),
	)
	handler := csrfMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		streamCtx, err := builder.Build(ctx, r)
		require.NoError(t, err)
		require.NotNil(t, streamCtx)

		// Verify StreamContext is truly lightweight
		// It should only have: UserID, TenantID, Permissions, CSRFToken, Session
		// NO: Translations, Locale, Routes, User details (email, name)
		assert.NotZero(t, streamCtx.UserID)
		assert.NotEmpty(t, streamCtx.TenantID)
		assert.NotEmpty(t, streamCtx.Permissions)
		assert.NotEmpty(t, streamCtx.CSRFToken)
		assert.NotZero(t, streamCtx.Session.ExpiresAt)

		// Verify no heavy fields
		assert.Nil(t, streamCtx.Extensions) // No custom context by default
	}))

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)
}
