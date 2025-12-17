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
	projectsPerms "github.com/iota-uz/iota-sdk/modules/projects/permissions"
	warehousePerms "github.com/iota-uz/iota-sdk/modules/warehouse/permissions"
)

// permissionSetBuilder helps create consistent permission sets
type permissionSetBuilder struct {
	module string
	prefix string
}

// newPermissionSetBuilder creates a new builder for a module
func newPermissionSetBuilder(module string) *permissionSetBuilder {
	return &permissionSetBuilder{
		module: module,
		prefix: "PermissionSets." + module + ".",
	}
}

// viewSet creates a "view" permission set for a resource
func (b *permissionSetBuilder) viewSet(resource string, readPerm permission.Permission) rbac.PermissionSet {
	return rbac.PermissionSet{
		Key:         resource + "_view",
		Label:       b.prefix + resource + "View.Label",
		Description: b.prefix + resource + "View._Description",
		Module:      b.module,
		Permissions: []permission.Permission{readPerm},
	}
}

// manageSet creates a "manage" permission set for a resource with full CRUD permissions
func (b *permissionSetBuilder) manageSet(resource string, create, read, update, deletePerm permission.Permission) rbac.PermissionSet {
	return rbac.PermissionSet{
		Key:         resource + "_manage",
		Label:       b.prefix + resource + "Manage.Label",
		Description: b.prefix + resource + "Manage._Description",
		Module:      b.module,
		Permissions: []permission.Permission{create, read, update, deletePerm},
	}
}

// AllPermissions returns all permissions from all modules
// This is used for seeding and RBAC initialization
func AllPermissions() []permission.Permission {
	// Pre-calculate total capacity to avoid slice re-allocations
	totalCapacity := len(corePerms.Permissions) +
		len(billingPerms.Permissions) +
		len(crmPerms.Permissions) +
		len(financePerms.Permissions) +
		len(hrmPerms.Permissions) +
		len(loggingPerms.Permissions) +
		len(projectsPerms.Permissions) +
		len(warehousePerms.Permissions)

	permissions := make([]permission.Permission, 0, totalCapacity)
	permissions = append(permissions, corePerms.Permissions...)
	permissions = append(permissions, billingPerms.Permissions...)
	permissions = append(permissions, crmPerms.Permissions...)
	permissions = append(permissions, financePerms.Permissions...)
	permissions = append(permissions, hrmPerms.Permissions...)
	permissions = append(permissions, loggingPerms.Permissions...)
	permissions = append(permissions, projectsPerms.Permissions...)
	permissions = append(permissions, warehousePerms.Permissions...)
	return permissions
}

// PermissionSchema returns the default permission schema with grouped permissions
func PermissionSchema() *rbac.PermissionSchema {
	sets := buildModulePermissionSets()

	// Add remaining modules as individual permission sets for now
	sets = appendRemainingPermissionSets(sets)

	return &rbac.PermissionSchema{Sets: sets}
}

// buildModulePermissionSets creates permission sets for all modules using the builder pattern
func buildModulePermissionSets() []rbac.PermissionSet {
	var sets []rbac.PermissionSet

	// Core module
	core := newPermissionSetBuilder("Core")
	sets = append(sets,
		core.viewSet("User", corePerms.UserRead),
		core.manageSet("User", corePerms.UserCreate, corePerms.UserRead, corePerms.UserUpdate, corePerms.UserDelete),
		core.viewSet("Role", corePerms.RoleRead),
		core.manageSet("Role", corePerms.RoleCreate, corePerms.RoleRead, corePerms.RoleUpdate, corePerms.RoleDelete),
		core.viewSet("Group", corePerms.GroupRead),
		core.manageSet("Group", corePerms.GroupCreate, corePerms.GroupRead, corePerms.GroupUpdate, corePerms.GroupDelete),
		core.viewSet("Upload", corePerms.UploadRead),
		core.manageSet("Upload", corePerms.UploadCreate, corePerms.UploadRead, corePerms.UploadUpdate, corePerms.UploadDelete),
	)

	// Finance module
	finance := newPermissionSetBuilder("Finance")
	sets = append(sets,
		finance.viewSet("Payment", financePerms.PaymentRead),
		finance.manageSet("Payment", financePerms.PaymentCreate, financePerms.PaymentRead, financePerms.PaymentUpdate, financePerms.PaymentDelete),
		finance.viewSet("Expense", financePerms.ExpenseRead),
		finance.manageSet("Expense", financePerms.ExpenseCreate, financePerms.ExpenseRead, financePerms.ExpenseUpdate, financePerms.ExpenseDelete),
		finance.viewSet("ExpenseCategory", financePerms.ExpenseCategoryRead),
		finance.manageSet("ExpenseCategory", financePerms.ExpenseCategoryCreate, financePerms.ExpenseCategoryRead, financePerms.ExpenseCategoryUpdate, financePerms.ExpenseCategoryDelete),
		finance.viewSet("Debt", financePerms.DebtRead),
		finance.manageSet("Debt", financePerms.DebtCreate, financePerms.DebtRead, financePerms.DebtUpdate, financePerms.DebtDelete),
	)

	// Projects module
	projects := newPermissionSetBuilder("Projects")
	sets = append(sets,
		projects.viewSet("Project", projectsPerms.ProjectRead),
		projects.manageSet("Project", projectsPerms.ProjectCreate, projectsPerms.ProjectRead, projectsPerms.ProjectUpdate, projectsPerms.ProjectDelete),
		projects.viewSet("ProjectStage", projectsPerms.ProjectStageRead),
		projects.manageSet("ProjectStage", projectsPerms.ProjectStageCreate, projectsPerms.ProjectStageRead, projectsPerms.ProjectStageUpdate, projectsPerms.ProjectStageDelete),
	)

	// Warehouse module
	warehouse := newPermissionSetBuilder("Warehouse")
	sets = append(sets,
		warehouse.viewSet("Product", warehousePerms.ProductRead),
		warehouse.manageSet("Product", warehousePerms.ProductCreate, warehousePerms.ProductRead, warehousePerms.ProductUpdate, warehousePerms.ProductDelete),
		warehouse.viewSet("Position", warehousePerms.PositionRead),
		warehouse.manageSet("Position", warehousePerms.PositionCreate, warehousePerms.PositionRead, warehousePerms.PositionUpdate, warehousePerms.PositionDelete),
		warehouse.viewSet("Order", warehousePerms.OrderRead),
		warehouse.manageSet("Order", warehousePerms.OrderCreate, warehousePerms.OrderRead, warehousePerms.OrderUpdate, warehousePerms.OrderDelete),
		warehouse.viewSet("Unit", warehousePerms.UnitRead),
		warehouse.manageSet("Unit", warehousePerms.UnitCreate, warehousePerms.UnitRead, warehousePerms.UnitUpdate, warehousePerms.UnitDelete),
		warehouse.viewSet("Inventory", warehousePerms.InventoryRead),
		warehouse.manageSet("Inventory", warehousePerms.InventoryCreate, warehousePerms.InventoryRead, warehousePerms.InventoryUpdate, warehousePerms.InventoryDelete),
	)

	// CRM module
	crm := newPermissionSetBuilder("CRM")
	sets = append(sets,
		crm.viewSet("Client", crmPerms.ClientRead),
		crm.manageSet("Client", crmPerms.ClientCreate, crmPerms.ClientRead, crmPerms.ClientUpdate, crmPerms.ClientDelete),
	)

	// HRM module
	hrm := newPermissionSetBuilder("HRM")
	sets = append(sets,
		hrm.viewSet("Employee", hrmPerms.EmployeeRead),
		hrm.manageSet("Employee", hrmPerms.EmployeeCreate, hrmPerms.EmployeeRead, hrmPerms.EmployeeUpdate, hrmPerms.EmployeeDelete),
	)

	return sets
}

// appendRemainingPermissionSets adds remaining modules as individual permission sets
func appendRemainingPermissionSets(sets []rbac.PermissionSet) []rbac.PermissionSet {
	// Collect all remaining permissions
	remainingPermissions := make([]permission.Permission, 0)
	remainingPermissions = append(remainingPermissions, billingPerms.Permissions...)
	remainingPermissions = append(remainingPermissions, loggingPerms.Permissions...)

	// Convert each permission to a permission set
	for _, perm := range remainingPermissions {
		sets = append(sets, rbac.PermissionSet{
			Key:         perm.ID().String(),
			Label:       "Permissions." + perm.Name(),
			Module:      "Core", // Assign to Core module for now
			Permissions: []permission.Permission{perm},
		})
	}

	return sets
}
