package role

import (
	"github.com/iota-agency/iota-erp/internal/domain/entities/permission"
	"time"

	model "github.com/iota-agency/iota-erp/internal/interfaces/graph/gqlmodels"
)

type Role struct {
	ID          uint
	Name        string
	Description string
	Permissions []*permission.Permission
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func (r *Role) AddPermission(p *permission.Permission) *Role {
	return &Role{
		ID:          r.ID,
		Name:        r.Name,
		Description: r.Description,
		Permissions: append(r.Permissions, p),
		CreatedAt:   r.CreatedAt,
		UpdatedAt:   r.UpdatedAt,
	}
}

func (r *Role) Can(resource permission.Resource, action permission.Action) bool {
	for _, p := range r.Permissions {
		if p.Resource == resource && p.Action == action {
			return true
		}
	}
	return false
}

func (r *Role) ToGraph() *model.Role {
	return &model.Role{
		ID:          int64(r.ID),
		Name:        r.Name,
		Description: &r.Description,
		CreatedAt:   r.CreatedAt,
		UpdatedAt:   r.UpdatedAt,
	}
}
