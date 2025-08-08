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

// PermissionSchema returns the default CRUD-style permission schema
// with all permissions from all built-in modules
func PermissionSchema() *rbac.PermissionSchema {
	// Collect all permissions from all modules
	allPermissions := make([]*permission.Permission, 0)
	allPermissions = append(allPermissions, corePerms.Permissions...)
	allPermissions = append(allPermissions, billingPerms.Permissions...)
	allPermissions = append(allPermissions, crmPerms.Permissions...)
	allPermissions = append(allPermissions, financePerms.Permissions...)
	allPermissions = append(allPermissions, hrmPerms.Permissions...)
	allPermissions = append(allPermissions, loggingPerms.Permissions...)
	allPermissions = append(allPermissions, warehousePerms.Permissions...)

	// Create individual permission sets (CRUD style)
	// Each permission gets its own set
	sets := make([]rbac.PermissionSet, 0, len(allPermissions))
	for _, perm := range allPermissions {
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
