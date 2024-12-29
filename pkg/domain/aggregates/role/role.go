package role

import (
	"time"

	"github.com/iota-uz/iota-sdk/pkg/domain/entities/permission"
)

type Role struct {
	ID          uint
	Name        string
	Description string
	Permissions []permission.Permission
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func (r *Role) AddPermission(p permission.Permission) *Role {
	return &Role{
		ID:          r.ID,
		Name:        r.Name,
		Description: r.Description,
		Permissions: append(r.Permissions, p),
		CreatedAt:   r.CreatedAt,
		UpdatedAt:   r.UpdatedAt,
	}
}

func (r *Role) Can(perm permission.Permission) bool {
	for _, p := range r.Permissions {
		if p.Equals(perm) {
			return true
		}
	}
	return false
}
