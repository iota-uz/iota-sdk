package applet

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestSanitizeForJSON tests XSS prevention
func TestSanitizeForJSON(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    map[string]interface{}
		expected map[string]interface{}
	}{
		{
			name: "XSS prevention - basic script tag",
			input: map[string]interface{}{
				"message": "<script>alert('xss')</script>",
			},
			expected: map[string]interface{}{
				"message": "&lt;script&gt;alert(&#39;xss&#39;)&lt;/script&gt;",
			},
		},
		{
			name: "XSS prevention - nested maps",
			input: map[string]interface{}{
				"user": map[string]interface{}{
					"name": "<img src=x onerror=alert(1)>",
				},
			},
			expected: map[string]interface{}{
				"user": map[string]interface{}{
					"name": "&lt;img src=x onerror=alert(1)&gt;",
				},
			},
		},
		{
			name: "non-string values pass through unchanged",
			input: map[string]interface{}{
				"count":   42,
				"enabled": true,
				"ratio":   3.14,
			},
			expected: map[string]interface{}{
				"count":   42,
				"enabled": true,
				"ratio":   3.14,
			},
		},
		{
			name:     "nil input returns nil",
			input:    nil,
			expected: nil,
		},
		{
			name:     "empty map returns empty map",
			input:    map[string]interface{}{},
			expected: map[string]interface{}{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sanitizeForJSON(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestValidatePermissions tests permission validation
func TestValidatePermissions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    []string
		expected []string
	}{
		{
			name:     "valid permissions accepted",
			input:    []string{"bichat.access", "finance.read", "core.admin"},
			expected: []string{"bichat.access", "finance.read", "core.admin"},
		},
		{
			name:     "empty permission filtered out",
			input:    []string{"bichat.access", "", "finance.read"},
			expected: []string{"bichat.access", "finance.read"},
		},
		{
			name: "overly long permission filtered out",
			input: []string{
				"bichat.access",
				string(make([]byte, 300)), // 300 chars > 255 limit
			},
			expected: []string{"bichat.access"},
		},
		{
			name:     "uppercase permissions normalized",
			input:    []string{"BICHAT.ACCESS", "bichat.access"},
			expected: []string{"bichat.access"},
		},
		{
			name:     "mixed case permissions normalized",
			input:    []string{"BiChat.Access", "Finance.Read"},
			expected: []string{"bichat.access", "finance.read"},
		},
		{
			name:     "format with no action part currently accepted (NOTE: may be bug)",
			input:    []string{"bichat", "bichat.access"},
			expected: []string{"bichat", "bichat.access"}, // Current behavior accepts single-part permissions
		},
		{
			name:     "invalid format - double dots rejected",
			input:    []string{"bichat..access", "bichat.access"},
			expected: []string{"bichat.access"},
		},
		{
			name: "max permissions limit enforced (100)",
			input: func() []string {
				perms := make([]string, 150)
				for i := 0; i < 150; i++ {
					perms[i] = fmt.Sprintf("module%d.action%d", i, i)
				}
				return perms
			}(),
			expected: func() []string {
				perms := make([]string, 100)
				for i := 0; i < 100; i++ {
					perms[i] = fmt.Sprintf("module%d.action%d", i, i)
				}
				return perms
			}(),
		},
		{
			name:     "empty input returns empty slice",
			input:    []string{},
			expected: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := validatePermissions(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestIsValidPermissionFormat tests permission format validation
func TestIsValidPermissionFormat(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		permission string
		valid      bool
	}{
		{name: "valid: module.action", permission: "bichat.access", valid: true},
		{name: "valid: nested module", permission: "finance.reports.read", valid: true},
		{name: "valid: with numbers", permission: "module1.action2", valid: true},
		{name: "valid: with underscores", permission: "my_module.my_action", valid: true},
		{name: "invalid: uppercase", permission: "Bichat.Access", valid: false},
		{name: "single word currently valid (NOTE: may be bug)", permission: "bichat", valid: true}, // Current regex accepts this
		{name: "invalid: double dots", permission: "bichat..access", valid: false},
		{name: "invalid: starts with dot", permission: ".bichat.access", valid: false},
		{name: "invalid: ends with dot", permission: "bichat.access.", valid: false},
		{name: "invalid: starts with number", permission: "1bichat.access", valid: false},
		{name: "invalid: contains space", permission: "bichat .access", valid: false},
		{name: "invalid: contains special chars", permission: "bichat@access", valid: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isValidPermissionFormat(tt.permission)
			assert.Equal(t, tt.valid, result)
		})
	}
}
