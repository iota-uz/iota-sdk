package endpoint

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

const opOwner = serrors.Op("extruntime.endpoint.PgOwners.Owner")

// PgOwners reads extruntime.trigger_owners, the table every trigger kind
// (subscriptions, schedules, public endpoints) shares.
type PgOwners struct {
	DB *pgxpool.Pool
}

var _ Owners = (*PgOwners)(nil)

func (o *PgOwners) Owner(ctx context.Context, triggerID string, tenant uuid.UUID) (int64, bool, error) {
	var userID int64
	err := o.DB.QueryRow(ctx, `SELECT user_id FROM extruntime.trigger_owners WHERE trigger_id = $1 AND tenant_id = $2`, triggerID, tenant).Scan(&userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, serrors.FromDB(opOwner, err)
	}
	return userID, true, nil
}
