package persistence

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/iota-agency/iota-sdk/modules/core/infrastructure/persistence/models"
	"github.com/iota-agency/iota-sdk/pkg/composables"
	"github.com/iota-agency/iota-sdk/pkg/domain/aggregates/role"
	"github.com/iota-agency/iota-sdk/pkg/domain/entities/permission"
)

var (
	ErrRoleNotFound = errors.New("role not found")
)

type GormRoleRepository struct {
	permissionRepo permission.Repository
}

func NewRoleRepository() role.Repository {
	return &GormRoleRepository{
		permissionRepo: NewPermissionRepository(),
	}
}

func (g *GormRoleRepository) GetPaginated(
	ctx context.Context, params *role.FindParams,
) ([]*role.Role, error) {
	pool, err := composables.UsePool(ctx)
	if err != nil {
		return nil, err
	}
	where, joins, args := []string{"1 = 1"}, []string{}, []interface{}{}

	if params.ID != 0 {
		where, args = append(where, fmt.Sprintf("id = $%d", len(args)+1)), append(args, params.ID)
	}

	if params.UserID != 0 {
		joins, args = append(joins, fmt.Sprintf("INNER JOIN user_roles ur ON ur.role_id = roles.id and ur.user_id = $%d", len(args)+1)), append(args, params.UserID)
	}

	rows, err := pool.Query(ctx, `
		SELECT id, name, description, roles.created_at, roles.updated_at FROM roles
		`+strings.Join(joins, "\n")+`
		WHERE `+strings.Join(where, " AND ")+`
	`, args...)

	if err != nil {
		return nil, err
	}

	defer rows.Close()

	roles := make([]*role.Role, 0)
	for rows.Next() {
		var role models.Role
		if err := rows.Scan(
			&role.ID,
			&role.Name,
			&role.Description,
			&role.CreatedAt,
			&role.UpdatedAt,
		); err != nil {
			return nil, err
		}

		domainRole, err := toDomainRole(&role)
		if err != nil {
			return nil, err
		}

		if params.AttachPermissions {
			if domainRole.Permissions, err = g.permissionRepo.GetPaginated(ctx, &permission.FindParams{
				RoleID: domainRole.ID,
			}); err != nil {
				return nil, err
			}
		}
		roles = append(roles, domainRole)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return roles, nil
}

func (g *GormRoleRepository) Count(ctx context.Context) (int64, error) {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return 0, composables.ErrNoTx
	}
	var count int64
	if err := tx.Model(&models.Role{}).Count(&count).Error; err != nil { //nolint:exhaustruct
		return 0, err
	}
	return count, nil
}

func (g *GormRoleRepository) GetAll(ctx context.Context) ([]*role.Role, error) {
	return nil, nil
	// tx, ok := composables.UseTx(ctx)
	// if !ok {
	// 	return nil, composables.ErrNoTx
	// }
	// var rows []*models.Role
	// if err := tx.Preload("Permissions").Find(&rows).Error; err != nil {
	// 	return nil, err
	// }
	// var entities []*role.Role
	// for _, row := range rows {
	// 	entities = append(entities, toDomainRole(row))
	// }
	// return entities, nil
}

func (g *GormRoleRepository) GetByID(ctx context.Context, id uint) (*role.Role, error) {
	roles, err := g.GetPaginated(ctx, &role.FindParams{
		ID: id,
	})
	if err != nil {
		return nil, err
	}
	if len(roles) == 0 {
		return nil, ErrPermissionNotFound
	}
	return roles[0], nil
}

func (g *GormRoleRepository) CreateOrUpdate(ctx context.Context, data *role.Role) error {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return composables.ErrNoTx
	}
	entity, permissions := toDBRole(data)
	if err := tx.Save(entity).Error; err != nil {
		return err
	}
	if err := tx.Model(entity).Association("Permissions").Replace(permissions); err != nil {
		return err
	}
	return nil
}

func (g *GormRoleRepository) Create(ctx context.Context, data *role.Role) error {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return composables.ErrNoTx
	}
	entity, permissions := toDBRole(data)
	if err := tx.Create(entity).Error; err != nil {
		return err
	}
	if err := tx.Model(entity).Association("Permissions").Append(permissions); err != nil {
		return err
	}
	return nil
}

func (g *GormRoleRepository) Update(ctx context.Context, data *role.Role) error {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return composables.ErrNoTx
	}
	entity, permissions := toDBRole(data)
	if err := tx.Updates(entity).Error; err != nil {
		return err
	}
	if err := tx.Model(entity).Association("Permissions").Replace(permissions); err != nil {
		return err
	}
	return nil
}

func (g *GormRoleRepository) Delete(ctx context.Context, id uint) error {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return composables.ErrNoTx
	}
	return tx.Delete(&models.Role{}, id).Error //nolint:exhaustruct
}
