package permission

import (
	model "github.com/iota-agency/iota-erp/internal/interfaces/graph/gqlmodels"
)

type Permission struct {
	ID          int64
	Description string
	Resource    string
	Action      string
	Modifier    string
}

func (p *Permission) ToGraph() *model.Permission {
	return &model.Permission{
		ID:          p.ID,
		Description: &p.Description,
		Resource:    &p.Resource,
		Action:      &p.Action,
		Modifier:    &p.Modifier,
	}
}

type RolePermissions struct {
	PermissionID int64
	RoleID       int64
}

func (rp *RolePermissions) ToGraph() *RolePermissions {
	return &RolePermissions{
		PermissionID: rp.PermissionID,
		RoleID:       rp.RoleID,
	}
}
