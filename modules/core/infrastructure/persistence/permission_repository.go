package persistence

import (
	"context"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/permission"
	"github.com/iota-uz/iota-sdk/modules/core/infrastructure/persistence/models"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/repo"
	"strings"
)

var (
	ErrPermissionNotFound = errors.New("permission not found")
)

const (
	permissionsSelectQuery = `SELECT id, name, resource, action, modifier FROM permissions`
	permissionsCountQuery  = `SELECT COUNT(*) FROM permissions`
	permissionsInsertQuery = `
		INSERT INTO permissions (name, resource, action, modifier)
		VALUES ($1, $2, $3, $4) 
		ON CONFLICT (name) DO UPDATE SET resource = permissions.resource
		RETURNING id`
	permissionsUpdateQuery = `
		UPDATE permissions
		SET name = $1, resource = $2, action = $3, modifier = $4
		WHERE id = $5`
	permissionsDeleteQuery = `DELETE FROM permissions WHERE id = $1`
)

type GormPermissionRepository struct{}

func NewPermissionRepository() permission.Repository {
	return &GormPermissionRepository{}
}

func (g *GormPermissionRepository) queryPermissions(
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

func (g *GormPermissionRepository) GetPaginated(
	ctx context.Context, params *permission.FindParams,
) ([]*permission.Permission, error) {
	pool, err := composables.UseTx(ctx)
	if err != nil {
		return nil, err
	}
	where, joins, args := []string{"1 = 1"}, []string{}, []interface{}{}

	if params.RoleID != 0 {
		joins, args = append(joins, fmt.Sprintf("INNER JOIN role_permissions rp ON rp.permission_id = permissions.id and rp.role_id = $%d", len(args)+1)), append(args, params.RoleID)
	}
	rows, err := pool.Query(ctx, repo.Join(
		permissionsSelectQuery,
		strings.Join(joins, " "),
		repo.JoinWhere(where...),
	))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	permissions := make([]*permission.Permission, 0)
	for rows.Next() {
		var p models.Permission
		if err := rows.Scan(
			&p.ID,
			&p.Name,
			&p.Resource,
			&p.Action,
			&p.Modifier,
		); err != nil {
			return nil, err
		}

		domainPermission, err := toDomainPermission(&p)
		if err != nil {
			return nil, err
		}
		permissions = append(permissions, domainPermission)
	}

	return permissions, nil
}

func (g *GormPermissionRepository) Count(ctx context.Context) (int64, error) {
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

func (g *GormPermissionRepository) GetAll(ctx context.Context) ([]*permission.Permission, error) {
	return g.queryPermissions(ctx, permissionsSelectQuery)
}

func (g *GormPermissionRepository) GetByID(ctx context.Context, id string) (*permission.Permission, error) {
	permissions, err := g.queryPermissions(ctx, permissionsSelectQuery+" WHERE id = $1", id)
	if err != nil {
		return nil, err
	}
	if len(permissions) == 0 {
		return nil, ErrPermissionNotFound
	}
	return permissions[0], nil
}

func (g *GormPermissionRepository) CreateOrUpdate(ctx context.Context, data *permission.Permission) error {
	_, err := g.GetByID(ctx, data.ID.String())
	if err != nil && !errors.Is(err, ErrPermissionNotFound) {
		return err
	}
	if errors.Is(err, ErrPermissionNotFound) {
		return g.Create(ctx, data)
	}
	return g.Update(ctx, data)
}

func (g *GormPermissionRepository) Create(ctx context.Context, data *permission.Permission) error {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return err
	}
	dbPerm := toDBPermission(data)
	if err := tx.QueryRow(
		ctx,
		permissionsInsertQuery,
		dbPerm.Name,
		dbPerm.Resource,
		dbPerm.Action,
		dbPerm.Modifier,
	).Scan(&data.ID); err != nil {
		return err
	}
	return nil
}

func (g *GormPermissionRepository) Update(ctx context.Context, data *permission.Permission) error {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return err
	}
	dbPerm := toDBPermission(data)
	if _, err := tx.Exec(
		ctx,
		permissionsUpdateQuery,
		dbPerm.Name,
		dbPerm.Resource,
		dbPerm.Action,
		dbPerm.Modifier,
		dbPerm.ID,
	); err != nil {
		return err
	}
	return nil
}

func (g *GormPermissionRepository) Delete(ctx context.Context, id string) error {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return err
	}
	if _, err := uuid.Parse(id); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, permissionsDeleteQuery, id); err != nil {
		return err
	}
	return nil
}
