package testenv

import (
	"context"
	"strings"
)

// Correlation identifies the physical scope (and, for jobs, the execution)
// that caused work. A request header alone does not isolate state: handlers
// must carry the correlation into the context of async work and persist the
// scope ID on records they create so cleanup and fakes can find them.
type Correlation struct {
	ScopeID     string `json:"scopeId"`
	ExecutionID string `json:"executionId,omitempty"`
}

// String returns the stable correlation key recorded on provider calls.
func (c Correlation) String() string {
	if c.ExecutionID == "" {
		return c.ScopeID
	}
	return c.ScopeID + "/" + c.ExecutionID
}

type correlationKey struct{}

// WithCorrelation binds work in ctx to a physical scope.
func WithCorrelation(ctx context.Context, c Correlation) context.Context {
	return context.WithValue(ctx, correlationKey{}, c)
}

// CorrelationFrom returns the scope correlation carried by ctx.
func CorrelationFrom(ctx context.Context) (Correlation, bool) {
	c, ok := ctx.Value(correlationKey{}).(Correlation)
	return c, ok && c.ScopeID != ""
}

// QueueNamespace derives an owned queue/stream/key namespace for a scope.
// Consumers dispose that namespace from a scope resource registered with
// Registry.OnDispose.
//
// The encoding is lossless (percent-escapes "%", ":" and "/"), so distinct
// scope IDs never share a namespace.
func QueueNamespace(prefix, scopeID string) string {
	return prefix + ":" + strings.NewReplacer("%", "%25", ":", "%3A", "/", "%2F").Replace(scopeID)
}
