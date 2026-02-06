package services

import (
	"context"

	"github.com/google/uuid"
)

type debugModeContextKey struct{}
type artifactMessageIDContextKey struct{}

// WithDebugMode marks request context for debug-oriented model behavior.
func WithDebugMode(ctx context.Context, enabled bool) context.Context {
	return context.WithValue(ctx, debugModeContextKey{}, enabled)
}

// UseDebugMode reads debug mode flag from context.
func UseDebugMode(ctx context.Context) bool {
	enabled, ok := ctx.Value(debugModeContextKey{}).(bool)
	return ok && enabled
}

// WithArtifactMessageID binds an existing message ID to the current request context
// so artifact handlers can associate persisted artifacts with a conversation turn.
func WithArtifactMessageID(ctx context.Context, messageID uuid.UUID) context.Context {
	return context.WithValue(ctx, artifactMessageIDContextKey{}, messageID)
}

// UseArtifactMessageID returns a context-bound message ID when present.
func UseArtifactMessageID(ctx context.Context) (*uuid.UUID, bool) {
	messageID, ok := ctx.Value(artifactMessageIDContextKey{}).(uuid.UUID)
	if !ok {
		return nil, false
	}
	return &messageID, true
}
