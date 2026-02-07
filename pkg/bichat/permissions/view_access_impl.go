package permissions

import (
	"context"
	"regexp"
	"strings"

	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/permission"
	"github.com/iota-uz/iota-sdk/pkg/analytics"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

// tableReferenceRegex matches table/view references in SQL queries.
// Matches patterns like: FROM analytics.table_name, JOIN analytics.table_name
// Case-insensitive, captures the table name after schema prefix.
var tableReferenceRegex = regexp.MustCompile(`(?i)\b(?:FROM|JOIN)\s+(\w+)\.(\w+)`)

// viewAccessControl implements ViewAccessControl interface.
type viewAccessControl struct {
	permissionMap   map[string]analytics.View
	schemaNameLower string
	defaultAccess   bool
}

// NewViewAccessControl creates a new ViewAccessControl instance.
// schema is the database schema to match in SQL queries.
// views are the registered analytics views with their permission requirements.
// defaultAccess controls behavior for unmapped views (true = allow, false = deny).
func NewViewAccessControl(schema string, views []analytics.View, defaultAccess bool) ViewAccessControl {
	m := make(map[string]analytics.View, len(views))
	for _, v := range views {
		m[normalizeViewName(v.Name)] = v
	}

	return &viewAccessControl{
		permissionMap:   m,
		schemaNameLower: strings.ToLower(schema),
		defaultAccess:   defaultAccess,
	}
}

// CanAccess checks if the current user can access the specified view.
func (v *viewAccessControl) CanAccess(ctx context.Context, viewName string) (bool, error) {
	const op serrors.Op = "viewAccessControl.CanAccess"

	u, err := composables.UseUser(ctx)
	if err != nil {
		return false, serrors.E(op, "failed to get user from context", err)
	}

	normalizedName := normalizeViewName(viewName)

	vp, exists := v.permissionMap[normalizedName]
	if !exists {
		return v.defaultAccess, nil
	}

	if len(vp.Required) == 0 {
		return true, nil
	}

	switch vp.Logic {
	case analytics.LogicAll:
		return v.hasAllPermissions(u, vp.Required), nil
	case analytics.LogicAny:
		return v.hasAnyPermission(u, vp.Required), nil
	default:
		return v.hasAnyPermission(u, vp.Required), nil
	}
}

// hasAnyPermission checks if user has at least one of the required permissions (OR logic).
func (v *viewAccessControl) hasAnyPermission(u user.User, required []permission.Permission) bool {
	for _, reqPerm := range required {
		if v.hasPermission(u, reqPerm) {
			return true
		}
	}
	return false
}

// hasAllPermissions checks if user has all of the required permissions (AND logic).
func (v *viewAccessControl) hasAllPermissions(u user.User, required []permission.Permission) bool {
	for _, reqPerm := range required {
		if !v.hasPermission(u, reqPerm) {
			return false
		}
	}
	return true
}

// hasPermission checks if user has a specific permission (direct or via roles).
func (v *viewAccessControl) hasPermission(u user.User, reqPerm permission.Permission) bool {
	for _, role := range u.Roles() {
		for _, userPerm := range role.Permissions() {
			if userPerm.ID() == reqPerm.ID() {
				return true
			}
		}
	}
	return false
}

// GetAccessibleViews returns view information with access status for all provided views.
func (v *viewAccessControl) GetAccessibleViews(ctx context.Context, views []string) ([]ViewInfo, error) {
	const op serrors.Op = "viewAccessControl.GetAccessibleViews"
	result := make([]ViewInfo, 0, len(views))

	for _, viewName := range views {
		canAccess, err := v.CanAccess(ctx, viewName)
		if err != nil {
			return nil, serrors.E(op, "failed to check access for view "+viewName, err)
		}

		access := "denied"
		if canAccess {
			access = "ok"
		}

		result = append(result, ViewInfo{
			Name:   viewName,
			Access: access,
		})
	}

	return result, nil
}

// CheckQueryPermissions parses SQL and checks if user has access to all referenced views.
func (v *viewAccessControl) CheckQueryPermissions(ctx context.Context, sql string) ([]DeniedView, error) {
	const op serrors.Op = "viewAccessControl.CheckQueryPermissions"

	matches := tableReferenceRegex.FindAllStringSubmatch(sql, -1)
	if len(matches) == 0 {
		return nil, nil
	}

	viewSet := make(map[string]bool)
	for _, match := range matches {
		if len(match) >= 3 {
			schemaName := strings.ToLower(match[1])
			viewName := strings.ToLower(match[2])

			if v.schemaNameLower == "" || schemaName == v.schemaNameLower {
				viewSet[viewName] = true
			}
		}
	}

	var deniedViews []DeniedView
	for viewName := range viewSet {
		canAccess, err := v.CanAccess(ctx, viewName)
		if err != nil {
			return nil, serrors.E(op, "failed to check access for view "+viewName, err)
		}

		if !canAccess {
			requiredPerms := v.GetRequiredPermissions(viewName)

			deniedViews = append(deniedViews, DeniedView{
				Name:                viewName,
				RequiredPermissions: requiredPerms,
			})
		}
	}

	return deniedViews, nil
}

// GetRequiredPermissions returns the required permissions for a view.
func (v *viewAccessControl) GetRequiredPermissions(viewName string) []string {
	normalizedName := normalizeViewName(viewName)
	vp, exists := v.permissionMap[normalizedName]
	if !exists {
		return nil
	}

	if len(vp.Required) == 0 {
		return nil
	}

	return extractPermissionNames(vp)
}

// extractPermissionNames converts an analytics.View's required permissions to names.
func extractPermissionNames(v analytics.View) []string {
	if len(v.Required) == 0 {
		return nil
	}

	names := make([]string, len(v.Required))
	for i, perm := range v.Required {
		names[i] = perm.Name()
	}
	return names
}

// normalizeViewName converts view name to lowercase for case-insensitive matching.
func normalizeViewName(name string) string {
	return strings.ToLower(name)
}
