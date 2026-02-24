package lens

import "context"

// DataSource executes queries against a data backend and returns tabular results.
type DataSource interface {
	// Execute runs a query and returns the result as rows of column→value maps.
	Execute(ctx context.Context, query string) (*QueryResult, error)

	// Close releases any resources held by the data source.
	Close() error
}

// QueryResult holds the tabular result of a query execution.
type QueryResult struct {
	Columns []QueryColumn          // ordered column metadata
	Rows    []map[string]any // each row is a map of column name → value
}

// QueryColumn describes a result column.
type QueryColumn struct {
	Name string
	Type string // "string", "number", "boolean", "timestamp"
}
