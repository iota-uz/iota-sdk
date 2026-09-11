package viewmodels

import "github.com/iota-uz/iota-sdk/modules/core/domain/entities/permission"

type AssignmentOption struct {
	ID          string
	Type        string
	Name        string
	Description string
	Permissions []permission.Permission
}

func (o *AssignmentOption) Role() *Role {
	return &Role{ID: o.ID, Type: o.Type, Name: o.Name, Description: o.Description}
}

func (o *AssignmentOption) Group() *Group {
	return &Group{ID: o.ID, Type: o.Type, Name: o.Name, Description: o.Description}
}
