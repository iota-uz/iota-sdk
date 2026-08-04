// Package query provides read-only organizational membership and hierarchy
// lookups (recursive-CTE subtree walks) for the core module, all tenant-scoped.
package query

import (
	"context"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

// SQL queries for organizational membership/hierarchy lookups.
//
// Subtree walks use recursive CTEs (Postgres VIEWs/TRIGGERs are intentionally
// avoided). Every query is scoped to the caller's tenant.
const (
	// userDepartmentsSQL returns the distinct departments a user holds an
	// active position in.
	userDepartmentsSQL = `
		SELECT DISTINCT p.department_id
		FROM core.user_positions p
		WHERE p.user_id = $1
		  AND p.tenant_id = $2
		  AND p.status = 'active'`

	// userManagedDepartmentsSQL returns the departments where the user holds
	// an active manager position (no subtree expansion).
	userManagedDepartmentsSQL = `
		SELECT DISTINCT p.department_id
		FROM core.user_positions p
		WHERE p.user_id = $1
		  AND p.tenant_id = $2
		  AND p.is_manager = TRUE
		  AND p.status = 'active'`

	// userManagedDepartmentsSubtreeSQL expands the manager's departments to
	// include every descendant department in the tenant.
	userManagedDepartmentsSubtreeSQL = `
		WITH RECURSIVE roots AS (
			SELECT DISTINCT p.department_id AS id
			FROM core.user_positions p
			WHERE p.user_id = $1
			  AND p.tenant_id = $2
			  AND p.is_manager = TRUE
			  AND p.status = 'active'
		),
		subtree AS (
			SELECT r.id
			FROM roots r
			UNION
			SELECT d.id
			FROM core.departments d
			JOIN subtree s ON d.parent_id = s.id
			WHERE d.tenant_id = $2
		)
		SELECT DISTINCT id FROM subtree`

	// departmentSubtreeSQL returns the department itself plus all of its
	// descendants within the tenant.
	departmentSubtreeSQL = `
		WITH RECURSIVE subtree AS (
			SELECT d.id
			FROM core.departments d
			WHERE d.id = $1 AND d.tenant_id = $2
			UNION
			SELECT child.id
			FROM core.departments child
			JOIN subtree s ON child.parent_id = s.id
			WHERE child.tenant_id = $2
		)
		SELECT id FROM subtree`
)

// OrgQueryRepository exposes read-only organizational membership and hierarchy
// lookups backed by recursive CTEs. All methods are tenant-scoped.
type OrgQueryRepository interface {
	// UserDepartments returns the departments the user holds a position in.
	UserDepartments(ctx context.Context, userID uint) ([]uuid.UUID, error)
	// UserManagedDepartments returns the departments where the user holds a
	// manager position. When includeSubtree is true the result also contains
	// every descendant department.
	UserManagedDepartments(ctx context.Context, userID uint, includeSubtree bool) ([]uuid.UUID, error)
	// DepartmentSubtree returns the department and all of its descendants.
	DepartmentSubtree(ctx context.Context, deptID uuid.UUID) ([]uuid.UUID, error)
}

type PgOrgQueryRepository struct{}

func NewPgOrgQueryRepository() OrgQueryRepository {
	return &PgOrgQueryRepository{}
}

func (r *PgOrgQueryRepository) UserDepartments(ctx context.Context, userID uint) ([]uuid.UUID, error) {
	const op serrors.Op = "PgOrgQueryRepository.UserDepartments"
	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return nil, serrors.E(op, err)
	}
	return r.queryDepartmentIDs(ctx, userDepartmentsSQL, userID, tenantID.String())
}

func (r *PgOrgQueryRepository) UserManagedDepartments(
	ctx context.Context,
	userID uint,
	includeSubtree bool,
) ([]uuid.UUID, error) {
	const op serrors.Op = "PgOrgQueryRepository.UserManagedDepartments"
	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return nil, serrors.E(op, err)
	}

	query := userManagedDepartmentsSQL
	if includeSubtree {
		query = userManagedDepartmentsSubtreeSQL
	}
	return r.queryDepartmentIDs(ctx, query, userID, tenantID.String())
}

func (r *PgOrgQueryRepository) DepartmentSubtree(ctx context.Context, deptID uuid.UUID) ([]uuid.UUID, error) {
	const op serrors.Op = "PgOrgQueryRepository.DepartmentSubtree"
	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return nil, serrors.E(op, err)
	}
	return r.queryDepartmentIDs(ctx, departmentSubtreeSQL, deptID.String(), tenantID.String())
}

func (r *PgOrgQueryRepository) queryDepartmentIDs(
	ctx context.Context,
	query string,
	args ...interface{},
) ([]uuid.UUID, error) {
	const op serrors.Op = "PgOrgQueryRepository.queryDepartmentIDs"
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return nil, serrors.E(op, err)
	}

	rows, err := tx.Query(ctx, query, args...)
	if err != nil {
		return nil, serrors.E(op, err)
	}
	defer rows.Close()

	ids := make([]uuid.UUID, 0)
	for rows.Next() {
		var idStr string
		if err := rows.Scan(&idStr); err != nil {
			return nil, serrors.E(op, err)
		}
		id, err := uuid.Parse(idStr)
		if err != nil {
			return nil, serrors.E(op, err)
		}
		ids = append(ids, id)
	}

	if err := rows.Err(); err != nil {
		return nil, serrors.E(op, err)
	}

	return ids, nil
}
