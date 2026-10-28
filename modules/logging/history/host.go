package history

import (
	"context"
	"net/http"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/iota-uz/iota-sdk/modules/logging/domain/aggregates/businessevent"
	"github.com/iota-uz/iota-sdk/pkg/composables"
)

// SystemExecution runs ctx as one operation of a system executor, such as a CLI command, seed,
// or scheduled task. id is a stable machine name and displayName is what readers see.
func SystemExecution(ctx context.Context, id, displayName string) context.Context {
	actor := businessevent.SystemActor(id, displayName)
	return WithExecution(ctx, Execution{
		Channel:     businessevent.ChannelJob,
		OperationID: uuid.NewString(),
		Actor:       &actor,
	})
}

// JobExecution runs ctx as one run of a background job executed on behalf of initiator, if known.
func JobExecution(ctx context.Context, runID, kind string, initiator *businessevent.ActorSnapshot) context.Context {
	actor := businessevent.ServiceActor("job:"+kind, kind)
	return WithExecution(ctx, Execution{
		Channel:     businessevent.ChannelJob,
		OperationID: runID,
		Actor:       &actor,
		Initiator:   initiator,
		RunID:       runID,
	})
}

// ImportExecution runs ctx as one import batch: every object it touches shares the operation,
// and the actor of the surrounding execution (the importing user by default) stays the actor.
func ImportExecution(ctx context.Context, runID string) context.Context {
	exec, _ := UseExecution(ctx)
	return WithExecution(ctx, Execution{
		Channel:     businessevent.ChannelImport,
		OperationID: uuid.NewString(),
		Actor:       exec.Actor,
		Initiator:   exec.Initiator,
		RequestID:   exec.RequestID,
		RunID:       runID,
	})
}

// UserInitiator is the snapshot of the authenticated user in ctx, or nil when there is none.
func UserInitiator(ctx context.Context) *businessevent.ActorSnapshot {
	u, err := composables.UseUser(ctx)
	if err != nil || u == nil {
		return nil
	}
	snapshot := businessevent.UserActor(u)
	return &snapshot
}

// Middleware makes every HTTP request one UI operation unless an outer layer already set the
// execution. The operation ID is generated here and never taken from the request.
func Middleware() mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if _, ok := UseExecution(r.Context()); ok {
				next.ServeHTTP(w, r)
				return
			}
			ctx := WithExecution(r.Context(), Execution{
				Channel:     businessevent.ChannelUI,
				OperationID: uuid.NewString(),
				RequestID:   w.Header().Get("X-Request-Id"),
			})
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
