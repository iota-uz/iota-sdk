package types

import (
	"github.com/a-h/templ"
	"github.com/iota-agency/iota-erp/internal/domain/aggregates/user"
	"github.com/iota-agency/iota-erp/internal/domain/entities/permission"
)

type NavigationItem struct {
	Name        string
	Href        string
	Children    []NavigationItem
	Icon        templ.Component
	Permissions []permission.Permission
}

func (n NavigationItem) HasPermission(user *user.User) bool {
	if n.Permissions == nil {
		return true
	}
	for _, perm := range n.Permissions {
		if !user.Can(perm) {
			return false
		}
	}
	return true
}
