package codecs

import (
	"fmt"

	"github.com/iota-uz/iota-sdk/pkg/bichat/context"
)

// QueryResultPayload represents a SQL query result block (BI-specific).
type QueryResultPayload struct {
	Query      string   `json:"query"`
	Columns    []string `json:"columns"`
	Rows       [][]any  `json:"rows"`
	RowCount   int      `json:"row_count"`
	Truncated  bool     `json:"truncated,omitempty"`
	MaxRows    int      `json:"max_rows,omitempty"`
	ExecutedAt string   `json:"executed_at"`
}

// QueryResultCodec handles SQL query result blocks for BI use cases.
type QueryResultCodec struct {
	*context.BaseCodec
	maxRows int
}

// QueryResultCodecOption configures the query result codec.
type QueryResultCodecOption func(*QueryResultCodec)

// WithMaxRows sets the maximum number of rows to include in the result.
func WithMaxRows(maxRows int) QueryResultCodecOption {
	return func(c *QueryResultCodec) {
		c.maxRows = maxRows
	}
}

// NewQueryResultCodec creates a new query result codec.
func NewQueryResultCodec(opts ...QueryResultCodecOption) *QueryResultCodec {
	c := &QueryResultCodec{
		BaseCodec: context.NewBaseCodec("query-result", "1.0.0"),
		maxRows:   100, // Default max rows
	}

	for _, opt := range opts {
		opt(c)
	}

	return c
}

// Validate validates the query result payload.
func (c *QueryResultCodec) Validate(payload any) error {
	switch v := payload.(type) {
	case QueryResultPayload:
		if v.Query == "" {
			return fmt.Errorf("query cannot be empty")
		}
		if len(v.Columns) == 0 {
			return fmt.Errorf("result must have at least one column")
		}
		return nil
	case map[string]any:
		if query, ok := v["query"].(string); !ok || query == "" {
			return fmt.Errorf("query cannot be empty")
		}
		if cols, ok := v["columns"].([]any); !ok || len(cols) == 0 {
			return fmt.Errorf("result must have at least one column")
		}
		return nil
	default:
		return fmt.Errorf("invalid query result payload type: %T", payload)
	}
}

// Canonicalize converts the payload to canonical form with truncation.
func (c *QueryResultCodec) Canonicalize(payload any) ([]byte, error) {
	var result QueryResultPayload

	switch v := payload.(type) {
	case QueryResultPayload:
		result = v
	case map[string]any:
		if query, ok := v["query"].(string); ok {
			result.Query = normalizeWhitespace(query)
		}
		if cols, ok := v["columns"].([]any); ok {
			for _, col := range cols {
				if colStr, ok := col.(string); ok {
					result.Columns = append(result.Columns, colStr)
				}
			}
		}
		if rows, ok := v["rows"].([]any); ok {
			for _, row := range rows {
				if rowArr, ok := row.([]any); ok {
					result.Rows = append(result.Rows, rowArr)
				}
			}
		}
		if rowCount, ok := v["row_count"].(int); ok {
			result.RowCount = rowCount
		}
		if truncated, ok := v["truncated"].(bool); ok {
			result.Truncated = truncated
		}
		if maxRows, ok := v["max_rows"].(int); ok {
			result.MaxRows = maxRows
		}
		if executedAt, ok := v["executed_at"].(string); ok {
			result.ExecutedAt = executedAt
		}
	default:
		return nil, fmt.Errorf("invalid query result payload type: %T", payload)
	}

	// Apply truncation if needed
	if len(result.Rows) > c.maxRows {
		result.Rows = result.Rows[:c.maxRows]
		result.Truncated = true
		result.MaxRows = c.maxRows
	}

	return context.SortedJSONBytes(result)
}
