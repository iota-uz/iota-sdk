package rbac

import (
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/permission"
)

// Permission interface for checking user permissions
type Permission interface {
	Can(u user.User) bool
}

type rbacPermission struct {
	permission permission.Permission
}

var _ Permission = (*rbacPermission)(nil)

func (p rbacPermission) Can(u user.User) bool {
	return u.Can(p.permission)
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

// Or creates a permission that passes if any of the given permissions pass
func Or(perms ...Permission) Permission {
	return or{permissions: perms}
}

// And creates a permission that passes only if all given permissions pass
func And(perms ...Permission) Permission {
	return and{permissions: perms}
}

// Perm wraps a permission.Permission into the Permission interface
func Perm(p permission.Permission) Permission {
	return rbacPermission{permission: p}
}
