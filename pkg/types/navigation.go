// Package types provides this package.
package types

import (
	"github.com/a-h/templ"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/permission"
)

// PermissionLogic controls how a NavigationItem's Permissions are combined when
// deciding visibility. The zero value is PermissionLogicAll, preserving the
// historical AND semantics for items that don't set it explicitly.
type PermissionLogic int

const (
	PermissionLogicAll PermissionLogic = iota
	PermissionLogicAny
)

type NavigationItem struct {
	Key         string
	Workspace   string
	Pinned      bool
	Name        string
	Href        string
	Children    []NavigationItem
	Keywords    []string
	Icon        templ.Component
	Permissions []permission.Permission
	Logic       PermissionLogic
	IsBeta      bool
}

func (n NavigationItem) HasPermission(user user.User) bool {
	if len(n.Permissions) == 0 {
		return true
	}
	if n.Logic == PermissionLogicAny {
		for _, perm := range n.Permissions {
			if user.Can(perm) {
				return true
			}
		}
		return false
	}
	for _, perm := range n.Permissions {
		if !user.Can(perm) {
			return false
		}
	}
	return true
}

type NavWorkspace struct {
	Key     string
	Label   string
	Order   int
	Default bool
	IsBeta  bool
}
