package services

import (
	"context"
)

// QueryExecutorService provides BI-specific SQL execution capabilities.
// This service enables agents to query database schemas and execute SQL safely.
type QueryExecutorService interface {
	// SchemaList returns a list of all available tables with basic metadata
	SchemaList(ctx context.Context) ([]TableInfo, error)

	// SchemaDescribe returns detailed schema information for a specific table
	SchemaDescribe(ctx context.Context, tableName string) (*TableSchema, error)

	// ExecuteQuery executes a SQL query and returns results.
	// The query is validated before execution to prevent dangerous operations.
	// timeoutMs specifies the maximum execution time in milliseconds.
	ExecuteQuery(ctx context.Context, sql string, params []any, timeoutMs int) (*QueryResult, error)

	// ValidateQuery validates SQL syntax and checks for dangerous operations
	// without executing the query. Returns an error if validation fails.
	ValidateQuery(sql string) error
}

// TableInfo provides basic metadata about a database table
type TableInfo struct {
	Name        string
	Schema      string // Database schema (e.g., "public", "finance")
	RowCount    int64  // Approximate row count
	Description string // Optional table description/comment
}

// TableSchema provides detailed schema information for a table
type TableSchema struct {
	Name        string
	Schema      string
	Description string
	Columns     []ColumnInfo
	Indexes     []IndexInfo
	PrimaryKey  []string // Column names in primary key
	ForeignKeys []ForeignKeyInfo
}

// ColumnInfo provides metadata about a table column
type ColumnInfo struct {
	Name         string
	Type         string // SQL data type (e.g., "integer", "varchar(255)")
	Nullable     bool
	DefaultValue *string
	Description  string
	IsPrimaryKey bool
	IsForeignKey bool
}

// IndexInfo provides metadata about a table index
type IndexInfo struct {
	Name      string
	Columns   []string
	IsUnique  bool
	IsPrimary bool
}

// ForeignKeyInfo provides metadata about a foreign key relationship
type ForeignKeyInfo struct {
	Name             string
	Column           string
	ReferencedTable  string
	ReferencedColumn string
}

// QueryResult contains the results of a SQL query execution
type QueryResult struct {
	Columns   []string // Column names
	Rows      [][]any  // Row data (each row is a slice of values)
	RowCount  int      // Number of rows returned
	Truncated bool     // True if result set was truncated
	Duration  int64    // Execution duration in milliseconds
	SQL       string   // The executed SQL (for reference)
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
