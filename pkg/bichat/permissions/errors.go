package permissions

import (
	"fmt"
	"strings"

	"github.com/iota-uz/iota-sdk/pkg/analytics"
)

// FormatPermissionError creates a user-friendly error message for permission denial.
// This message is intended for LLM consumption to explain why access was denied.
//
// Example output:
//
//	"Access denied: John Doe does not have permission to access 'financial_reports'.
//	 Required permissions: Expense.Read AND Payment.Read"
//
// For multiple denied views:
//
//	"Query blocked: John Doe cannot access the following views:
//	 - financial_reports (requires: Expense.Read AND Payment.Read)
//	 - sensitive_data (requires: Admin.Read)"
func FormatPermissionError(userName string, deniedViews []DeniedView) string {
	if len(deniedViews) == 0 {
		return ""
	}

	if len(deniedViews) == 1 {
		return formatSingleDeniedView(userName, deniedViews[0])
	}

	return formatMultipleDeniedViews(userName, deniedViews)
}

// formatSingleDeniedView formats an error for a single denied view.
func formatSingleDeniedView(userName string, denied DeniedView) string {
	var permStr string
	if len(denied.RequiredPermissions) == 0 {
		permStr = "view does not exist or access is not configured"
	} else if len(denied.RequiredPermissions) == 1 {
		permStr = fmt.Sprintf("Required permission: %s", denied.RequiredPermissions[0])
	} else {
		permStr = fmt.Sprintf("Required permissions: %s", strings.Join(denied.RequiredPermissions, ", "))
	}

	return fmt.Sprintf(
		"Access denied: %s does not have permission to access '%s'. %s",
		userName,
		denied.Name,
		permStr,
	)
}

// formatMultipleDeniedViews formats an error for multiple denied views.
func formatMultipleDeniedViews(userName string, deniedViews []DeniedView) string {
	var errMsg strings.Builder
	errMsg.WriteString(fmt.Sprintf("Query blocked: %s cannot access the following views:\n", userName))

	for _, dv := range deniedViews {
		permList := "view does not exist or access is not configured"
		if len(dv.RequiredPermissions) > 0 {
			permList = strings.Join(dv.RequiredPermissions, ", ")
		}

		errMsg.WriteString(fmt.Sprintf("- %s (requires: %s)\n", dv.Name, permList))
	}

	return errMsg.String()
}

// FormatRequiredPermissions converts a list of permission names with logic to a readable string.
//
// Examples:
//   - ["Expense.Read"] with LogicAny -> "Expense.Read"
//   - ["Expense.Read", "Payment.Read"] with LogicAny -> "Expense.Read OR Payment.Read"
//   - ["Expense.Read", "Payment.Read"] with LogicAll -> "Expense.Read AND Payment.Read"
func FormatRequiredPermissions(permNames []string, logic analytics.PermissionLogic) string {
	if len(permNames) == 0 {
		return ""
	}

	if len(permNames) == 1 {
		return permNames[0]
	}

	var operator string
	switch logic {
	case analytics.LogicAll:
		operator = " AND "
	case analytics.LogicAny:
		operator = " OR "
	default:
		operator = " OR "
	}

	return strings.Join(permNames, operator)
}
