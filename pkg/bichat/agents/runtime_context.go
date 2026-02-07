package agents

import (
	"context"

	"github.com/google/uuid"
)

type runtimeSessionIDKey struct{}

// WithRuntimeSessionID stores the active session ID in context for tool-level scoping.
func WithRuntimeSessionID(ctx context.Context, sessionID uuid.UUID) context.Context {
	return context.WithValue(ctx, runtimeSessionIDKey{}, sessionID)
}

// UseRuntimeSessionID returns the active session ID from context.
func UseRuntimeSessionID(ctx context.Context) (uuid.UUID, bool) {
	sessionID, ok := ctx.Value(runtimeSessionIDKey{}).(uuid.UUID)
	if !ok || sessionID == uuid.Nil {
		return uuid.UUID{}, false
	}
	return sessionID, true
}
