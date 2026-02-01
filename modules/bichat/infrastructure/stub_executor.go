package infrastructure

import (
	"context"

	"github.com/iota-uz/iota-sdk/pkg/bichat/tools"
)

// StubQueryExecutor is a placeholder QueryExecutor for development.
// Replace this with a real database query executor in production.
//
// To use a real executor:
//  1. Implement tools.QueryExecutorService interface
//  2. Connect to your database (PostgreSQL, MySQL, etc.)
//  3. Replace StubQueryExecutor in module.go
//
// Example with PostgreSQL:
//
//	type PostgresExecutor struct {
//	    db *sql.DB
//	}
//
//	func (e *PostgresExecutor) ExecuteQuery(ctx context.Context, sql string, params []any, timeoutMs int) (*tools.QueryResult, error) {
//	    rows, err := e.db.QueryContext(ctx, sql, params...)
//	    if err != nil {
//	        return nil, err
//	    }
//	    defer rows.Close()
//	    // ... process rows and return QueryResult
//	}
type StubQueryExecutor struct{}

// NewStubQueryExecutor creates a placeholder query executor.
func NewStubQueryExecutor() tools.QueryExecutorService {
	return &StubQueryExecutor{}
}

// ExecuteQuery returns a stub response.
func (e *StubQueryExecutor) ExecuteQuery(ctx context.Context, sql string, params []any, timeoutMs int) (*tools.QueryResult, error) {
	return &tools.QueryResult{
		Columns: []string{"message"},
		Rows: []map[string]interface{}{
			{"message": "SQL execution not configured. Please provide a real QueryExecutorService implementation."},
		},
		RowCount: 1,
	}, nil
}
