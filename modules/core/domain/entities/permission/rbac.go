package permission

import (
	"errors"
	"github.com/google/uuid"
)

var (
	ErrPermissionNotFound = errors.New("permission not found")
)

type RBAC interface {
	Register(permissions ...*Permission)
	Get(id uuid.UUID) (*Permission, error)
	Permissions() []*Permission
}

type rbac struct {
	permissions []*Permission
}

func NewRbac() RBAC {
	return &rbac{
		permissions: Permissions,
	}
}

func (r *rbac) Register(permissions ...*Permission) {
	r.permissions = append(r.permissions, permissions...)
}

func (r *rbac) Get(id uuid.UUID) (*Permission, error) {
	for _, p := range r.permissions {
		if p.ID == id {
			return p, nil
		}
	}
	return nil, ErrPermissionNotFound
}

func (r *rbac) Permissions() []*Permission {
	return r.permissions
}
