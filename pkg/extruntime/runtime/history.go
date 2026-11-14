package runtime

import (
	"context"
	"strconv"

	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/logging/domain/aggregates/businessevent"
	"github.com/iota-uz/iota-sdk/modules/logging/history"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/host"
)

// HistoryExecution is host.Config.NativeContext of the runtime: it gives the
// Go code of a native contract the trusted business-history execution of the
// invocation, whatever started it.
//
// A root that came from a request of the acting user (the CRM forms, the
// MCP tools) already carries its execution and keeps it. Every other root (a
// subscription delivery, a schedule slot, a workflow step, an endpoint, a
// request acting as someone else) has none, and the history writer would
// refuse every business action ("no execution context"). For those the
// execution is derived from the invocation: the runtime is the service actor
// and the subject (the initiating user, or the trigger's owner) the
// initiator, so the record says who the work was done for; the channel
// follows the trigger kind; the operation is this invocation and the run the
// root it belongs to.
func HistoryExecution(users user.Repository) func(ctx context.Context, inv host.NativeInvocation) context.Context {
	return func(ctx context.Context, inv host.NativeInvocation) context.Context {
		current, hasExec := history.UseExecution(ctx)
		if u, err := composables.UseUser(ctx); err == nil && u != nil && int64(u.ID()) == inv.Subject.UserID {
			if hasExec {
				return ctx
			}
			// The subject is the request's user: it acts, as in any request.
			return history.WithExecution(ctx, history.Execution{
				Channel: historyChannel(inv.Trigger.Kind), OperationID: inv.InvocationID, RunID: inv.RootID,
			})
		}
		kind := string(inv.Trigger.Kind)
		if kind == "" {
			kind = string(host.TriggerUI)
		}
		display := "Extension runtime: " + kind
		if inv.Trigger.Ref != "" {
			display += " " + inv.Trigger.Ref
		}
		actor := businessevent.ServiceActor("extruntime:"+kind, display)
		initiator := initiatorOf(ctx, users, inv)
		return history.WithExecution(ctx, history.Execution{
			Channel:     historyChannel(inv.Trigger.Kind),
			OperationID: inv.InvocationID,
			Actor:       &actor,
			Initiator:   initiator,
			RequestID:   current.RequestID,
			RunID:       inv.RootID,
		})
	}
}

func historyChannel(k host.TriggerKind) businessevent.Channel {
	switch k {
	case host.TriggerMCP:
		return businessevent.ChannelMCP
	case host.TriggerEndpoint:
		return businessevent.ChannelAPI
	case host.TriggerSchedule, host.TriggerSubscription, host.TriggerWorkflow:
		return businessevent.ChannelJob
	case host.TriggerUI:
		return businessevent.ChannelUI
	}
	return businessevent.ChannelUI
}

// initiatorOf is the snapshot of the invocation's subject: its names when the
// user can be read, its ID otherwise (the ID alone is a valid user snapshot).
func initiatorOf(ctx context.Context, users user.Repository, inv host.NativeInvocation) *businessevent.ActorSnapshot {
	if inv.Subject.UserID <= 0 {
		return nil
	}
	if users != nil {
		// A fresh context: the user is read outside the invocation's
		// transaction, and only the tenant and the pool matter to the read.
		rctx := composables.WithTenantID(context.WithoutCancel(ctx), inv.Subject.TenantID)
		if pool, err := composables.UsePool(ctx); err == nil {
			rctx = composables.WithPool(rctx, pool)
		}
		if u, err := users.GetByID(rctx, uint(inv.Subject.UserID)); err == nil && u != nil {
			snapshot := businessevent.UserActor(u)
			return &snapshot
		}
	}
	return &businessevent.ActorSnapshot{
		SchemaVersion: businessevent.ActorSnapshotVersion,
		Kind:          businessevent.ActorUser,
		UserID:        strconv.FormatInt(inv.Subject.UserID, 10),
	}
}
