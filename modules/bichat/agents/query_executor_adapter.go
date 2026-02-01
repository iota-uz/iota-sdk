package agents

import (
	"context"

	bichatservices "github.com/iota-uz/iota-sdk/pkg/bichat/services"
	"github.com/iota-uz/iota-sdk/pkg/bichat/tools"
)

// queryExecutorAdapter adapts bichatservices.QueryExecutorService to tools.QueryExecutorService.
// This adapter bridges the two interfaces which have different QueryResult types.
//
// bichatservices.QueryResult uses Rows [][]any while tools.QueryResult uses Rows []map[string]interface{}.
// The adapter converts between these formats.
type queryExecutorAdapter struct {
	svc bichatservices.QueryExecutorService
}

// newQueryExecutorAdapter creates an adapter that wraps a bichatservices.QueryExecutorService
// and exposes it as a tools.QueryExecutorService.
func newQueryExecutorAdapter(svc bichatservices.QueryExecutorService) tools.QueryExecutorService {
	return &queryExecutorAdapter{svc: svc}
}

// ExecuteQuery executes a query and converts the result to tools.QueryResult format.
func (a *queryExecutorAdapter) ExecuteQuery(
	ctx context.Context,
	sql string,
	params []any,
	timeoutMs int,
) (*tools.QueryResult, error) {
	// Call the underlying service
	result, err := a.svc.ExecuteQuery(ctx, sql, params, timeoutMs)
	if err != nil {
		return nil, err
	}

	// Convert [][]any to []map[string]interface{}
	rows := make([]map[string]interface{}, len(result.Rows))
	for i, row := range result.Rows {
		rowMap := make(map[string]interface{}, len(result.Columns))
		for j, col := range result.Columns {
			if j < len(row) {
				rowMap[col] = row[j]
			}
		}
		rows[i] = rowMap
	}

	// Convert to tools.QueryResult
	return &tools.QueryResult{
		Columns:    result.Columns,
		Rows:       rows,
		RowCount:   result.RowCount,
		IsLimited:  result.Truncated,
		DurationMs: result.Duration,
	}, nil
}
