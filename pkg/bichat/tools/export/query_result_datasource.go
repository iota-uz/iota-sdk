package export

import (
	"context"

	bichatsql "github.com/iota-uz/iota-sdk/pkg/bichat/sql"
	"github.com/iota-uz/iota-sdk/pkg/excel"
)

// QueryResultDataSource adapts bichatsql.QueryResult to the excel.DataSource interface.
// This allows BiChat query results to use the SDK's rich Excel export infrastructure.
type QueryResultDataSource struct {
	result    *bichatsql.QueryResult
	sheetName string
}

// NewQueryResultDataSource creates a new adapter from a QueryResult.
func NewQueryResultDataSource(result *bichatsql.QueryResult) excel.DataSource {
	return &QueryResultDataSource{
		result:    result,
		sheetName: "Sheet1",
	}
}

// WithSheetName sets a custom sheet name.
func (d *QueryResultDataSource) WithSheetName(name string) *QueryResultDataSource {
	d.sheetName = name
	return d
}

// GetHeaders returns the column names from the query result.
func (d *QueryResultDataSource) GetHeaders() []string {
	return d.result.Columns
}

// GetRows returns an iterator function for fetching rows.
// The iterator pattern allows the SDK exporter to stream data without loading
// all rows into memory at once.
func (d *QueryResultDataSource) GetRows(ctx context.Context) (func() ([]interface{}, error), error) {
	index := 0

	return func() ([]interface{}, error) {
		// Check for context cancellation
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}

		// Check if we've exhausted all rows
		if index >= len(d.result.Rows) {
			return nil, nil // EOF
		}

		// Rows are already in slice format [][]any, just return the row
		row := d.result.Rows[index]
		index++
		return row, nil
	}, nil
}

// GetSheetName returns the name for the Excel sheet.
func (d *QueryResultDataSource) GetSheetName() string {
	return d.sheetName
}
