package defaults

import (
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/permission"
	"github.com/iota-uz/iota-sdk/pkg/rbac"

	billingPerms "github.com/iota-uz/iota-sdk/modules/billing/permissions"
	corePerms "github.com/iota-uz/iota-sdk/modules/core/permissions"
	crmPerms "github.com/iota-uz/iota-sdk/modules/crm/permissions"
	financePerms "github.com/iota-uz/iota-sdk/modules/finance/permissions"
	hrmPerms "github.com/iota-uz/iota-sdk/modules/hrm/permissions"
	loggingPerms "github.com/iota-uz/iota-sdk/modules/logging/permissions"
	warehousePerms "github.com/iota-uz/iota-sdk/modules/warehouse/permissions"
)

// AllPermissions returns all permissions from all modules
// This is used for seeding and RBAC initialization
func AllPermissions() []*permission.Permission {
	permissions := make([]*permission.Permission, 0)
	permissions = append(permissions, corePerms.Permissions...)
	permissions = append(permissions, billingPerms.Permissions...)
	permissions = append(permissions, crmPerms.Permissions...)
	permissions = append(permissions, financePerms.Permissions...)
	permissions = append(permissions, hrmPerms.Permissions...)
	permissions = append(permissions, loggingPerms.Permissions...)
	permissions = append(permissions, warehousePerms.Permissions...)
	return permissions
}

// PermissionSchema returns the default permission schema with grouped permissions
func PermissionSchema() *rbac.PermissionSchema {
	sets := []rbac.PermissionSet{
		// Core module sets
		{
			Key:         "users_manage",
			Label:       "Manage Users",
			Description: "Full user management capabilities",
			Permissions: []*permission.Permission{
				corePerms.UserCreate,
				corePerms.UserRead,
				corePerms.UserUpdate,
				corePerms.UserDelete,
			},
		},
		{
			Key:         "users_view",
			Label:       "View Users",
			Description: "View user information only",
			Permissions: []*permission.Permission{
				corePerms.UserRead,
			},
		},
		{
			Key:         "roles_manage",
			Label:       "Manage Roles",
			Description: "Full role and permission management",
			Permissions: []*permission.Permission{
				corePerms.RoleCreate,
				corePerms.RoleRead,
				corePerms.RoleUpdate,
				corePerms.RoleDelete,
			},
		},
		{
			Key:         "roles_view",
			Label:       "View Roles",
			Description: "View role information only",
			Permissions: []*permission.Permission{
				corePerms.RoleRead,
			},
		},
		{
			Key:         "uploads_manage",
			Label:       "Manage Uploads",
			Description: "Full file upload management",
			Permissions: []*permission.Permission{
				corePerms.UploadCreate,
				corePerms.UploadRead,
				corePerms.UploadUpdate,
				corePerms.UploadDelete,
			},
		},
	}

	// Add other module permissions as individual sets for now
	// This maintains backward compatibility while allowing grouped sets above
	otherPermissions := make([]*permission.Permission, 0)
	otherPermissions = append(otherPermissions, billingPerms.Permissions...)
	otherPermissions = append(otherPermissions, crmPerms.Permissions...)
	otherPermissions = append(otherPermissions, financePerms.Permissions...)
	otherPermissions = append(otherPermissions, hrmPerms.Permissions...)
	otherPermissions = append(otherPermissions, loggingPerms.Permissions...)
	otherPermissions = append(otherPermissions, warehousePerms.Permissions...)

	for _, perm := range otherPermissions {
		sets = append(sets, rbac.PermissionSet{
			Key:         perm.ID.String(),
			Label:       perm.Name,
			Permissions: []*permission.Permission{perm},
		})
	}

	return &rbac.PermissionSchema{
		Sets: sets,
	}
}
