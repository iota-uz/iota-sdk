package permissions

import (
	"github.com/iota-uz/iota-sdk/pkg/bichat/permissions"
)

// This file provides default view permission mappings for the analytics schema.
// These are examples and should be customized based on your specific module permissions.
//
// Usage:
//
//	import bichatperms "github.com/iota-uz/iota-sdk/modules/bichat/permissions"
//
//	viewConfig := permissions.NewConfig("analytics", bichatperms.DefaultAnalyticsViewPermissions)
//	cfg := bichat.NewModuleConfig(..., bichat.WithViewAccessConfig(viewConfig))
//
// Or create custom mappings:
//
//	customViews := []permissions.ViewPermission{
//	    {ViewName: "expenses", Required: []permission.Permission{financeperm.ExpenseRead}},
//	    {ViewName: "payments", Required: []permission.Permission{financeperm.PaymentRead}},
//	}
//	viewConfig := permissions.NewConfig("analytics", customViews)

// DefaultAnalyticsViewPermissions provides example view-to-permission mappings.
// These are illustrative - replace with your actual permission imports and view names.
//
// The mappings use OR logic (LogicAny) by default, meaning a user needs ANY ONE
// of the listed permissions to access the view.
//
// For AND logic (require ALL permissions), use LogicAll:
//
//	{ViewName: "sensitive_data", Required: []permission.Permission{perm1, perm2}, Logic: permissions.LogicAll}
var DefaultAnalyticsViewPermissions = []permissions.ViewPermission{
	// Example: Public views (accessible to all authenticated users)
	// {ViewName: "reference_data", Required: nil},

	// Example: Views requiring specific permissions (replace with your actual permissions)
	// Finance views
	// {ViewName: "expenses", Required: []permission.Permission{financeperm.ExpenseRead}, Logic: LogicAny},
	// {ViewName: "payments", Required: []permission.Permission{financeperm.PaymentRead}, Logic: LogicAny},
	// {ViewName: "debts", Required: []permission.Permission{financeperm.DebtRead}, Logic: LogicAny},

	// Example: AND logic - requires ALL permissions
	// {ViewName: "financial_summary", Required: []permission.Permission{
	//     financeperm.ExpenseRead,
	//     financeperm.PaymentRead,
	// }, Logic: LogicAll},
}

// EmptyViewPermissions returns an empty configuration with allow-by-default policy.
// Use this when you want to enable the permission system but haven't configured
// specific view mappings yet. All views will be accessible.
func EmptyViewPermissions() []permissions.ViewPermission {
	return []permissions.ViewPermission{}
}
