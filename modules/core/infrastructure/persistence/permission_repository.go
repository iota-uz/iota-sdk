package persistence

import (
	"context"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/permission"
	"github.com/iota-uz/iota-sdk/modules/core/infrastructure/persistence/models"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/graphql/helpers"
)

type GormPermissionRepository struct{}

func NewPermissionRepository() permission.Repository {
	return &GormPermissionRepository{}
}

func (g *GormPermissionRepository) GetPaginated(
	ctx context.Context, params *permission.FindParams,
) ([]permission.Permission, error) {
	pool, err := composables.UsePool(ctx)
	if err != nil {
		return nil, err
	}
	where, joins, args := []string{"1 = 1"}, []string{}, []interface{}{}
	if validID, err := uuid.Parse(params.ID); err == nil {
		where, args = append(where, fmt.Sprintf("id = $%d", len(args)+1)), append(args, validID)
	}

	if params.RoleID != 0 {
		joins, args = append(joins, fmt.Sprintf("INNER JOIN role_permissions rp ON rp.permission_id = permissions.id and rp.role_id = $%d", len(args)+1)), append(args, params.RoleID)
	}

	rows, err := pool.Query(ctx, `
		SELECT id, name, resource, action, modifier FROM permissions
		`+strings.Join(joins, "\n")+`
		WHERE `+strings.Join(where, " AND ")+``,
		args...)

	if err != nil {
		return nil, err
	}

	defer rows.Close()

	permissions := make([]permission.Permission, 0)
	for rows.Next() {
		var permission models.Permission
		if err := rows.Scan(
			&permission.ID,
			&permission.Name,
			&permission.Resource,
			&permission.Action,
			&permission.Modifier,
		); err != nil {
			return nil, err
		}

		domainPermission, err := toDomainPermission(permission)
		if err != nil {
			return nil, err
		}
		permissions = append(permissions, domainPermission)
	}

	return permissions, nil
}

func (g *GormPermissionRepository) Count(ctx context.Context) (int64, error) {
	pool, err := composables.UsePool(ctx)
	if err != nil {
		return 0, err
	}
	var count int64
	if err := pool.QueryRow(ctx, `
		SELECT COUNT(*) as count FROM permissions
	`).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

func (g *GormPermissionRepository) GetAll(ctx context.Context) ([]permission.Permission, error) {
	return g.GetPaginated(ctx, &permission.FindParams{
		Limit: 100000,
	})
}

func (g *GormPermissionRepository) GetByID(ctx context.Context, id string) (permission.Permission, error) {
	permissions, err := g.GetPaginated(ctx, &permission.FindParams{
		ID: id,
	})
	if err != nil {
		return permission.Permission{}, err
	}
	if len(permissions) == 0 {
		return permission.Permission{}, ErrPermissionNotFound
	}
	return permissions[0], nil
}

func (g *GormPermissionRepository) CreateOrUpdate(ctx context.Context, data *permission.Permission) error {
	u, err := g.GetByID(ctx, data.ID.String())
	if err != nil && !errors.Is(err, ErrPermissionNotFound) {
		return err
	}
	if u.ID.String() != "" {
		if err := g.Update(ctx, data); err != nil {
			return err
		}
	} else {
		if err := g.Create(ctx, data); err != nil {
			return err
		}
	}
	return nil
}

func (g *GormPermissionRepository) Create(ctx context.Context, data *permission.Permission) error {
	tx, err := composables.UsePoolTx(ctx)
	if err != nil {
		return err
	}
	dbPerm := toDBPermission(*data)
	if err := tx.QueryRow(ctx, `
		INSERT INTO permissions (name, resource, action, modifier)
		VALUES ($1, $2, $3, $4) RETURNING id
	`, dbPerm.Name, dbPerm.Resource, dbPerm.Action, dbPerm.Modifier).Scan(&data.ID); err != nil {
		return err
	}
	return nil
}

func (g *GormPermissionRepository) Update(ctx context.Context, data *permission.Permission) error {
	tx, err := composables.UsePoolTx(ctx)
	if err != nil {
		return err
	}
	dbPerm := toDBPermission(*data)
	if _, err := tx.Exec(ctx, `
		UPDATE permissions
		SET name = $1, resource = $2, action = $3, modifier = $4
		WHERE id = $5
	`, dbPerm.Name, dbPerm.Resource, dbPerm.Action, dbPerm.Modifier, dbPerm.ID); err != nil {
		return err
	}
	return nil
}

func (g *GormPermissionRepository) Delete(ctx context.Context, id string) error {
	tx, err := composables.UsePoolTx(ctx)
	if err != nil {
		return err
	}
	if _, err := uuid.Parse(id); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
		DELETE FROM permissions WHERE id = $1 
	`, id); err != nil {
		return err
	}
	return nil
}
