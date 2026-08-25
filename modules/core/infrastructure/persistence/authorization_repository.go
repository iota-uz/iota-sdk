package persistence

import (
	"context"
	"sort"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/core/domain/authorization"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

type privilegeRepository struct{}

func NewPrivilegeRepository() authorization.Repository {
	return &privilegeRepository{}
}

func (r *privilegeRepository) LockTenant(ctx context.Context) error {
	const op serrors.Op = "PrivilegeRepository.LockTenant"
	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return serrors.E(op, err)
	}
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return serrors.E(op, err)
	}
	// A transaction-scoped advisory lock prevents cross-type deadlocks and
	// closes TOCTOU windows when one mutation touches users, groups, and roles.
	if _, err := tx.Exec(ctx, "SELECT pg_advisory_xact_lock(hashtextextended($1::text, 1033))", tenantID.String()); err != nil {
		return serrors.E(op, err)
	}
	return nil
}

func (r *privilegeRepository) LockUsers(ctx context.Context, ids ...uint) error {
	const op serrors.Op = "PrivilegeRepository.LockUsers"
	if len(ids) == 0 {
		return nil
	}
	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return serrors.E(op, err)
	}
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return serrors.E(op, err)
	}
	values := uniqueSortedUserIDs(ids)
	rows, err := tx.Query(ctx, `
		SELECT id FROM users
		WHERE tenant_id = $1 AND id = ANY($2::int[])
		ORDER BY id FOR UPDATE`, tenantID, values)
	if err != nil {
		return serrors.E(op, err)
	}
	rows.Close()
	return nil
}

func (r *privilegeRepository) LockRoles(ctx context.Context, ids ...uint) error {
	const op serrors.Op = "PrivilegeRepository.LockRoles"
	if len(ids) == 0 {
		return nil
	}
	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return serrors.E(op, err)
	}
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return serrors.E(op, err)
	}
	values := uniqueSortedUserIDs(ids)
	rows, err := tx.Query(ctx, `
		SELECT id FROM roles
		WHERE tenant_id = $1 AND id = ANY($2::int[])
		ORDER BY id FOR UPDATE`, tenantID, values)
	if err != nil {
		return serrors.E(op, err)
	}
	rows.Close()
	return nil
}

func (r *privilegeRepository) LockGroups(ctx context.Context, ids ...uuid.UUID) error {
	const op serrors.Op = "PrivilegeRepository.LockGroups"
	if len(ids) == 0 {
		return nil
	}
	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return serrors.E(op, err)
	}
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return serrors.E(op, err)
	}
	values := uniqueSortedGroupIDs(ids)
	rows, err := tx.Query(ctx, `
		SELECT id FROM user_groups
		WHERE tenant_id = $1 AND id = ANY($2::uuid[])
		ORDER BY id FOR UPDATE`, tenantID, values)
	if err != nil {
		return serrors.E(op, err)
	}
	rows.Close()
	return nil
}

func uniqueSortedUserIDs(ids []uint) []int32 {
	seen := make(map[uint]struct{}, len(ids))
	values := make([]int, 0, len(ids))
	for _, id := range ids {
		if id == 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		values = append(values, int(id))
	}
	sort.Ints(values)
	result := make([]int32, len(values))
	for i, value := range values {
		result[i] = int32(value)
	}
	return result
}

func uniqueSortedGroupIDs(ids []uuid.UUID) []uuid.UUID {
	seen := make(map[uuid.UUID]struct{}, len(ids))
	values := make([]uuid.UUID, 0, len(ids))
	for _, id := range ids {
		if id == uuid.Nil {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		values = append(values, id)
	}
	sort.Slice(values, func(i, j int) bool { return values[i].String() < values[j].String() })
	return values
}
