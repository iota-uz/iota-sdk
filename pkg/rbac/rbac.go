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
	Register(permissions ...*permission.Permission)
	Get(id uuid.UUID) (*permission.Permission, error)
	Permissions() []*permission.Permission
	PermissionsByResource() map[string][]*permission.Permission
}

type rbac struct {
	permissions []*permission.Permission
}

var _ RBAC = (*rbac)(nil)

func NewRbac() RBAC {
	return &rbac{
		permissions: []*permission.Permission{},
	}
}

func (r *rbac) Register(permissions ...*permission.Permission) {
	r.permissions = append(r.permissions, permissions...)
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
