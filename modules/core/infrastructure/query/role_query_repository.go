// Package query provides this package.
package query

import (
	"context"
	"strconv"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/permission"
	"github.com/iota-uz/iota-sdk/modules/core/infrastructure/persistence/models"
	"github.com/iota-uz/iota-sdk/modules/core/presentation/viewmodels"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
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
	FindAssignmentOptions(ctx context.Context) ([]*viewmodels.AssignmentOption, error)
}

func (r *pgRoleQueryRepository) FindAssignmentOptions(ctx context.Context) ([]*viewmodels.AssignmentOption, error) {
	const op = serrors.Op("RoleQueryRepository.FindAssignmentOptions")
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return nil, serrors.E(op, err)
	}
	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return nil, serrors.E(op, err)
	}

	rows, err := tx.Query(ctx, `
		SELECT r.id, r.type, r.name, COALESCE(r.description, '')
		FROM roles r
		WHERE r.tenant_id = $1
		ORDER BY LOWER(r.name), r.id`, tenantID)
	if err != nil {
		return nil, serrors.E(op, err)
	}
	defer rows.Close()

	options := make([]*viewmodels.AssignmentOption, 0)
	byID := make(map[uint]*viewmodels.AssignmentOption)
	ids := make([]uint, 0)
	for rows.Next() {
		var id uint
		option := &viewmodels.AssignmentOption{Permissions: []permission.Permission{}}
		if err := rows.Scan(&id, &option.Type, &option.Name, &option.Description); err != nil {
			return nil, serrors.E(op, err)
		}
		option.ID = strconv.FormatUint(uint64(id), 10)
		options = append(options, option)
		byID[id] = option
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, serrors.E(op, err)
	}
	if len(ids) == 0 {
		return options, nil
	}

	permissionRows, err := tx.Query(ctx, `
		SELECT rp.role_id, p.id, p.name, p.resource, p.action, p.modifier
		FROM role_permissions rp
		JOIN roles r ON r.id = rp.role_id
		JOIN permissions p ON p.id = rp.permission_id
		WHERE rp.role_id = ANY($1) AND r.tenant_id = $2
		ORDER BY rp.role_id, p.name, p.id`, ids, tenantID)
	if err != nil {
		return nil, serrors.E(op, err)
	}
	defer permissionRows.Close()
	for permissionRows.Next() {
		var roleID uint
		var id uuid.UUID
		var name, resource, action, modifier string
		if err := permissionRows.Scan(&roleID, &id, &name, &resource, &action, &modifier); err != nil {
			return nil, serrors.E(op, err)
		}
		if option := byID[roleID]; option != nil {
			option.Permissions = append(option.Permissions, permission.New(
				permission.WithID(id), permission.WithName(name), permission.WithResource(permission.Resource(resource)),
				permission.WithAction(permission.Action(action)), permission.WithModifier(permission.Modifier(modifier)),
			))
		}
	}
	if err := permissionRows.Err(); err != nil {
		return nil, serrors.E(op, err)
	}
	return options, nil
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
