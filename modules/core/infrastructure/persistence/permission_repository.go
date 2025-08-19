package persistence

import (
	"context"
	"errors"
	"fmt"

	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/permission"
	"github.com/iota-uz/iota-sdk/modules/core/infrastructure/persistence/models"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/repo"

	"github.com/google/uuid"
)

var (
	ErrPermissionNotFound = errors.New("permission not found")
)

const (
	permissionsSelectQuery = `SELECT id, name, resource, action, modifier, description FROM permissions`
	permissionsCountQuery  = `SELECT COUNT(*) FROM permissions`
	permissionsInsertQuery = `
		INSERT INTO permissions (id, name, resource, action, modifier, description)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (name) DO UPDATE SET resource = permissions.resource
		RETURNING id`
	permissionsUpdateQuery = `
		UPDATE permissions
		SET name = $1, resource = $2, action = $3, modifier = $4
		WHERE id = $5`
	permissionsDeleteQuery = `DELETE FROM permissions WHERE id = $1`
)

type PgPermissionRepository struct {
	fieldMap map[permission.Field]string
}

func NewPermissionRepository() permission.Repository {
	return &PgPermissionRepository{
		fieldMap: map[permission.Field]string{
			permission.NameField:     "name",
			permission.ResourceField: "resource",
			permission.ActionField:   "action",
			permission.ModifierField: "modifier",
		},
	}
}

func (g *PgPermissionRepository) GetPaginated(
	ctx context.Context, params *permission.FindParams,
) ([]*permission.Permission, error) {
	var where []string
	var joins []string
	var args []interface{}

	if params.RoleID != 0 {
		joins = append(joins, fmt.Sprintf("INNER JOIN role_permissions rp ON rp.permission_id = permissions.id and rp.role_id = $%d", len(args)+1))
		args = append(args, params.RoleID)
	}

	query := repo.Join(
		permissionsSelectQuery,
		repo.Join(joins...),
		params.SortBy.ToSQL(g.fieldMap),
		repo.JoinWhere(where...),
		repo.FormatLimitOffset(params.Limit, params.Offset),
	)

	return g.queryPermissions(ctx, query, args...)
}

func (g *PgPermissionRepository) Count(ctx context.Context) (int64, error) {
	pool, err := composables.UseTx(ctx)
	if err != nil {
		return 0, err
	}

	var count int64
	if err := pool.QueryRow(
		ctx,
		permissionsCountQuery,
	).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

func (g *PgPermissionRepository) GetAll(ctx context.Context) ([]*permission.Permission, error) {
	return g.queryPermissions(ctx, permissionsSelectQuery)
}

func (g *PgPermissionRepository) GetByID(ctx context.Context, id string) (*permission.Permission, error) {
	permissions, err := g.queryPermissions(ctx, permissionsSelectQuery+" WHERE id = $1", id)
	if err != nil {
		return nil, err
	}
	if len(permissions) == 0 {
		return nil, ErrPermissionNotFound
	}
	return permissions[0], nil
}

func (g *PgPermissionRepository) Save(ctx context.Context, data *permission.Permission) error {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return err
	}

	dbPerm := toDBPermission(data)

	if err := tx.QueryRow(
		ctx,
		permissionsInsertQuery,
		dbPerm.ID,
		dbPerm.Name,
		dbPerm.Resource,
		dbPerm.Action,
		dbPerm.Modifier,
		dbPerm.Description,
	).Scan(&data.ID); err != nil {
		return err
	}
	return nil
}

func (g *PgPermissionRepository) Delete(ctx context.Context, id string) error {
	if err := uuid.Validate(id); err != nil {
		return err
	}

	return g.execQuery(ctx, permissionsDeleteQuery, id)
}

func (g *PgPermissionRepository) queryPermissions(
	ctx context.Context,
	query string,
	args ...interface{},
) ([]*permission.Permission, error) {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return nil, err
	}

	rows, err := tx.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var permissions []*permission.Permission

	for rows.Next() {
		var p models.Permission

		if err := rows.Scan(
			&p.ID,
			&p.Name,
			&p.Resource,
			&p.Action,
			&p.Modifier,
			&p.Description,
		); err != nil {
			return nil, err
		}

		domainPermission, err := toDomainPermission(&p)
		if err != nil {
			return nil, err
		}
		permissions = append(permissions, domainPermission)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return permissions, nil
}

func (g *PgPermissionRepository) execQuery(ctx context.Context, query string, args ...interface{}) error {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, query, args...)
	return err
}
