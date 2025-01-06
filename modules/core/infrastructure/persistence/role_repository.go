package persistence

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/go-faster/errors"
	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/role"
	"github.com/iota-uz/iota-sdk/modules/core/infrastructure/persistence/models"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"strings"
)

var (
	ErrRoleNotFound = errors.New("role not found")
)

const (
	roleFindQuery = `
		SELECT 
			roles.id, 
			roles.name, 
			roles.description, 
			roles.created_at, 
			roles.updated_at,
			p.id,
			p.name,
			p.resource,
			p.action,
			p.modifier,
			p.description
		FROM roles 
		LEFT JOIN role_permissions rp ON rp.role_id = roles.id
		LEFT JOIN permissions p ON p.id = rp.permission_id`
	roleCountQuery = `
		SELECT COUNT(DISTINCT roles.id) FROM roles`
	roleInsertQuery = `
		INSERT INTO roles (name, description)
		VALUES ($1, $2)
		RETURNING id`
	roleUpdateQuery = `
		UPDATE roles
		SET name = $1, description = $2
		WHERE id = $3`
	roleDeletePermissionsQuery = `DELETE FROM role_permissions WHERE role_id = $1`
	roleInsertPermissionQuery  = `
		INSERT INTO role_permissions (role_id, permission_id)
		VALUES ($1, $2) 
		ON CONFLICT (role_id, permission_id) DO NOTHING`
	roleDeleteQuery = `DELETE FROM roles WHERE id = $1`
)

type GormRoleRepository struct{}

func NewRoleRepository() role.Repository {
	return &GormRoleRepository{}
}

func (g *GormRoleRepository) GetPaginated(ctx context.Context, params *role.FindParams) ([]role.Role, error) {
	where, args := []string{"1 = 1"}, []interface{}{}
	joins := []string{}

	if params.ID != 0 {
		where = append(where, fmt.Sprintf("roles.id = $%d", len(args)+1))
		args = append(args, params.ID)
	}

	if params.UserID != 0 {
		joins = append(joins, fmt.Sprintf(
			"INNER JOIN user_roles ur ON ur.role_id = roles.id AND ur.user_id = $%d",
			len(args)+1,
		))
		args = append(args, params.UserID)
	}

	query := roleFindQuery + "\n" +
		strings.Join(joins, "\n") + "\n" +
		"WHERE " + strings.Join(where, " AND ")

	if params.Limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", params.Limit)
	}
	if params.Offset > 0 {
		query += fmt.Sprintf(" OFFSET %d", params.Offset)
	}

	return g.queryRoles(ctx, query, args...)
}

func (g *GormRoleRepository) Count(ctx context.Context) (int64, error) {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return 0, err
	}
	var count int64
	if err := tx.QueryRow(ctx, roleCountQuery).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

func (g *GormRoleRepository) GetAll(ctx context.Context) ([]role.Role, error) {
	return g.queryRoles(ctx, roleFindQuery)
}

func (g *GormRoleRepository) GetByID(ctx context.Context, id uint) (role.Role, error) {
	query := roleFindQuery + " WHERE roles.id = $1"
	roles, err := g.queryRoles(ctx, query, id)
	if err != nil {
		return nil, err
	}
	if len(roles) == 0 {
		return nil, ErrRoleNotFound
	}
	return roles[0], nil
}

func (g *GormRoleRepository) CreateOrUpdate(ctx context.Context, data role.Role) error {
	r, err := g.GetByID(ctx, data.ID())
	if err != nil && !errors.Is(err, ErrRoleNotFound) {
		return err
	}
	if r.ID() != 0 {
		return g.Update(ctx, data)
	}
	return g.Create(ctx, data)
}

func (g *GormRoleRepository) Create(ctx context.Context, data role.Role) error {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return err
	}

	entity, permissions := toDBRole(data)
	var id uint
	if err := tx.QueryRow(ctx, roleInsertQuery,
		entity.Name,
		entity.Description,
	).Scan(&id); err != nil {
		return err
	}

	for _, permission := range permissions {
		if err := g.execQuery(ctx, roleInsertPermissionQuery,
			data.ID,
			permission.ID,
		); err != nil {
			return err
		}
	}
	return nil
}

func (g *GormRoleRepository) Update(ctx context.Context, data role.Role) error {
	entity := toDBRole(data)

	if err := g.execQuery(ctx, roleUpdateQuery,
		entity.Name,
		entity.Description,
		entity.ID,
	); err != nil {
		return err
	}

	if err := g.execQuery(ctx, roleDeletePermissionsQuery, entity.ID); err != nil {
		return err
	}

	if permissions != nil {
		for _, permission := range permissions {
			if err := g.execQuery(ctx, roleInsertPermissionQuery,
				entity.ID,
				permission.ID,
			); err != nil {
				return err
			}
		}
	}
	return nil
}

func (g *GormRoleRepository) Delete(ctx context.Context, id uint) error {
	if err := g.execQuery(ctx, roleDeletePermissionsQuery, id); err != nil {
		return err
	}
	return g.execQuery(ctx, roleDeleteQuery, id)
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

	roleMap := make(map[uint]*models.Role)
	permissionMap := make(map[uint][]*models.Permission)

	for rows.Next() {
		var r models.Role
		var p models.Permission
		var permID sql.NullString // for UUID

		if err := rows.Scan(
			&r.ID,
			&r.Name,
			&r.Description,
			&r.CreatedAt,
			&r.UpdatedAt,
			&permID,
			&p.Name,
			&p.Resource,
			&p.Action,
			&p.Modifier,
			&p.Description,
		); err != nil {
			return nil, err
		}

		if _, ok := roleMap[r.ID]; !ok {
			roleMap[r.ID] = &r
		}

		if permID.Valid {
			id, err := uuid.Parse(permID.String)
			if err != nil {
				return nil, err
			}
			p.ID = id
			permissionMap[r.ID] = append(permissionMap[r.ID], &p)
		}
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	roles := make([]role.Role, 0, len(roleMap))
	for id, r := range roleMap {
		domainRole, err := toDomainRole(r, permissionMap[id])
		if err != nil {
			return nil, err
		}
		roles = append(roles, domainRole)
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
