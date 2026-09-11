package query

import (
	"context"
	"strconv"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/permission"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/repo"
	"github.com/pkg/errors"
)

type AssignmentOption struct {
	ID          string
	Type        string
	Name        string
	Description string
	Permissions []permission.Permission
}

type UserFormOptionSet struct {
	Roles  []*AssignmentOption
	Groups []*AssignmentOption
}

// UserFormOptionsRepository is the tenant-scoped read contract for role and
// group choices shown on user create/edit forms. It deliberately returns every
// choice and never materializes write-side aggregates or group members.
type UserFormOptionsRepository interface {
	ListUserFormOptions(ctx context.Context) (*UserFormOptionSet, error)
}

type pgUserFormOptionsRepository struct{}

func NewPgUserFormOptionsRepository() UserFormOptionsRepository {
	return &pgUserFormOptionsRepository{}
}

func (r *pgUserFormOptionsRepository) ListUserFormOptions(ctx context.Context) (*UserFormOptionSet, error) {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get transaction")
	}
	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get tenant ID")
	}

	roles, err := loadRoleAssignmentOptions(ctx, tx, tenantID)
	if err != nil {
		return nil, err
	}
	groups, err := loadGroupAssignmentOptions(ctx, tx, tenantID)
	if err != nil {
		return nil, err
	}
	return &UserFormOptionSet{Roles: roles, Groups: groups}, nil
}

func loadRoleAssignmentOptions(ctx context.Context, tx repo.Tx, tenantID uuid.UUID) ([]*AssignmentOption, error) {
	rows, err := tx.Query(ctx, `
		SELECT r.id, r.type, r.name, COALESCE(r.description, '')
		FROM roles r
		WHERE r.tenant_id = $1
		ORDER BY LOWER(r.name), r.id`, tenantID)
	if err != nil {
		return nil, errors.Wrap(err, "failed to list role assignment options")
	}
	defer rows.Close()

	options := make([]*AssignmentOption, 0)
	byID := make(map[uint]*AssignmentOption)
	ids := make([]uint, 0)
	for rows.Next() {
		var id uint
		option := &AssignmentOption{Permissions: []permission.Permission{}}
		if err := rows.Scan(&id, &option.Type, &option.Name, &option.Description); err != nil {
			return nil, errors.Wrap(err, "failed to scan role assignment option")
		}
		option.ID = strconv.FormatUint(uint64(id), 10)
		options = append(options, option)
		byID[id] = option
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, errors.Wrap(err, "failed to iterate role assignment options")
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
		return nil, errors.Wrap(err, "failed to load role assignment permissions")
	}
	defer permissionRows.Close()
	for permissionRows.Next() {
		var roleID uint
		perm, err := scanAssignmentPermission(permissionRows, &roleID)
		if err != nil {
			return nil, errors.Wrap(err, "failed to scan role assignment permission")
		}
		if option := byID[roleID]; option != nil {
			option.Permissions = append(option.Permissions, perm)
		}
	}
	if err := permissionRows.Err(); err != nil {
		return nil, errors.Wrap(err, "failed to iterate role assignment permissions")
	}
	return options, nil
}

func loadGroupAssignmentOptions(ctx context.Context, tx repo.Tx, tenantID uuid.UUID) ([]*AssignmentOption, error) {
	rows, err := tx.Query(ctx, `
		SELECT g.id, g.type, g.name, COALESCE(g.description, '')
		FROM user_groups g
		WHERE g.tenant_id = $1
		ORDER BY LOWER(g.name), g.id`, tenantID)
	if err != nil {
		return nil, errors.Wrap(err, "failed to list group assignment options")
	}
	defer rows.Close()

	options := make([]*AssignmentOption, 0)
	byID := make(map[uuid.UUID]*AssignmentOption)
	ids := make([]uuid.UUID, 0)
	for rows.Next() {
		var id uuid.UUID
		option := &AssignmentOption{Permissions: []permission.Permission{}}
		if err := rows.Scan(&id, &option.Type, &option.Name, &option.Description); err != nil {
			return nil, errors.Wrap(err, "failed to scan group assignment option")
		}
		option.ID = id.String()
		options = append(options, option)
		byID[id] = option
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, errors.Wrap(err, "failed to iterate group assignment options")
	}
	if len(ids) == 0 {
		return options, nil
	}

	permissionRows, err := tx.Query(ctx, `
		SELECT gr.group_id, p.id, p.name, p.resource, p.action, p.modifier
		FROM group_roles gr
		JOIN user_groups g ON g.id = gr.group_id
		JOIN roles r ON r.id = gr.role_id AND r.tenant_id = g.tenant_id
		JOIN role_permissions rp ON rp.role_id = r.id
		JOIN permissions p ON p.id = rp.permission_id
		WHERE gr.group_id = ANY($1::uuid[]) AND g.tenant_id = $2
		GROUP BY gr.group_id, p.id, p.name, p.resource, p.action, p.modifier
		ORDER BY gr.group_id, p.name, p.id`, ids, tenantID)
	if err != nil {
		return nil, errors.Wrap(err, "failed to load group assignment permissions")
	}
	defer permissionRows.Close()
	for permissionRows.Next() {
		var groupID uuid.UUID
		perm, err := scanAssignmentPermission(permissionRows, &groupID)
		if err != nil {
			return nil, errors.Wrap(err, "failed to scan group assignment permission")
		}
		if option := byID[groupID]; option != nil {
			option.Permissions = append(option.Permissions, perm)
		}
	}
	if err := permissionRows.Err(); err != nil {
		return nil, errors.Wrap(err, "failed to iterate group assignment permissions")
	}
	return options, nil
}

type scanner interface{ Scan(dest ...any) error }

func scanAssignmentPermission[T uint | uuid.UUID](row scanner, ownerID *T) (permission.Permission, error) {
	var id uuid.UUID
	var name, resource, action, modifier string
	if err := row.Scan(ownerID, &id, &name, &resource, &action, &modifier); err != nil {
		return nil, err
	}
	return permission.New(
		permission.WithID(id), permission.WithName(name),
		permission.WithResource(permission.Resource(resource)),
		permission.WithAction(permission.Action(action)), permission.WithModifier(permission.Modifier(modifier)),
	), nil
}
