package tools

import (
	"encoding/json"
	"fmt"
)

// ToolErrorCode represents a category of tool error for LLM self-correction.
type ToolErrorCode string

const (
	// ErrCodeInvalidRequest indicates malformed or invalid input parameters.
	// LLM should fix parameter format, types, or required fields.
	ErrCodeInvalidRequest ToolErrorCode = "INVALID_REQUEST"

	// ErrCodeQueryError indicates SQL syntax errors or database query failures.
	// LLM should fix SQL syntax, table/column names, or query structure.
	ErrCodeQueryError ToolErrorCode = "QUERY_ERROR"

	// ErrCodePolicyViolation indicates query violates security policies.
	// LLM should use only allowed operations (SELECT/WITH) and valid tables.
	ErrCodePolicyViolation ToolErrorCode = "POLICY_VIOLATION"

	// ErrCodeNoData indicates query/search returned no results.
	// LLM should try different filters, search terms, or table names.
	ErrCodeNoData ToolErrorCode = "NO_DATA"

	// ErrCodeDataTooLarge indicates result set exceeds size limits.
	// LLM should add LIMIT clauses or more specific WHERE filters.
	ErrCodeDataTooLarge ToolErrorCode = "DATA_TOO_LARGE"

	// ErrCodeServiceUnavailable indicates external service is down or unreachable.
	// LLM should retry or use alternative approach.
	ErrCodeServiceUnavailable ToolErrorCode = "SERVICE_UNAVAILABLE"
)

// ToolError represents a structured error with hints for LLM self-correction.
type ToolError struct {
	Code    ToolErrorCode `json:"code"`
	Message string        `json:"message"`
	Hints   []string      `json:"hints,omitempty"`
}

// Error implements the error interface.
func (e *ToolError) Error() string {
	return e.Message
}

// FormatToolError creates a JSON-formatted error string for LLM consumption.
// The error includes a code, message, and optional hints for self-correction.
//
// Example:
//
//	FormatToolError(
//	    ErrCodeQueryError,
//	    "syntax error at or near 'SELCT'",
//	    "Check query syntax - use SELECT not SELCT",
//	    "Verify table and column names exist",
//	)
//
// Returns JSON like:
//
//	{
//	  "error": {
//	    "code": "QUERY_ERROR",
//	    "message": "syntax error at or near 'SELCT'",
//	    "hints": [
//	      "Check query syntax - use SELECT not SELCT",
//	      "Verify table and column names exist"
//	    ]
//	  }
//	}
func FormatToolError(code ToolErrorCode, message string, hints ...string) string {
	toolErr := ToolError{
		Code:    code,
		Message: message,
		Hints:   hints,
	}

	wrapper := map[string]interface{}{
		"error": toolErr,
	}

	data, err := json.MarshalIndent(wrapper, "", "  ")
	if err != nil {
		// Fallback to plain error message if JSON marshaling fails
		return fmt.Sprintf(`{"error": {"code": "%s", "message": "%s"}}`, code, message)
	}

	return string(data)
}

// Common error hint templates for reuse across tools.
var (
	// SQL query hints
	HintCheckSQLSyntax      = "Check SQL syntax - ensure proper SELECT statement format"
	HintVerifyTableNames    = "Verify table and column names exist using schema_describe tool"
	HintUseSchemaList       = "Use schema_list tool to see all available tables and views"
	HintAddLimitClause      = "Add LIMIT clause to reduce result size (e.g., LIMIT 100)"
	HintFilterWithWhere     = "Use WHERE clause to filter more specifically"
	HintCheckJoinConditions = "Verify JOIN conditions are correct and columns exist"

	// Policy hints
	HintOnlySelectAllowed = "Only SELECT and WITH (CTE) queries are allowed"
	HintNoWriteOperations = "Write operations (INSERT, UPDATE, DELETE, DROP) are forbidden"
	HintCheckSchemaAccess = "Use schema_describe to check available tables in analytics schema"

	// Data hints
	HintTableMayBeEmpty   = "Table may be empty - use sql_execute to verify with COUNT(*)"
	HintTryDifferentTerms = "Try different search terms or broader filters"
	HintCheckDateRange    = "Verify date range filters are reasonable"

	// Service hints
	HintServiceMayBeDown = "External service may be temporarily unavailable"
	HintRetryLater       = "Try again in a few moments or use alternative approach"
	HintCheckConnection  = "Verify service connection and credentials"

	// Validation hints
	HintCheckRequiredFields = "Ensure all required parameters are provided"
	HintCheckFieldTypes     = "Verify parameter types match schema (string, integer, boolean)"
	HintCheckFieldFormat    = "Check parameter format (e.g., hex colors, valid identifiers)"
)
