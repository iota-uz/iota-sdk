package applet

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/gorilla/csrf"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
	"github.com/sirupsen/logrus"
)

// StreamWriter provides utilities for Server-Sent Events (SSE) streaming.
// It handles proper SSE formatting and flushing for real-time communication
// with React/Next.js applets.
type StreamWriter struct {
	w       http.ResponseWriter
	flusher http.Flusher
}

// NewStreamWriter creates a new StreamWriter from an http.ResponseWriter.
// It sets appropriate SSE headers and verifies that streaming is supported.
// Returns an error if the ResponseWriter doesn't support flushing (required for SSE).
func NewStreamWriter(w http.ResponseWriter) (*StreamWriter, error) {
	const op serrors.Op = "NewStreamWriter"

	// Verify flusher support
	flusher, ok := w.(http.Flusher)
	if !ok {
		return nil, serrors.E(op, serrors.Internal, "http.ResponseWriter does not support flushing")
	}

	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no") // Disable nginx buffering

	return &StreamWriter{
		w:       w,
		flusher: flusher,
	}, nil
}

// WriteEvent writes a Server-Sent Event with the given event type and data.
// Format: event: <event>\ndata: <data>\n\n
//
// Example:
//
//	sw.WriteEvent("message", "Hello, world!")
//	// Sends: event: message\ndata: Hello, world!\n\n
func (sw *StreamWriter) WriteEvent(event, data string) error {
	const op serrors.Op = "StreamWriter.WriteEvent"

	_, err := fmt.Fprintf(sw.w, "event: %s\ndata: %s\n\n", event, data)
	if err != nil {
		return serrors.E(op, err)
	}

	sw.flusher.Flush()
	return nil
}

// WriteJSON writes a Server-Sent Event with JSON-encoded data.
// Format: event: <event>\ndata: <json>\n\n
//
// Example:
//
//	sw.WriteJSON("update", map[string]string{"status": "processing"})
//	// Sends: event: update\ndata: {"status":"processing"}\n\n
func (sw *StreamWriter) WriteJSON(event string, data interface{}) error {
	const op serrors.Op = "StreamWriter.WriteJSON"

	// Marshal data to JSON
	jsonData, err := json.Marshal(data)
	if err != nil {
		return serrors.E(op, serrors.Internal, "failed to marshal JSON", err)
	}

	return sw.WriteEvent(event, string(jsonData))
}

// WriteDone writes a "done" event to signal the end of the stream.
// This is a convention used by many SSE clients to know when to stop listening.
//
// Example:
//
//	sw.WriteDone()
//	// Sends: event: done\ndata: \n\n
func (sw *StreamWriter) WriteDone() error {
	return sw.WriteEvent("done", "")
}

// WriteError writes an "error" event with the given error message.
// This allows clients to handle errors gracefully.
//
// Example:
//
//	sw.WriteError("Failed to process request")
//	// Sends: event: error\ndata: Failed to process request\n\n
func (sw *StreamWriter) WriteError(errMsg string) error {
	return sw.WriteEvent("error", errMsg)
}

// WriteErrorJSON writes an "error" event with JSON-encoded error details.
//
// Example:
//
//	sw.WriteErrorJSON(map[string]string{
//	    "code": "INVALID_INPUT",
//	    "message": "Invalid session ID",
//	})
//	// Sends: event: error\ndata: {"code":"INVALID_INPUT","message":"Invalid session ID"}\n\n
func (sw *StreamWriter) WriteErrorJSON(errData interface{}) error {
	return sw.WriteJSON("error", errData)
}

// WriteComment writes an SSE comment (ignored by clients, useful for keeping connection alive).
// Format: : <comment>\n
//
// Example:
//
//	sw.WriteComment("keep-alive")
//	// Sends: : keep-alive\n
func (sw *StreamWriter) WriteComment(comment string) error {
	const op serrors.Op = "StreamWriter.WriteComment"

	_, err := fmt.Fprintf(sw.w, ": %s\n", comment)
	if err != nil {
		return serrors.E(op, err)
	}

	sw.flusher.Flush()
	return nil
}

// StreamContextBuilder builds lightweight context for SSE streaming endpoints.
// Excludes heavy fields like translations, full locale, routes for optimal performance.
type StreamContextBuilder struct {
	config        Config
	logger        *logrus.Logger
	sessionConfig SessionConfig
}

// NewStreamContextBuilder creates a StreamContextBuilder.
// Does not require i18n bundle since translations are excluded for performance.
//
// Parameters:
//   - config: Applet configuration (primarily for CustomContext extender)
//   - sessionConfig: Session expiry and refresh configuration
//   - logger: Structured logger for operations
func NewStreamContextBuilder(
	config Config,
	sessionConfig SessionConfig,
	logger *logrus.Logger,
) *StreamContextBuilder {
	return &StreamContextBuilder{
		config:        config,
		logger:        logger,
		sessionConfig: sessionConfig,
	}
}

// Build builds lightweight StreamContext for SSE endpoints.
// Excludes heavy fields like translations, full locale, routes.
//
// Performance target: <5ms (much faster than full InitialContext)
// Logs: Entry/exit with user/tenant/duration
//
// Returns only essential data:
//   - User ID (for authentication)
//   - Tenant ID (for data isolation)
//   - Permissions (for authorization)
//   - CSRF token (for security)
//   - Session context (for expiry handling)
//   - Extensions (optional custom data)
func (b *StreamContextBuilder) Build(ctx context.Context, r *http.Request) (*StreamContext, error) {
	const op serrors.Op = "StreamContextBuilder.Build"
	start := time.Now()

	// Extract user
	user, err := composables.UseUser(ctx)
	if err != nil {
		if b.logger != nil {
			b.logger.WithError(err).Error("Failed to extract user for stream context")
		}
		return nil, serrors.E(op, serrors.Internal, "user extraction failed", err)
	}

	// Extract tenant ID
	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		if b.logger != nil {
			b.logger.WithError(err).WithField("user_id", user.ID()).Error("Failed to extract tenant ID")
		}
		return nil, serrors.E(op, serrors.Internal, "tenant extraction failed", err)
	}

	// Get permissions (validated)
	permissions := getUserPermissions(ctx)
	permissions = validatePermissions(permissions)

	// Build session context
	// StreamContextBuilder doesn't support SessionStore, use default config expiry
	session := buildSessionContext(r, b.sessionConfig, nil)

	streamCtx := &StreamContext{
		UserID:      int64(user.ID()),
		TenantID:    tenantID.String(),
		Permissions: permissions,
		CSRFToken:   csrf.Token(r),
		Session:     session,
	}

	// Apply custom context extender if provided
	if b.config.CustomContext != nil {
		customData, err := b.config.CustomContext(ctx)
		if err != nil {
			if b.logger != nil {
				b.logger.WithError(err).Warn("Failed to build custom stream context")
			}
		} else if customData != nil {
			// Sanitize custom data to prevent XSS
			streamCtx.Extensions = sanitizeForJSON(customData)
		}
	}

	// Log build completion
	buildDuration := time.Since(start)
	if b.logger != nil {
		b.logger.WithFields(logrus.Fields{
			"user_id":     user.ID(),
			"tenant_id":   tenantID.String(),
			"duration_ms": buildDuration.Milliseconds(),
		}).Debug("Built stream context")
	}

	return streamCtx, nil
}
