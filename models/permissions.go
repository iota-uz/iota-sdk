package models

import model "github.com/iota-agency/iota-erp/graph/gqlmodels"

type Permission struct {
	Id          int64
	Description string
	Resource    string
	Action      string
	Modifier    string
}

func (p *Permission) ToGraph() *model.Permission {
	return &model.Permission{
		ID:          p.Id,
		Description: &p.Description,
		Resource:    &p.Resource,
		Action:      &p.Action,
		Modifier:    &p.Modifier,
	}
}

type RolePermissions struct {
	PermissionId int64
	RoleId       int64
}

func (rp *RolePermissions) ToGraph() *RolePermissions {
	return &RolePermissions{
		PermissionId: rp.PermissionId,
		RoleId:       rp.RoleId,
	}
}
