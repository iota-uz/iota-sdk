package controllers_test

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestReadonlyFieldsCSS verifies that CSS styles for readonly fields are properly defined
func TestReadonlyFieldsCSS(t *testing.T) {
	// Read the CSS file
	cssPath := "modules/core/presentation/assets/css/main.css"
	// Try from workspace root first
	cssBytes, err := os.ReadFile(cssPath)
	if err != nil {
		// Try relative path if running from test directory
		cssBytes, err = os.ReadFile("../assets/css/main.css")
	}
	require.NoError(t, err)
	cssContent := string(cssBytes)

	// Test cases for CSS rules that should exist
	testCases := []struct {
		name        string
		cssRule     string
		description string
	}{
		{
			name:        "Basic readonly input styles",
			cssRule:     ".form-control-input[readonly]",
			description: "Should have styles for readonly inputs",
		},
		{
			name:        "Readonly opacity",
			cssRule:     "opacity: 0.7",
			description: "Readonly fields should have reduced opacity",
		},
		{
			name:        "Readonly cursor",
			cssRule:     "cursor: not-allowed",
			description: "Readonly fields should show not-allowed cursor",
		},
		{
			name:        "Readonly pointer events",
			cssRule:     "pointer-events: none",
			description: "Readonly fields should disable pointer events",
		},
		{
			name:        "Readonly background color",
			cssRule:     "background-color: oklch(var(--gray-100))",
			description: "Readonly fields should have gray background",
		},
		{
			name:        "Dark mode readonly styles",
			cssRule:     ".dark .form-control-input[readonly]",
			description: "Should have dark mode styles for readonly inputs",
		},
		{
			name:        "Select readonly styles",
			cssRule:     "select.form-control-input[readonly]",
			description: "Should have styles for readonly select elements",
		},
		{
			name:        "Textarea readonly styles",
			cssRule:     "textarea.form-control-input[readonly]",
			description: "Should have styles for readonly textareas",
		},
		{
			name:        "Checkbox readonly styles",
			cssRule:     "input[type=\"checkbox\"][readonly]",
			description: "Should have styles for readonly checkboxes",
		},
		{
			name:        "Date input readonly styles",
			cssRule:     "input[type=\"date\"][readonly]",
			description: "Should have styles for readonly date inputs",
		},
		{
			name:        "Readonly focus state",
			cssRule:     ".form-control-input[readonly]:focus",
			description: "Should handle focus state for readonly inputs",
		},
		{
			name:        "No focus shadow",
			cssRule:     "box-shadow: none",
			description: "Readonly fields should not have focus shadow",
		},
		{
			name:        "Form control wrapper with readonly",
			cssRule:     ".form-control:has(input[readonly])",
			description: "Should handle form control wrapper with readonly input",
		},
		{
			name:        "Fallback for browsers without :has",
			cssRule:     ".form-control[data-readonly=\"true\"]",
			description: "Should have fallback for browsers without :has support",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert.True(t,
				strings.Contains(cssContent, tc.cssRule),
				"%s: %s", tc.description, tc.cssRule,
			)
		})
	}

	// Test that certain combinations exist
	t.Run("Complete readonly rule set", func(t *testing.T) {
		// Check that the main readonly rule has all required properties
		readonlyRuleIndex := strings.Index(cssContent, ".form-control-input[readonly] {")
		if assert.Greater(t, readonlyRuleIndex, -1, "Should find readonly rule") {
			// Find the closing brace
			closeIndex := strings.Index(cssContent[readonlyRuleIndex:], "}")
			if assert.Greater(t, closeIndex, -1, "Should find closing brace") {
				ruleContent := cssContent[readonlyRuleIndex : readonlyRuleIndex+closeIndex+1]

				// Check all properties are in this rule
				assert.Contains(t, ruleContent, "cursor: not-allowed")
				assert.Contains(t, ruleContent, "opacity: 0.7")
				assert.Contains(t, ruleContent, "pointer-events: none")
				assert.Contains(t, ruleContent, "background-color:")
			}
		}
	})

	// Test dark mode has different background
	t.Run("Dark mode background different from light mode", func(t *testing.T) {
		assert.Contains(t, cssContent, ".dark .form-control-input[readonly]")
		assert.Contains(t, cssContent, "background-color: oklch(var(--gray-700))")
	})
}
