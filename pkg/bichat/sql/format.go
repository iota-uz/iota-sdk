package sql

import (
	stdlibsql "database/sql"
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

// FormatValue normalizes a database value for JSON serialization. It handles
// pgx native types, database/sql Null* wrappers, and pgtype.Numeric (with
// exponent normalization) so the LLM consumer sees consistent shapes:
//
//   - time.Time → RFC3339 string
//   - []byte    → string
//   - [16]byte  → uuid string
//   - Null* (Valid=false) → nil
//   - pgtype.Numeric → decimal string preserving precision
//
// Returns the original value unchanged for unknown types.
func FormatValue(value any) any {
	if value == nil {
		return nil
	}

	switch v := value.(type) {
	case time.Time:
		return v.Format(time.RFC3339)
	case []byte:
		return string(v)
	case [16]byte:
		return uuid.UUID(v).String()
	case stdlibsql.NullString:
		if v.Valid {
			return v.String
		}
		return nil
	case stdlibsql.NullInt64:
		if v.Valid {
			return v.Int64
		}
		return nil
	case stdlibsql.NullFloat64:
		if v.Valid {
			return v.Float64
		}
		return nil
	case stdlibsql.NullBool:
		if v.Valid {
			return v.Bool
		}
		return nil
	case stdlibsql.NullTime:
		if v.Valid {
			return v.Time.Format(time.RFC3339)
		}
		return nil
	case json.Number:
		return v.String()
	case pgtype.Numeric:
		return FormatNumeric(v)
	case *pgtype.Numeric:
		if v == nil {
			return nil
		}
		return FormatNumeric(*v)
	default:
		return v
	}
}

// FormatNumeric converts pgtype.Numeric to its canonical JSON representation,
// preferring exact decimal strings over float64 to avoid precision loss.
func FormatNumeric(v pgtype.Numeric) any {
	if !v.Valid {
		return nil
	}

	raw, err := v.MarshalJSON()
	if err != nil {
		return NumericToString(v)
	}

	if string(raw) == "null" {
		return nil
	}

	var out any
	if err := json.Unmarshal(raw, &out); err == nil {
		if value, ok := out.(float64); ok {
			if value == float64(int64(value)) {
				return strconv.FormatInt(int64(value), 10)
			}
		}
	}

	return strings.Trim(string(raw), "\"")
}

// NumericToString fallback: builds the decimal string directly from
// (Int, Exp) so we still produce sensible output when MarshalJSON fails.
func NumericToString(v pgtype.Numeric) string {
	if !v.Valid {
		return "NULL"
	}

	if v.NaN {
		return "NaN"
	}
	if v.InfinityModifier == pgtype.Infinity {
		return "Infinity"
	}
	if v.InfinityModifier == pgtype.NegativeInfinity {
		return "-Infinity"
	}

	if v.Int == nil {
		return "0"
	}

	intStr := v.Int.String()

	if v.Exp == 0 {
		return intStr
	}

	if v.Exp > 0 {
		return intStr + strings.Repeat("0", int(v.Exp))
	}

	absExp := int(-v.Exp)

	negative := false
	if len(intStr) > 0 && intStr[0] == '-' {
		negative = true
		intStr = intStr[1:]
	}

	if absExp >= len(intStr) {
		zeros := strings.Repeat("0", absExp-len(intStr))
		result := "0." + zeros + intStr
		if negative {
			return "-" + result
		}
		return result
	}

	decimalPos := len(intStr) - absExp
	result := intStr[:decimalPos] + "." + intStr[decimalPos:]
	if negative {
		return "-" + result
	}
	return result
}
