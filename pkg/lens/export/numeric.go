package export

import (
	"github.com/iota-uz/iota-sdk/pkg/excel"
	"github.com/iota-uz/iota-sdk/pkg/lens/frame"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/shopspring/decimal"
)

// Exact source decimals need validation before becoming native cells. Existing
// Lens float ratios keep their established approximate-number behavior.
func normalizeExactValue(value any) (any, error) {
	switch v := value.(type) {
	case decimal.Decimal, *decimal.Decimal, pgtype.Numeric, *pgtype.Numeric:
		return excel.NormalizeNumeric(v)
	case frame.Formula:
		result, err := normalizeExactValue(v.Result)
		v.Result = result
		return v, err
	case *frame.Formula:
		if v == nil {
			return nil, nil
		}
		copy := *v
		result, err := normalizeExactValue(copy.Result)
		copy.Result = result
		return copy, err
	default:
		return value, nil
	}
}

func exactValue(value any) bool {
	switch v := value.(type) {
	case decimal.Decimal, *decimal.Decimal, pgtype.Numeric, *pgtype.Numeric:
		return true
	case frame.Formula:
		return exactValue(v.Result)
	case *frame.Formula:
		return v != nil && exactValue(v.Result)
	default:
		return false
	}
}
