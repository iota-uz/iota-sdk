package persistence

import (
	"context"
	"fmt"

	"github.com/go-faster/errors"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/role"
	"github.com/iota-uz/iota-sdk/modules/core/infrastructure/persistence/models"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/repo"
)

var (
	ErrRoleNotFound = errors.New("role not found")
)

const (
	roleFindQuery = `
		SELECT
			r.id,
			r.name,
			r.description,
			r.created_at,
			r.updated_at
		FROM roles r`
	rolePermissionsQuery = `
		SELECT
			p.id,
			p.name,
			p.resource,
			p.action,
			p.modifier,
			p.description,
			rp.role_id
		FROM permissions p LEFT JOIN role_permissions rp ON rp.permission_id = p.id WHERE rp.role_id = ANY($1)`
	roleCountQuery             = `SELECT COUNT(DISTINCT roles.id) FROM roles`
	roleInsertQuery            = `INSERT INTO roles (name, description) VALUES ($1, $2) RETURNING id`
	roleUpdateQuery            = `UPDATE roles SET name = $1, description = $2, updated_at = $3	WHERE id = $4`
	roleDeletePermissionsQuery = `DELETE FROM role_permissions WHERE role_id = $1`
	roleInsertPermissionQuery  = `
		INSERT INTO role_permissions (role_id, permission_id)
		VALUES ($1, $2)
		ON CONFLICT (role_id, permission_id) DO NOTHING`
	roleDeleteQuery = `DELETE FROM roles WHERE id = $1`
)

type GormRoleRepository struct {
	fieldMap map[role.Field]string
}

func NewRoleRepository() role.Repository {
	return &GormRoleRepository{
		fieldMap: map[role.Field]string{
			role.Name:         "r.name",
			role.Description:  "r.description",
			role.CreatedAt:    "r.created_at",
			role.PermissionID: "rp.permission_id",
		},
	}
}

func (g *GormRoleRepository) buildRoleFilters(params *role.FindParams) ([]string, []interface{}, error) {
	where := []string{"1 = 1"}
	args := []interface{}{}

	for _, filter := range params.Filters {
		column, ok := g.fieldMap[filter.Column]
		if !ok {
			return nil, nil, errors.Wrap(fmt.Errorf("unknown filter field: %v", filter.Column), "invalid filter")
		}

		where = append(where, filter.Filter.String(column, len(args)+1))
		args = append(args, filter.Filter.Value())
	}

	if params.Search != "" {
		index := len(args) + 1
		where = append(
			where,
			fmt.Sprintf(
				"(r.name ILIKE $%d OR r.description ILIKE $%d)",
				index,
				index,
			),
		)
		args = append(args, "%"+params.Search+"%")
	}

	return where, args, nil
}

func (g *GormRoleRepository) GetPaginated(ctx context.Context, params *role.FindParams) ([]role.Role, error) {
	sortFields := []string{}
	for _, f := range params.SortBy.Fields {
		if field, ok := g.fieldMap[f]; ok {
			sortFields = append(sortFields, field)
		} else {
			return nil, fmt.Errorf("unknown sort field: %v", f)
		}
	}

	where, args, err := g.buildRoleFilters(params)
	if err != nil {
		return nil, err
	}

	baseQuery := roleFindQuery
	// Add necessary joins based on filters
	for _, f := range params.Filters {
		if f.Column == role.PermissionID {
			baseQuery += " JOIN role_permissions rp ON r.id = rp.role_id"
			break // Only add the join once
		}
	}

	query := repo.Join(
		baseQuery,
		repo.JoinWhere(where...),
		repo.OrderBy(sortFields, params.SortBy.Ascending),
		repo.FormatLimitOffset(params.Limit, params.Offset),
	)
	return g.queryRoles(ctx, query, args...)
}

func (g *GormRoleRepository) Count(ctx context.Context, params *role.FindParams) (int64, error) {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return 0, errors.Wrap(err, "failed to get transaction")
	}

	if params == nil {
		var count int64
		if err := tx.QueryRow(ctx, roleCountQuery).Scan(&count); err != nil {
			return 0, errors.Wrap(err, "failed to count roles")
		}
		return count, nil
	}

	where, args, err := g.buildRoleFilters(params)
	if err != nil {
		return 0, err
	}

	baseQuery := roleCountQuery

	// Add necessary joins based on filters
	for _, f := range params.Filters {
		if f.Column == role.PermissionID {
			baseQuery += " JOIN role_permissions rp ON r.id = rp.role_id"
			break // Only add the join once
		}
	}

	query := repo.Join(
		baseQuery,
		repo.JoinWhere(where...),
	)

	var count int64
	err = tx.QueryRow(ctx, query, args...).Scan(&count)
	if err != nil {
		return 0, errors.Wrap(err, "failed to count roles")
	}
	return count, nil
}

func (g *GormRoleRepository) GetAll(ctx context.Context) ([]role.Role, error) {
	return g.queryRoles(ctx, roleFindQuery)
}

func (g *GormRoleRepository) GetByID(ctx context.Context, id uint) (role.Role, error) {
	query := roleFindQuery + " WHERE r.id = $1"
	roles, err := g.queryRoles(ctx, query, id)
	if err != nil {
		return nil, err
	}
	if len(roles) == 0 {
		return nil, ErrRoleNotFound
	}
	return roles[0], nil
}

func (g *GormRoleRepository) Create(ctx context.Context, data role.Role) (role.Role, error) {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return nil, err
	}

	entity, permissions := toDBRole(data)
	var id uint
	if err := tx.QueryRow(
		ctx,
		roleInsertQuery,
		entity.Name,
		entity.Description,
	).Scan(&id); err != nil {
		return nil, err
	}

	for _, permission := range permissions {
		if err := g.execQuery(ctx, roleInsertPermissionQuery,
			id,
			permission.ID,
		); err != nil {
			return nil, err
		}
	}
	return g.GetByID(ctx, id)
}

func (g *GormRoleRepository) Update(ctx context.Context, data role.Role) (role.Role, error) {
	dbRole, dbPermissions := toDBRole(data)

	if err := g.execQuery(
		ctx,
		roleUpdateQuery,
		dbRole.Name,
		dbRole.Description,
		dbRole.UpdatedAt,
		dbRole.ID,
	); err != nil {
		return nil, err
	}

	if err := g.execQuery(ctx, roleDeletePermissionsQuery, dbRole.ID); err != nil {
		return nil, err
	}

	for _, permission := range dbPermissions {
		if err := g.execQuery(ctx, roleInsertPermissionQuery,
			dbRole.ID,
			permission.ID,
		); err != nil {
			return nil, err
		}
	}
	return g.GetByID(ctx, dbRole.ID)
}

func (g *GormRoleRepository) Delete(ctx context.Context, id uint) error {
	if err := g.execQuery(ctx, roleDeletePermissionsQuery, id); err != nil {
		return err
	}
	return g.execQuery(ctx, roleDeleteQuery, id)
}

func (g *GormRoleRepository) queryPermissions(ctx context.Context, roleIDs []uint) (map[uint][]*models.Permission, error) {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return nil, err
	}

	rows, err := tx.Query(ctx, rolePermissionsQuery, roleIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[uint][]*models.Permission)
	for rows.Next() {
		var roleID uint
		var p models.Permission
		if err := rows.Scan(
			&p.ID,
			&p.Name,
			&p.Resource,
			&p.Action,
			&p.Modifier,
			&p.Description,
			&roleID,
		); err != nil {
			return nil, err
		}
		result[roleID] = append(result[roleID], &p)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

func (g *GormRoleRepository) queryRoles(ctx context.Context, query string, args ...interface{}) ([]role.Role, error) {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return nil, err
	}

	rows, err := tx.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var dbRoles []*models.Role

	for rows.Next() {
		var r models.Role

		if err := rows.Scan(
			&r.ID,
			&r.Name,
			&r.Description,
			&r.CreatedAt,
			&r.UpdatedAt,
		); err != nil {
			return nil, err
		}
		dbRoles = append(dbRoles, &r)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	ids := make([]uint, 0, len(dbRoles))
	for _, dbRole := range dbRoles {
		ids = append(ids, dbRole.ID)
	}
	permissionsMap, err := g.queryPermissions(ctx, ids)
	if err != nil {
		return nil, err
	}
	roles := make([]role.Role, 0, len(dbRoles))
	for _, dbRole := range dbRoles {
		entity, err := toDomainRole(dbRole, permissionsMap[dbRole.ID])
		if err != nil {
			return nil, err
		}
		roles = append(roles, entity)
	}
	return roles, nil
}

func (g *GormRoleRepository) execQuery(ctx context.Context, query string, args ...interface{}) error {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, query, args...)
	return err
}
