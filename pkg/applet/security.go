package applet

import (
	"html"
	"regexp"
)

// sanitizeForJSON removes or escapes dangerous characters before JSON serialization.
// Prevents XSS attacks by HTML-escaping string values in nested maps.
// Returns a new sanitized map without modifying the original.
func sanitizeForJSON(data map[string]interface{}) map[string]interface{} {
	if data == nil {
		return nil
	}

	sanitized := make(map[string]interface{}, len(data))
	for key, value := range data {
		switch v := value.(type) {
		case string:
			// HTML-escape string values to prevent XSS
			sanitized[key] = html.EscapeString(v)
		case map[string]interface{}:
			// Recursively sanitize nested maps
			sanitized[key] = sanitizeForJSON(v)
		default:
			// Pass through non-string, non-map values unchanged
			sanitized[key] = value
		}
	}
	return sanitized
}

// validatePermissions ensures permissions are valid permission names.
// Returns only valid permissions, filtering out malformed ones.
// Limits to maxPermissions to prevent DoS attacks.
func validatePermissions(permissions []string) []string {
	const maxPermissions = 100
	const maxPermissionLength = 255

	if len(permissions) == 0 {
		return []string{}
	}

	validated := make([]string, 0, len(permissions))
	for _, perm := range permissions {
		// Skip empty or overly long permissions
		if len(perm) == 0 || len(perm) > maxPermissionLength {
			continue
		}

		// Validate permission format
		if !isValidPermissionFormat(perm) {
			continue
		}

		validated = append(validated, perm)

		// Prevent DoS by limiting total permissions
		if len(validated) >= maxPermissions {
			break
		}
	}
	return validated
}

// permissionRegex validates permission format: module.action (lowercase, dots, underscores)
// Example valid permissions: "bichat.access", "finance.read", "core.admin"
// Invalid: "BICHAT.ACCESS" (uppercase), "bichat" (no action), "bichat..access" (double dots)
var permissionRegex = regexp.MustCompile(`^[a-z][a-z0-9_]*(\.[a-z][a-z0-9_]*)*$`)

// isValidPermissionFormat checks if permission follows the format: module.action
// Must start with lowercase letter, contain only lowercase, digits, dots, underscores.
func isValidPermissionFormat(perm string) bool {
	return permissionRegex.MatchString(perm)
}
