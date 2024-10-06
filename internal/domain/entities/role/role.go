package role

import (
	"time"

	"github.com/iota-agency/iota-erp/internal/interfaces/graph/gqlmodels"
)

type Role struct {
	Id          int64
	Name        string
	Description string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func (r *Role) ToGraph() *model.Role {
	return &model.Role{
		ID:          r.Id,
		Name:        r.Name,
		Description: &r.Description,
		CreatedAt:   r.CreatedAt,
		UpdatedAt:   r.UpdatedAt,
	}
}

type UserRole struct {
	UserId int64 `gorm:"primaryKey"`
	RoleId int64 `gorm:"primaryKey"`
}
