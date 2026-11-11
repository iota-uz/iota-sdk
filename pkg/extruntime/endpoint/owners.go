package endpoint

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/iota-uz/iota-sdk/pkg/extruntime/triggerowner"
)

// PgOwners reads extruntime.trigger_owners, the table every trigger kind
// (subscriptions, schedules, public endpoints) shares, through the one lookup
// all of them use (package triggerowner).
type PgOwners struct {
	DB *pgxpool.Pool
}

var _ Owners = (*PgOwners)(nil)

func (o *PgOwners) Owner(ctx context.Context, triggerID string, tenant uuid.UUID) (int64, bool, error) {
	return triggerowner.Lookup(ctx, o.DB, triggerID, tenant)
}
