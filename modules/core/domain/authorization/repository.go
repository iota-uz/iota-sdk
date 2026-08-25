// Package authorization defines persistence boundaries used by administrative
// authorization policies.
package authorization

import (
	"context"

	"github.com/google/uuid"
)

// Repository serializes authorization decisions with the writes they protect.
// Implementations must lock rows in deterministic order and scope every lock to
// the tenant in context.
type Repository interface {
	// LockTenant serializes privilege-changing decisions for one tenant. This
	// gives mixed user/group/role mutations a single global lock order before
	// their narrower row locks are acquired.
	LockTenant(ctx context.Context) error
	LockUsers(ctx context.Context, ids ...uint) error
	LockRoles(ctx context.Context, ids ...uint) error
	LockGroups(ctx context.Context, ids ...uuid.UUID) error
}
