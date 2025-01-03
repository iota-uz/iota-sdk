package persistence

import (
	"context"
	"fmt"
	"github.com/go-faster/errors"
	"strings"

	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/role"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/permission"
	"github.com/iota-uz/iota-sdk/modules/core/infrastructure/persistence/models"
	"github.com/iota-uz/iota-sdk/pkg/composables"
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
	pool, err := composables.UseTx(ctx)
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
	pool, err := composables.UseTx(ctx)
	if err != nil {
		return 0, err
	}
	var count int64
	if err := pool.QueryRow(ctx, `
		SELECT COUNT(*) as count FROM roles
	`).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

func (g *GormRoleRepository) GetAll(ctx context.Context) ([]*role.Role, error) {
	return g.GetPaginated(ctx, &role.FindParams{
		Limit: 100000,
	})
}

func (g *GormRoleRepository) GetByID(ctx context.Context, id uint) (*role.Role, error) {
	roles, err := g.GetPaginated(ctx, &role.FindParams{
		ID: id,
	})
	if err != nil {
		return nil, err
	}
	if len(roles) == 0 {
		return nil, ErrRoleNotFound
	}
	return roles[0], nil
}

func (g *GormRoleRepository) CreateOrUpdate(ctx context.Context, data *role.Role) error {
	u, err := g.GetByID(ctx, data.ID)
	if err != nil && !errors.Is(err, ErrRoleNotFound) {
		return err
	}
	if u != nil {
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

func (g *GormRoleRepository) Create(ctx context.Context, data *role.Role) error {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return err
	}
	entity, permissions := toDBRole(data)
	if err := tx.QueryRow(ctx, `
		INSERT INTO roles (name, description)
		VALUES ($1, $2)
	`, entity.Name, entity.Description).Scan(&data.ID); err != nil {
		return err
	}
	for _, permission := range permissions {
		if _, err := tx.Exec(ctx, `
			INSERT INTO role_permissions (role_id, permission_id)
			VALUES ($1, $2) ON CONFLICT (role_id, permission_id) DO NOTHING
		`, data.ID, permission.ID); err != nil {
			return err
		}
	}
	return nil
}

func (g *GormRoleRepository) Update(ctx context.Context, data *role.Role) error {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return composables.ErrNoTx
	}
	entity, permissions := toDBRole(data)
	if _, err := tx.Exec(ctx, `
		UPDATE roles
		SET name = $1, description = $2
		WHERE id = $3
	`, entity.Name, entity.Description, entity.ID); err != nil {
		return err
	}

	if _, err := tx.Exec(ctx, `DELETE FROM role_permissions WHERE role_id = $1`, entity.ID); err != nil {
		return err
	}
	for _, permission := range permissions {
		if _, err := tx.Exec(ctx, `
			INSERT INTO role_permissions (role_id, permission_id)
			VALUES ($1, $2) ON CONFLICT (role_id, permission_id) DO NOTHING
		`, entity.ID, permission.ID); err != nil {
			return err
		}
	}
	return nil
}

func (g *GormRoleRepository) Delete(ctx context.Context, id uint) error {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
		DELETE FROM roles WHERE id = $1
	`, id); err != nil {
		return err
	}
	return nil
}
