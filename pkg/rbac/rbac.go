package rbac

import (
	"errors"

	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/permission"

	"github.com/google/uuid"
)

var (
	ErrPermissionNotFound = errors.New("permission not found")
)

type Permission interface {
	Can(u user.User) bool
}

type rbacPermission struct {
	*permission.Permission
}

var _ Permission = (*rbacPermission)(nil)

func (p rbacPermission) Can(u user.User) bool {
	return u.Can(p.Permission)
}

type or struct {
	permissions []Permission
}

var _ Permission = (*or)(nil)

func (o or) Can(u user.User) bool {
	for _, p := range o.permissions {
		if p.Can(u) {
			return true
		}
	}
	return false
}

type and struct {
	permissions []Permission
}

var _ Permission = (*and)(nil)

func (a and) Can(u user.User) bool {
	for _, p := range a.permissions {
		if !p.Can(u) {
			return false
		}
	}
	return true
}

func Or(perms ...Permission) Permission {
	return or{permissions: perms}
}

func And(perms ...Permission) Permission {
	return and{permissions: perms}
}

func Perm(p *permission.Permission) Permission {
	return rbacPermission{Permission: p}
}

type RBAC interface {
	Get(id uuid.UUID) (*permission.Permission, error)
	Permissions() []*permission.Permission
	PermissionsByResource() map[string][]*permission.Permission
	// Deprecated: Use permission schema directly in UI controllers
	PermissionSets() []PermissionSet
	// Deprecated: Use permission schema directly in UI controllers
	Schema() *PermissionSchema
}

type rbac struct {
	permissions []*permission.Permission
	schema      *PermissionSchema // Optional, for backward compatibility
}

var _ RBAC = (*rbac)(nil)

// NewRbac creates a new RBAC instance with just permissions (no schema required)
func NewRbac(permissions []*permission.Permission) RBAC {
	if permissions == nil {
		permissions = []*permission.Permission{}
	}
	return &rbac{
		permissions: permissions,
		schema:      nil,
	}
}

// NewRbacWithSchema creates a new RBAC instance with a schema (for backward compatibility)
// Deprecated: Use NewRbac with permissions directly
func NewRbacWithSchema(schema *PermissionSchema) RBAC {
	if schema == nil {
		panic("RBAC schema is required")
	}
	if len(schema.Sets) == 0 {
		panic("RBAC schema must have at least one permission set defined")
	}

	// Extract all unique permissions from the schema sets
	permMap := make(map[string]*permission.Permission)
	for _, set := range schema.Sets {
		for _, perm := range set.Permissions {
			permMap[perm.ID.String()] = perm
		}
	}

	// Convert map to slice
	permissions := make([]*permission.Permission, 0, len(permMap))
	for _, perm := range permMap {
		permissions = append(permissions, perm)
	}

	return &rbac{
		permissions: permissions,
		schema:      schema,
	}
}

func (r *rbac) Get(id uuid.UUID) (*permission.Permission, error) {
	for _, p := range r.permissions {
		if p.ID == id {
			return p, nil
		}
	}
	return nil, ErrPermissionNotFound
}

func (r *rbac) Permissions() []*permission.Permission {
	return r.permissions
}

func (r *rbac) PermissionsByResource() map[string][]*permission.Permission {
	result := make(map[string][]*permission.Permission)

	for _, p := range r.permissions {
		resource := string(p.Resource)
		result[resource] = append(result[resource], p)
	}

	return result
}

func (r *rbac) Schema() *PermissionSchema {
	return r.schema
}

func (r *rbac) PermissionSets() []PermissionSet {
	if r.schema == nil {
		return []PermissionSet{}
	}
	return r.schema.Sets
}
