package role

import (
	"time"

	model "github.com/iota-agency/iota-erp/internal/interfaces/graph/gqlmodels"
)

type Role struct {
	ID          int64
	Name        string
	Description string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func (r *Role) ToGraph() *model.Role {
	return &model.Role{
		ID:          r.ID,
		Name:        r.Name,
		Description: &r.Description,
		CreatedAt:   r.CreatedAt,
		UpdatedAt:   r.UpdatedAt,
	}
}

type UserRole struct {
	UserID int64 `gorm:"primaryKey"`
	RoleID int64 `gorm:"primaryKey"`
}
