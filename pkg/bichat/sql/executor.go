package sql

import (
	"context"
	"time"
)

// QueryExecutor executes SQL queries and returns results.
type QueryExecutor interface {
	// ExecuteQuery executes a SQL query and returns results.
	// The query is validated before execution to prevent dangerous operations.
	// timeout specifies the maximum execution time.
	ExecuteQuery(ctx context.Context, sql string, params []any, timeout time.Duration) (*QueryResult, error)
}

// QueryValidator validates SQL syntax and checks for dangerous operations
// without executing the query. Returns an error if validation fails.
type QueryValidator interface {
	ValidateQuery(sql string) error
}

// QueryResult contains the results of a SQL query execution.
// This is the canonical representation used across the codebase.
type QueryResult struct {
	// Columns are the column names in order
	Columns []string `json:"columns"`

	// ColumnTypes are the inferred data types in the same order as Columns.
	// Values: "string", "number", "boolean", "date".
	// May be nil if the executor does not provide type information.
	ColumnTypes []string `json:"column_types,omitempty"`

	// Rows contains the row data (each row is a slice of values matching Columns order)
	Rows [][]any `json:"rows"`

	// RowCount is the number of rows returned
	RowCount int `json:"row_count"`

	// Truncated indicates if result set was truncated due to limits
	Truncated bool `json:"truncated,omitempty"`

	// Duration is the execution duration
	Duration time.Duration `json:"-"`

	// SQL is the executed SQL (for reference/debugging)
	SQL string `json:"-"`
}

// ToMap converts a row to a map of column name -> value.
// Returns nil if rowIndex is out of bounds.
func (r *QueryResult) ToMap(rowIndex int) map[string]any {
	if rowIndex < 0 || rowIndex >= len(r.Rows) {
		return nil
	}

	result := make(map[string]any, len(r.Columns))
	for i, col := range r.Columns {
		if i < len(r.Rows[rowIndex]) {
			result[col] = r.Rows[rowIndex][i]
		}
	}

	return result
}

// AllMaps converts all rows to a slice of maps.
func (r *QueryResult) AllMaps() []map[string]any {
	result := make([]map[string]any, len(r.Rows))
	for i := range r.Rows {
		result[i] = r.ToMap(i)
	}
	return result
}

// PgOIDToColumnType maps a PostgreSQL type OID to a frontend column type string.
// Valid return values: "string", "number", "boolean", "date".
func PgOIDToColumnType(oid uint32) string {
	switch oid {
	case 21, 23, 20, 700, 701, 1700: // int2, int4, int8, float4, float8, numeric
		return "number"
	case 16: // bool
		return "boolean"
	case 1114, 1184, 1082, 1083: // timestamp, timestamptz, date, time
		return "date"
	default:
		return "string"
	}
}
