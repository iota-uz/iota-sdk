package services

import "context"

type debugModeContextKey struct{}

// WithDebugMode marks request context for debug-oriented model behavior.
func WithDebugMode(ctx context.Context, enabled bool) context.Context {
	return context.WithValue(ctx, debugModeContextKey{}, enabled)
}

// UseDebugMode reads debug mode flag from context.
func UseDebugMode(ctx context.Context) bool {
	enabled, ok := ctx.Value(debugModeContextKey{}).(bool)
	return ok && enabled
}
