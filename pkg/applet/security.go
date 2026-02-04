package applet

import (
	"html"
	"regexp"
)

// sanitizeForJSON removes or escapes dangerous characters before JSON serialization.
// Prevents XSS attacks by HTML-escaping string values in nested maps and arrays.
// Returns a new sanitized structure without modifying the original.
// Supports: maps, arrays/slices, strings, and basic JSON primitives (numbers, bools, null).
func sanitizeForJSON(data map[string]interface{}) map[string]interface{} {
	if data == nil {
		return nil
	}

	sanitized := make(map[string]interface{}, len(data))
	for key, value := range data {
		sanitized[key] = sanitizeValue(value)
	}
	return sanitized
}

// sanitizeValue recursively sanitizes a value, handling maps, arrays, and strings.
func sanitizeValue(value interface{}) interface{} {
	switch v := value.(type) {
	case string:
		// HTML-escape string values to prevent XSS
		return html.EscapeString(v)
	case map[string]interface{}:
		// Recursively sanitize nested maps
		return sanitizeForJSON(v)
	case []interface{}:
		// Recursively sanitize arrays/slices
		sanitized := make([]interface{}, len(v))
		for i, item := range v {
			sanitized[i] = sanitizeValue(item)
		}
		return sanitized
	case []string:
		// Handle []string specifically (common case)
		sanitized := make([]string, len(v))
		for i, s := range v {
			sanitized[i] = html.EscapeString(s)
		}
		return sanitized
	case []map[string]interface{}:
		// Handle []map[string]interface{} specifically
		sanitized := make([]map[string]interface{}, len(v))
		for i, m := range v {
			sanitized[i] = sanitizeForJSON(m)
		}
		return sanitized
	default:
		// Pass through basic JSON primitives (numbers, bools, null) unchanged
		// These are safe and don't need sanitization
		return value
	}
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
