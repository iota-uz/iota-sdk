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
	Columns []string

	// Rows contains the row data (each row is a slice of values matching Columns order)
	Rows [][]any

	// RowCount is the number of rows returned
	RowCount int

	// Truncated indicates if result set was truncated due to limits
	Truncated bool

	// Duration is the execution duration
	Duration time.Duration

	// SQL is the executed SQL (for reference/debugging)
	SQL string
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
