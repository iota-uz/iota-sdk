package permissions

import (
	"context"
)

// ViewInfo contains view name and access status for schema_list output.
type ViewInfo struct {
	Name   string `json:"name"`
	Access string `json:"access"` // "ok" | "denied"
}

// DeniedView contains information about a view that was denied access.
type DeniedView struct {
	Name                string   `json:"name"`
	RequiredPermissions []string `json:"required_permissions"` // e.g., ["Expense.Read OR Payment.Read"]
}

// ViewAccessControl defines the interface for checking view permissions.
// Implementations validate user permissions against database views for SQL execution.
type ViewAccessControl interface {
	// CanAccess checks if the current user can access the specified view.
	// Returns true if:
	// - View is not configured (and DefaultAccess is true)
	// - View has nil/empty Required permissions (public view)
	// - User has at least one required permission (LogicAny/OR)
	// - User has all required permissions (LogicAll/AND)
	CanAccess(ctx context.Context, viewName string) (bool, error)

	// GetAccessibleViews returns view information with access status for all provided views.
	// Used by schema_list tool to show users which views they can access.
	// Each view is annotated with "access": "ok" or "access": "denied".
	GetAccessibleViews(ctx context.Context, views []string) ([]ViewInfo, error)

	// CheckQueryPermissions parses SQL and checks if user has access to all referenced views.
	// Returns a list of denied views with their required permissions.
	// Used by sql_execute tool to validate queries before execution.
	// Returns empty slice if all views are accessible or if no views are referenced.
	CheckQueryPermissions(ctx context.Context, sql string) ([]DeniedView, error)

	// GetRequiredPermissions returns the required permissions for a view.
	// Used for error messages when access is denied.
	// Returns nil if view is not configured or is public.
	GetRequiredPermissions(viewName string) []string
}
