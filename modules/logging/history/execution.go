package history

import (
	"context"

	"github.com/iota-uz/iota-sdk/modules/logging/domain/aggregates/businessevent"
)

type executionKey struct{}

// Execution is the trusted context of a business run, set by the host (HTTP middleware,
// MCP or chat executor, import, job runner) and never taken from request or model input.
type Execution struct {
	Channel     businessevent.Channel
	OperationID string
	// Actor is the service or system executor; nil means the authenticated user is the actor.
	Actor     *businessevent.ActorSnapshot
	Initiator *businessevent.ActorSnapshot
	RequestID string
	RunID     string
	ClientID  string
}

func WithExecution(ctx context.Context, e Execution) context.Context {
	return context.WithValue(ctx, executionKey{}, e)
}

func UseExecution(ctx context.Context) (Execution, bool) {
	e, ok := ctx.Value(executionKey{}).(Execution)
	return e, ok
}
