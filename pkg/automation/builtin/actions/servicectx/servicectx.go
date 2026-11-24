// Package servicectx builds the context an SDK action calls the module services with: the actor's
// tenant, the database pool, the actor as the request's user (services check rights with
// composables.CanUser) and a business-history execution that says an automation did it for the actor.
package servicectx

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/logging/domain/aggregates/businessevent"
	"github.com/iota-uz/iota-sdk/modules/logging/history"
	"github.com/iota-uz/iota-sdk/pkg/automation/builtin/actions"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/spi"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

// Context is ctx for the services an action of actor calls. An actor that is not a user of the tenant,
// or a blocked one, is actions.ErrForbidden.
func Context(ctx context.Context, pool *pgxpool.Pool, users user.Repository, actor spi.Subject) (context.Context, error) {
	if actor.UserID <= 0 {
		return nil, actions.ErrForbidden
	}
	ctx = composables.WithTenantID(ctx, actor.TenantID)
	if pool != nil {
		ctx = composables.WithPool(ctx, pool)
	}
	u, err := users.GetByID(ctx, uint(actor.UserID))
	switch {
	case serrors.CodeOf(err) == serrors.NotFound:
		return nil, actions.ErrForbidden
	case err != nil:
		return nil, err
	case u == nil || u.IsBlocked():
		return nil, actions.ErrForbidden
	}
	ctx = composables.WithUser(ctx, u)
	if _, ok := history.UseExecution(ctx); ok {
		return ctx, nil // the command host's: this invocation for the subject
	}
	op := actions.InvocationID(ctx)
	if op == "" {
		op = uuid.NewString()
	}
	service := businessevent.ServiceActor("automation", "Automation")
	initiator := businessevent.UserActor(u)
	return history.WithExecution(ctx, history.Execution{
		Channel: businessevent.ChannelJob, OperationID: op, Actor: &service, Initiator: &initiator,
	}), nil
}

// Refusal maps a service's refusal of the actor to actions.ErrForbidden; other errors stay.
func Refusal(err error) error {
	if errors.Is(err, composables.ErrForbidden) || serrors.CodeOf(err) == serrors.PermissionDenied {
		return actions.ErrForbidden
	}
	return err
}
