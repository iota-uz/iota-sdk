package tools

import (
	"encoding/json"
	"fmt"

	"github.com/iota-uz/iota-sdk/pkg/bichat/context/formatters"
	"github.com/iota-uz/iota-sdk/pkg/bichat/types"
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

	// ErrCodePermissionDenied indicates the user lacks permission to access the requested resource.
	// LLM should inform the user to contact administrator for access.
	ErrCodePermissionDenied ToolErrorCode = "PERMISSION_DENIED"

	// SQL-specific error codes for structured diagnostics
	// ErrCodeColumnNotFound indicates a referenced column does not exist in the table.
	// LLM should use schema_describe to verify column names.
	ErrCodeColumnNotFound ToolErrorCode = "COLUMN_NOT_FOUND"

	// ErrCodeTableNotFound indicates a referenced table does not exist.
	// LLM should use schema_list to find the correct table name.
	ErrCodeTableNotFound ToolErrorCode = "TABLE_NOT_FOUND"

	// ErrCodeTypeMismatch indicates column type does not match expected type.
	// LLM should use schema_describe to check column types and cast appropriately.
	ErrCodeTypeMismatch ToolErrorCode = "TYPE_MISMATCH"

	// ErrCodeSyntaxError indicates invalid SQL syntax.
	// LLM should review and fix the SQL syntax.
	ErrCodeSyntaxError ToolErrorCode = "SYNTAX_ERROR"

	// ErrCodeAmbiguousColumn indicates a column reference is ambiguous (exists in multiple tables).
	// LLM should qualify the column with table alias.
	ErrCodeAmbiguousColumn ToolErrorCode = "AMBIGUOUS_COLUMN"
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
	payload := types.ToolErrorPayload{
		Code:    string(code),
		Message: message,
		Hints:   hints,
	}
	registry := formatters.DefaultFormatterRegistry()
	if f := registry.Get(types.CodecToolError); f != nil {
		s, err := f.Format(payload, types.DefaultFormatOptions())
		if err == nil {
			return s
		}
	}
	// Fallback (should never happen — formatter is always registered)
	data, err := json.MarshalIndent(map[string]interface{}{
		"error": map[string]interface{}{
			"code":    string(code),
			"message": message,
			"hints":   hints,
		},
	}, "", "  ")
	if err != nil {
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

	// Permission hints
	HintRequestAccess        = "Contact administrator to request access to this resource"
	HintCheckAccessibleViews = "Use schema_list tool to see views you have permission to access"

	// SQL-specific diagnostic hints
	HintUseSchemaDescribe  = "Use schema_describe tool to verify column names and types for the table"
	HintCheckColumnTypes   = "Column types may differ from expected - verify with schema_describe"
	HintCheckColumnExists  = "Column may not exist in this table - use schema_describe to check available columns"
	HintDisambiguateColumn = "Qualify ambiguous column with table alias (e.g., t.column_name)"
)
