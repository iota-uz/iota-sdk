package tools

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"

	"github.com/jackc/pgx/v5/pgconn"
)

// SQLErrorDiagnosis represents a structured diagnosis of a SQL error.
type SQLErrorDiagnosis struct {
	Code       ToolErrorCode `json:"code"`
	Message    string        `json:"message"`
	Table      string        `json:"table,omitempty"`
	Column     string        `json:"column,omitempty"`
	Suggestion string        `json:"suggestion"`
	Hints      []string      `json:"hints"`
}

// ClassifySQLError analyzes a SQL error and returns a structured diagnosis.
// It extracts PostgreSQL error codes and provides actionable recommendations.
func ClassifySQLError(err error) *SQLErrorDiagnosis {
	if err == nil {
		return &SQLErrorDiagnosis{
			Code:       ErrCodeQueryError,
			Message:    "unknown error",
			Suggestion: "Check query syntax and retry",
			Hints:      []string{HintCheckSQLSyntax},
		}
	}

	// Try to unwrap to PostgreSQL error
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		// Not a PostgreSQL error - return generic diagnosis
		return &SQLErrorDiagnosis{
			Code:       ErrCodeQueryError,
			Message:    err.Error(),
			Suggestion: "Check query syntax and database connectivity",
			Hints:      []string{HintCheckSQLSyntax, HintVerifyTableNames},
		}
	}

	// Map SQLSTATE code to specific diagnosis
	switch pgErr.Code {
	case "42703": // undefined_column
		return classifyUndefinedColumn(pgErr)

	case "42P01": // undefined_table
		return classifyUndefinedTable(pgErr)

	case "42804": // datatype_mismatch
		return classifyTypeMismatch(pgErr)

	case "42601": // syntax_error
		return classifySyntaxError(pgErr)

	case "42702": // ambiguous_column
		return classifyAmbiguousColumn(pgErr)

	default:
		// Unrecognized PostgreSQL error
		return &SQLErrorDiagnosis{
			Code:       ErrCodeQueryError,
			Message:    pgErr.Message,
			Suggestion: fmt.Sprintf("PostgreSQL error: %s (code: %s)", pgErr.Message, pgErr.Code),
			Hints:      []string{HintCheckSQLSyntax, HintVerifyTableNames},
		}
	}
}

// classifyUndefinedColumn handles "column does not exist" errors.
func classifyUndefinedColumn(pgErr *pgconn.PgError) *SQLErrorDiagnosis {
	// Try to extract column name from error message
	// Example: column "user_name" does not exist
	columnPattern := regexp.MustCompile(`column "([^"]+)" does not exist`)
	matches := columnPattern.FindStringSubmatch(pgErr.Message)

	var column, suggestion string
	hints := []string{HintCheckColumnExists, HintUseSchemaDescribe}

	if len(matches) > 1 {
		column = matches[1]
		suggestion = fmt.Sprintf("Column '%s' not found. Use schema_describe to verify available columns.", column)
	} else {
		suggestion = "Referenced column does not exist. Use schema_describe to check table structure."
	}

	// Try to extract table name from hint if available
	table := extractTableFromHint(pgErr.Hint)

	return &SQLErrorDiagnosis{
		Code:       ErrCodeColumnNotFound,
		Message:    pgErr.Message,
		Table:      table,
		Column:     column,
		Suggestion: suggestion,
		Hints:      hints,
	}
}

// classifyUndefinedTable handles "table does not exist" errors.
func classifyUndefinedTable(pgErr *pgconn.PgError) *SQLErrorDiagnosis {
	// Try to extract table name from error message
	// Example: relation "users" does not exist
	tablePattern := regexp.MustCompile(`relation "([^"]+)" does not exist`)
	matches := tablePattern.FindStringSubmatch(pgErr.Message)

	var table, suggestion string
	hints := []string{HintUseSchemaList, HintVerifyTableNames}

	if len(matches) > 1 {
		table = matches[1]
		suggestion = fmt.Sprintf("Table '%s' not found. Use schema_list to find available tables.", table)
	} else {
		suggestion = "Referenced table does not exist. Use schema_list to check available tables."
	}

	return &SQLErrorDiagnosis{
		Code:       ErrCodeTableNotFound,
		Message:    pgErr.Message,
		Table:      table,
		Suggestion: suggestion,
		Hints:      hints,
	}
}

// classifyTypeMismatch handles type mismatch errors.
func classifyTypeMismatch(pgErr *pgconn.PgError) *SQLErrorDiagnosis {
	return &SQLErrorDiagnosis{
		Code:       ErrCodeTypeMismatch,
		Message:    pgErr.Message,
		Suggestion: "Column type does not match expected type. Use schema_describe to check types and add appropriate casts (e.g., column::text).",
		Hints:      []string{HintCheckColumnTypes, HintUseSchemaDescribe},
	}
}

// classifySyntaxError handles SQL syntax errors.
func classifySyntaxError(pgErr *pgconn.PgError) *SQLErrorDiagnosis {
	return &SQLErrorDiagnosis{
		Code:       ErrCodeSyntaxError,
		Message:    pgErr.Message,
		Suggestion: "SQL syntax error. Review the query syntax - check for missing commas, parentheses, or keywords.",
		Hints:      []string{HintCheckSQLSyntax},
	}
}

// classifyAmbiguousColumn handles ambiguous column reference errors.
func classifyAmbiguousColumn(pgErr *pgconn.PgError) *SQLErrorDiagnosis {
	// Try to extract column name from error message
	// Example: column "id" is ambiguous
	columnPattern := regexp.MustCompile(`column "([^"]+)" (?:reference|is) ambiguous`)
	matches := columnPattern.FindStringSubmatch(pgErr.Message)

	var column, suggestion string
	hints := []string{HintDisambiguateColumn, HintUseSchemaDescribe}

	if len(matches) > 1 {
		column = matches[1]
		suggestion = fmt.Sprintf("Column '%s' is ambiguous. Qualify it with table alias (e.g., t1.%s).", column, column)
	} else {
		suggestion = "Column reference is ambiguous. Qualify column with table alias (e.g., table_alias.column_name)."
	}

	return &SQLErrorDiagnosis{
		Code:       ErrCodeAmbiguousColumn,
		Message:    pgErr.Message,
		Column:     column,
		Suggestion: suggestion,
		Hints:      hints,
	}
}

// extractTableFromHint tries to extract table name from PostgreSQL hint field.
func extractTableFromHint(hint string) string {
	if hint == "" {
		return ""
	}

	// Example hint: "Perhaps you meant to reference the column \"users.id\"."
	tablePattern := regexp.MustCompile(`"([^"]+)\.`)
	matches := tablePattern.FindStringSubmatch(hint)
	if len(matches) > 1 {
		return matches[1]
	}

	return ""
}

// FormatSQLDiagnosis formats a SQL diagnosis as JSON for LLM consumption.
// It uses the same JSON structure as FormatToolError but includes additional diagnostic fields.
func FormatSQLDiagnosis(diagnosis *SQLErrorDiagnosis) string {
	if diagnosis == nil {
		return FormatToolError(ErrCodeQueryError, "unknown error", HintCheckSQLSyntax)
	}

	wrapper := map[string]interface{}{
		"error": map[string]interface{}{
			"code":       diagnosis.Code,
			"message":    diagnosis.Message,
			"table":      diagnosis.Table,
			"column":     diagnosis.Column,
			"suggestion": diagnosis.Suggestion,
			"hints":      diagnosis.Hints,
		},
	}

	data, err := json.MarshalIndent(wrapper, "", "  ")
	if err != nil {
		// Fallback to simple format
		return FormatToolError(diagnosis.Code, diagnosis.Message, diagnosis.Hints...)
	}

	return string(data)
}
