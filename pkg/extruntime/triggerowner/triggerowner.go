// Package triggerowner reads extruntime.trigger_owners, the one table that
// says which user a trigger without a user (a schedule, a subscription on a
// user-less event, a public endpoint, a delivery policy) acts as in a tenant.
// Every slice that needs the owner (events, endpoints, notifications, the
// management service) reads it here, so there is one query and one error
// mapping.
package triggerowner

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

const opLookup = serrors.Op("extruntime.triggerowner.Lookup")

// Querier is a pool or a transaction.
type Querier interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// Lookup returns the owner-user of the trigger in the tenant; found is false
// when none is assigned.
func Lookup(ctx context.Context, q Querier, triggerID string, tenant uuid.UUID) (userID int64, found bool, err error) {
	err = q.QueryRow(ctx, `SELECT user_id FROM extruntime.trigger_owners WHERE trigger_id = $1 AND tenant_id = $2`, triggerID, tenant).Scan(&userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, serrors.FromDB(opLookup, err)
	}
	return userID, true, nil
}
