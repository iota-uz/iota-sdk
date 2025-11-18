package query

import (
	"context"

	"github.com/iota-uz/iota-sdk/modules/core/infrastructure/persistence/models"
	"github.com/iota-uz/iota-sdk/modules/core/presentation/viewmodels"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/pkg/errors"
)

const (
	selectRolesWithCountsSQL = `
		SELECT
			r.id,
			r.type,
			r.name,
			r.description,
			r.created_at,
			r.updated_at,
			COALESCE(COUNT(DISTINCT ur.user_id), 0) as users_count
		FROM roles r
		LEFT JOIN user_roles ur ON r.id = ur.role_id
		WHERE r.tenant_id = $1
		GROUP BY r.id, r.type, r.name, r.description, r.created_at, r.updated_at
		ORDER BY r.name ASC
	`
)

type RoleQueryRepository interface {
	FindRolesWithCounts(ctx context.Context) ([]*viewmodels.Role, error)
}

type pgRoleQueryRepository struct{}

func NewPgRoleQueryRepository() RoleQueryRepository {
	return &pgRoleQueryRepository{}
}

func (r *pgRoleQueryRepository) FindRolesWithCounts(ctx context.Context) ([]*viewmodels.Role, error) {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get transaction")
	}

	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get tenant ID")
	}

	rows, err := tx.Query(ctx, selectRolesWithCountsSQL, tenantID)
	if err != nil {
		return nil, errors.Wrap(err, "failed to query roles with counts")
	}
	defer rows.Close()

	roles := make([]*viewmodels.Role, 0)
	for rows.Next() {
		var dbRole models.Role
		var usersCount int

		err := rows.Scan(
			&dbRole.ID,
			&dbRole.Type,
			&dbRole.Name,
			&dbRole.Description,
			&dbRole.CreatedAt,
			&dbRole.UpdatedAt,
			&usersCount,
		)
		if err != nil {
			return nil, errors.Wrap(err, "failed to scan role")
		}

		role := mapToRoleViewModel(dbRole)
		role.UsersCount = usersCount

		// Set permissions based on role type
		role.CanUpdate = dbRole.Type != "system"
		role.CanDelete = dbRole.Type != "system"

		roles = append(roles, role)
	}

	if err := rows.Err(); err != nil {
		return nil, errors.Wrap(err, "error iterating role rows")
	}

	return roles, nil
}
