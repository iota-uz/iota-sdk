package excel

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/shopspring/decimal"
)

// Cell applies an Excel number format without changing the underlying value's
// type. Strings remain text, even when they look like numbers or formulas.
type Cell struct {
	Value        any
	NumberFormat string
}

// Date keeps the value's calendar date in its own location. It does not convert
// time zones; the caller owns that policy. A zero date produces an empty cell.
func Date(value time.Time) Cell {
	if value.IsZero() {
		return Cell{}
	}
	return Cell{
		Value:        time.Date(value.Year(), value.Month(), value.Day(), 0, 0, 0, 0, time.UTC),
		NumberFormat: "yyyy-mm-dd",
	}
}

// PercentPoints converts a percentage-point value such as 20.2 to the numeric
// fraction 0.202 displayed by Excel as 20.20%. Decimal scaling avoids introducing
// binary-division artifacts before the workbook's precision validation.
func PercentPoints(value float64) Cell {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return Cell{Value: value, NumberFormat: "0.00%"}
	}
	decimalFraction := decimal.NewFromFloat(value).Shift(-2)
	return Cell{
		Value:        pgtype.Numeric{Int: decimalFraction.Coefficient(), Exp: decimalFraction.Exponent(), Valid: true},
		NumberFormat: "0.00%",
	}
}

// DateTime preserves the supplied wall clock with an explicit Excel format.
// A zero date-time produces an empty cell.
func DateTime(value time.Time) Cell {
	if value.IsZero() {
		return Cell{}
	}
	return Cell{Value: value, NumberFormat: "yyyy-mm-dd hh:mm:ss"}
}

func unwrapCell(value any) (any, string) {
	switch cell := value.(type) {
	case Cell:
		return cell.Value, cell.NumberFormat
	case *Cell:
		if cell == nil {
			return nil, ""
		}
		return cell.Value, cell.NumberFormat
	default:
		return value, ""
	}
}

func workbookValue(value any, opts *ExportOptions) (any, int, string, error) {
	value, format := unwrapCell(value)
	switch numeric := value.(type) {
	case pgtype.Numeric:
		return workbookNumeric(numeric, format, opts)
	case *pgtype.Numeric:
		if numeric == nil {
			return nil, 0, format, nil
		}
		return workbookNumeric(*numeric, format, opts)
	}
	value = convertPgxValue(value)
	switch v := value.(type) {
	case *time.Time:
		if v == nil {
			return nil, 0, format, nil
		}
		value = *v
	}
	switch v := value.(type) {
	case time.Time:
		if v.IsZero() {
			return nil, 0, format, nil
		}
		if v.Year() < 1900 || v.Year() > 9999 {
			return nil, 0, "", fmt.Errorf("date is outside Excel's supported calendar range")
		}
		if format == "" {
			if opts != nil {
				format = opts.DateTimeNumberFormat
				if format == "" && opts.DateTimeFormat != "" && opts.DateTimeFormat != "2006-01-02 15:04:05" {
					return nil, 0, "", fmt.Errorf("legacy DateTimeFormat requires an Excel DateTimeNumberFormat or Cell.NumberFormat")
				}
			}
			if format == "" {
				format = "yyyy-mm-dd hh:mm:ss"
			}
		}
		return v, 0, format, nil
	case float64:
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return nil, 0, "", fmt.Errorf("non-finite Excel number")
		}
		if !excelFloatPreserves(v, 64) {
			return nil, 0, "", fmt.Errorf("float exceeds Excel's 15 significant digit precision")
		}
	case float32:
		if math.IsNaN(float64(v)) || math.IsInf(float64(v), 0) {
			return nil, 0, "", fmt.Errorf("non-finite Excel number")
		}
		if !excelFloatPreserves(float64(v), 32) {
			return nil, 0, "", fmt.Errorf("float exceeds Excel's 15 significant digit precision")
		}
	}
	switch value.(type) {
	case float64, float32:
		if format == "" && opts != nil {
			format = opts.FloatNumberFormat
		}
		return value, 2, format, nil
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		integer, err := decimal.NewFromString(fmt.Sprint(value))
		if err != nil || !excelNumberPreserves(integer) {
			return nil, 0, "", fmt.Errorf("integer exceeds Excel's 15 significant digit precision")
		}
		return value, 1, format, nil
	default:
		return value, 0, format, nil
	}
}

func workbookNumeric(value pgtype.Numeric, format string, opts *ExportOptions) (any, int, string, error) {
	if !value.Valid {
		return nil, 0, format, nil
	}
	raw, err := value.Value()
	if err != nil {
		return nil, 0, "", fmt.Errorf("invalid Excel decimal: %w", err)
	}
	text, ok := raw.(string)
	if !ok {
		return nil, 0, "", fmt.Errorf("invalid Excel decimal")
	}
	number, err := decimal.NewFromString(text)
	if err != nil {
		return nil, 0, "", fmt.Errorf("invalid Excel decimal: %w", err)
	}
	if !excelNumberPreserves(number) {
		return nil, 0, "", fmt.Errorf("decimal exceeds Excel's 15 significant digit precision")
	}
	float, _ := number.Float64()
	if format == "" && opts != nil {
		format = opts.FloatNumberFormat
	}
	if format == "" {
		scale := int(-value.Exp)
		if scale > 0 && scale <= 15 {
			format = "0." + strings.Repeat("0", scale)
		} else {
			format = "0"
		}
	}
	return float, 0, format, nil
}

func excelNumberPreserves(number decimal.Decimal) bool {
	value, _ := number.Float64()
	restored, err := decimal.NewFromString(strconv.FormatFloat(value, 'g', 15, 64))
	return err == nil && restored.Equal(number)
}

func excelFloatPreserves(value float64, bits int) bool {
	shortest, err := decimal.NewFromString(strconv.FormatFloat(value, 'g', -1, bits))
	return err == nil && excelNumberPreserves(shortest)
}
