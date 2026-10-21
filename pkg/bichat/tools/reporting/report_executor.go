package reporting

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	bichatsql "github.com/iota-uz/iota-sdk/pkg/bichat/sql"
	"github.com/shopspring/decimal"
)

// ReportMaxRows is a resource limit, never a successful partial export.
const ReportMaxRows = 50000

var (
	ErrIncompleteReport = errors.New("report exceeds export capacity; narrow the period or scope; no complete file was generated")
	ErrReportShape      = errors.New("invalid report column metadata or row shape")
	ErrExcelPrecision   = errors.New("value cannot be represented safely as an Excel number")
)

// ReportExecutor preserves the SQL authorization boundary while making the
// completeness and cell type contract mandatory for every download path.
type ReportExecutor struct{ source bichatsql.QueryExecutor }

func NewReportExecutor(source bichatsql.QueryExecutor) *ReportExecutor {
	if existing, ok := source.(*ReportExecutor); ok {
		return existing
	}
	return &ReportExecutor{source: source}
}

func (e *ReportExecutor) ValidateQuery(ctx context.Context, query string) error {
	if validator, ok := e.source.(bichatsql.QueryValidator); ok {
		return validator.ValidateQuery(ctx, query)
	}
	return ErrReportShape
}

func (e *ReportExecutor) ExecuteQuery(ctx context.Context, query string, params []any, timeout time.Duration) (*bichatsql.QueryResult, error) {
	r, err := e.source.ExecuteQuery(ctx, query, params, timeout)
	if err != nil {
		return nil, err
	}
	if r == nil {
		return nil, ErrReportShape
	}
	if r.Truncated || len(r.Rows) > ReportMaxRows {
		return nil, ErrIncompleteReport
	}
	if len(r.Columns) != len(r.ColumnTypes) || r.RowCount != len(r.Rows) {
		return nil, ErrReportShape
	}
	copyResult := *r
	copyResult.Rows = make([][]any, len(r.Rows))
	for i, row := range r.Rows {
		if len(row) != len(r.Columns) {
			return nil, ErrReportShape
		}
		copyResult.Rows[i] = append([]any(nil), row...)
		for j, value := range row {
			if value == nil {
				continue
			}
			s, isString := value.(string)
			if !isString && r.ColumnTypes[j] == "number" {
				s = fmt.Sprint(value)
				isString = true
			}
			if !isString {
				continue
			}
			switch r.ColumnTypes[j] {
			case "number":
				d, err := decimal.NewFromString(s)
				if err != nil {
					return nil, fmt.Errorf("%w: row %d column %s", ErrReportShape, i+1, r.Columns[j])
				}
				f, _ := d.Float64()
				// Excel stores 15 significant decimal digits. Reject rounding rather
				// than silently corrupting money or turning it back into text.
				if math.IsInf(f, 0) || !decimal.NewFromFloat(f).Equal(d) || len(strings.TrimRight(d.Abs().Coefficient().String(), "0")) > 15 {
					return nil, fmt.Errorf("%w: row %d column %s", ErrExcelPrecision, i+1, r.Columns[j])
				}
				copyResult.Rows[i][j] = f
			case "date":
				var parsed time.Time
				var parseErr error
				for _, layout := range []string{time.RFC3339Nano, "2006-01-02", "15:04:05.999999999"} {
					parsed, parseErr = time.Parse(layout, s)
					if parseErr == nil {
						break
					}
				}
				if parseErr != nil {
					return nil, fmt.Errorf("%w: date column %s", ErrReportShape, r.Columns[j])
				}
				copyResult.Rows[i][j] = parsed
			}
		}
	}
	return &copyResult, nil
}
