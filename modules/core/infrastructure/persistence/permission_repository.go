package persistence

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/iota-agency/iota-sdk/modules/core/infrastructure/persistence/models"
	"github.com/iota-agency/iota-sdk/pkg/composables"
	"github.com/iota-agency/iota-sdk/pkg/domain/entities/permission"
)

var (
	ErrPermissionNotFound = errors.New("permission not found")
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
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return 0, composables.ErrNoTx
	}
	var count int64
	if err := tx.Model(&models.Permission{}).Count(&count).Error; err != nil { //nolint:exhaustruct
		return 0, err
	}
	return count, nil
}

func (g *GormPermissionRepository) GetAll(ctx context.Context) ([]permission.Permission, error) {
	return nil, nil
	// tx, ok := composables.UseTx(ctx)
	// if !ok {
	// 	return nil, composables.ErrNoTx
	// }
	// var rows []*models.Permission
	// if err := tx.Find(&rows).Error; err != nil {
	// 	return nil, err
	// }
	// entities := make([]*permission.Permission, len(rows))
	// for i, row := range rows {
	// 	entities[i] = toDomainPermission(row)
	// }
	// return entities, nil
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
	// tx, ok := composables.UseTx(ctx)
	// if !ok {
	// 	return composables.ErrNoTx
	// }
	// return tx.Save(toDBPermission(data)).Error
	return nil
}

func (g *GormPermissionRepository) Create(ctx context.Context, data *permission.Permission) error {
	// tx, ok := composables.UseTx(ctx)
	// if !ok {
	// 	return composables.ErrNoTx
	// }
	// return tx.Create(toDBPermission(data)).Error
	return nil
}

func (g *GormPermissionRepository) Update(ctx context.Context, data *permission.Permission) error {
	return nil
	// tx, ok := composables.UseTx(ctx)
	// if !ok {
	// 	return composables.ErrNoTx
	// }
	// return tx.Updates(toDBPermission(data)).Error
}

func (g *GormPermissionRepository) Delete(ctx context.Context, id string) error {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return composables.ErrNoTx
	}
	return tx.Delete(&models.Permission{}, id).Error //nolint:exhaustruct
}
