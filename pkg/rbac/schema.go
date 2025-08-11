package rbac

import (
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/permission"
)

// PermissionSchema defines how permissions are organized into sets
type PermissionSchema struct {
	Sets []PermissionSet
}

// PermissionSet defines a group of permissions that are managed together
type PermissionSet struct {
	Key         string                   // Unique identifier (e.g., "view", "manage")
	Label       string                   // Display name
	Description string                   // Optional description
	Module      string                   // Module name (e.g., "Core", "Finance", "CRM")
	Permissions []*permission.Permission // The actual permissions in this set
}
