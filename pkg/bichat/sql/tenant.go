package sql

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// setConfigTenantSQL sets app.tenant_id at transaction scope.
// The third argument true means "local": reset on tx commit/rollback.
const setConfigTenantSQL = `SELECT set_config('app.tenant_id', $1, true)`

// SetTenantContext binds tenantID to the current transaction via
// set_config(..., true) so tenant-aware views and RLS policies can resolve
// it via current_setting('app.tenant_id', true)::uuid.
//
// tx must be a live transaction; the setting does not outlive it. Pass
// uuid.Nil to skip the call (useful when the caller has no tenant context,
// e.g. a system-level query).
func SetTenantContext(ctx context.Context, tx pgx.Tx, tenantID uuid.UUID) error {
	if tx == nil {
		return fmt.Errorf("sql.SetTenantContext: tx is nil")
	}
	if tenantID == uuid.Nil {
		return nil
	}
	if _, err := tx.Exec(ctx, setConfigTenantSQL, tenantID.String()); err != nil {
		return fmt.Errorf("sql.SetTenantContext: %w", err)
	}
	return nil
}

// TenantResolver derives a tenant UUID from a request context. Consumers
// typically wire composables.UseTenantID here. Returning uuid.Nil signals
// "no tenant context" — SetTenantContext becomes a no-op.
type TenantResolver func(ctx context.Context) (uuid.UUID, error)

// NoTenantResolver is the default resolver: always returns uuid.Nil.
// SafeQueryExecutor uses this when WithTenantResolver is not supplied.
func NoTenantResolver(context.Context) (uuid.UUID, error) { return uuid.Nil, nil }
