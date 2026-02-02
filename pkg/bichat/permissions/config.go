package permissions

import (
	"strings"

	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/permission"
)

// PermissionLogic defines how multiple permissions are evaluated.
type PermissionLogic int

const (
	// LogicAny means the user needs ANY ONE of the listed permissions (OR logic).
	// This is the default and most common case.
	LogicAny PermissionLogic = iota

	// LogicAll means the user needs ALL of the listed permissions (AND logic).
	// Use this for sensitive views that require multiple authorizations.
	LogicAll
)

// ViewPermission maps a database view to its required permissions.
type ViewPermission struct {
	// ViewName is the name of the database view (without schema prefix).
	// Example: "expenses", "payments_with_details"
	ViewName string

	// Required is a list of permissions needed to access this view.
	// If nil or empty, the view is considered public (accessible to all authenticated users).
	Required []permission.Permission

	// Logic defines how multiple permissions are evaluated:
	// - LogicAny (default): User needs ANY ONE of the permissions
	// - LogicAll: User needs ALL of the permissions
	Logic PermissionLogic

	// Description is an optional human-readable description of the view.
	// Used for error messages and documentation.
	Description string
}

// Config holds the view permission configuration.
type Config struct {
	// SchemaName is the database schema where views are located.
	// Example: "analytics", "mv_ui", "public"
	SchemaName string

	// ViewPermissions maps view names to their required permissions.
	// Only views listed here will have permission checks applied.
	ViewPermissions []ViewPermission

	// DefaultAccess controls behavior for unmapped views:
	// - true (default): Allow access to views not in ViewPermissions map
	// - false: Deny access to unmapped views
	DefaultAccess bool
}

// NewConfig creates a new Config with sensible defaults.
func NewConfig(schemaName string, viewPermissions []ViewPermission) *Config {
	return &Config{
		SchemaName:      schemaName,
		ViewPermissions: viewPermissions,
		DefaultAccess:   true, // Allow unmapped views by default (Option B)
	}
}

// NewConfigWithDenyDefault creates a Config that denies unmapped views.
// Use this for maximum security - only explicitly listed views are accessible.
func NewConfigWithDenyDefault(schemaName string, viewPermissions []ViewPermission) *Config {
	return &Config{
		SchemaName:      schemaName,
		ViewPermissions: viewPermissions,
		DefaultAccess:   false, // Deny unmapped views
	}
}

// buildPermissionMap creates a lookup map for efficient view permission checks.
func (c *Config) buildPermissionMap() map[string]ViewPermission {
	m := make(map[string]ViewPermission, len(c.ViewPermissions))
	for _, vp := range c.ViewPermissions {
		// Store with lowercase key for case-insensitive lookup
		m[normalizeViewName(vp.ViewName)] = vp
	}
	return m
}

// normalizeViewName converts view name to lowercase for case-insensitive matching.
func normalizeViewName(name string) string {
	// Simple lowercase - PostgreSQL identifiers are case-insensitive unless quoted
	return strings.ToLower(name)
}
