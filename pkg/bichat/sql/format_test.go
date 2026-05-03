package sql_test

import (
	"math/big"
	"testing"

	bichatsql "github.com/iota-uz/iota-sdk/pkg/bichat/sql"
	"github.com/jackc/pgx/v5/pgtype"
)

func mustNumeric(mantissa string, exp int32) pgtype.Numeric {
	i := new(big.Int)
	if _, ok := i.SetString(mantissa, 10); !ok {
		panic("invalid mantissa: " + mantissa)
	}
	return pgtype.Numeric{Valid: true, Int: i, Exp: exp}
}

func TestFormatValue_PGNumeric(t *testing.T) {
	t.Parallel()

	var n pgtype.Numeric
	if err := n.Scan("160000"); err != nil {
		t.Fatalf("scan numeric: %v", err)
	}

	got := bichatsql.FormatValue(n)
	if got != "160000" {
		t.Fatalf("expected 160000, got %#v", got)
	}

	n.Valid = false
	if got := bichatsql.FormatValue(n); got != nil {
		t.Fatalf("expected nil for invalid numeric, got %#v", got)
	}
}

func TestNumericToString(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		n    pgtype.Numeric
		want string
	}{
		{name: "basic decimal", n: mustNumeric("123456", -2), want: "1234.56"},
		{name: "large SUM aggregate", n: mustNumeric("1016637120000000000", -12), want: "1016637.120000000000"},
		{name: "integer zero exp", n: mustNumeric("42", 0), want: "42"},
		{name: "positive exp", n: mustNumeric("5", 3), want: "5000"},
		{name: "leading zeros", n: mustNumeric("1", -5), want: "0.00001"},
		{name: "negative number", n: mustNumeric("-999", -1), want: "-99.9"},
		{name: "nil Int", n: pgtype.Numeric{Valid: true, Int: nil, Exp: 0}, want: "0"},
		{name: "NaN", n: pgtype.Numeric{Valid: true, NaN: true}, want: "NaN"},
		{name: "Infinity", n: pgtype.Numeric{Valid: true, InfinityModifier: pgtype.Infinity}, want: "Infinity"},
		{name: "NegativeInfinity", n: pgtype.Numeric{Valid: true, InfinityModifier: pgtype.NegativeInfinity}, want: "-Infinity"},
		{name: "Invalid", n: pgtype.Numeric{Valid: false}, want: "NULL"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := bichatsql.NumericToString(tt.n)
			if got != tt.want {
				t.Errorf("NumericToString() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestFormatNumeric(t *testing.T) {
	t.Parallel()

	t.Run("valid numeric via MarshalJSON", func(t *testing.T) {
		t.Parallel()
		var n pgtype.Numeric
		if err := n.Scan("160000"); err != nil {
			t.Fatalf("scan: %v", err)
		}
		got := bichatsql.FormatNumeric(n)
		if got != "160000" {
			t.Errorf("FormatNumeric() = %#v, want \"160000\"", got)
		}
	})

	t.Run("valid numeric fallback NumericToString", func(t *testing.T) {
		t.Parallel()
		n := mustNumeric("123456", -2)
		got := bichatsql.FormatNumeric(n)
		if got != "1234.56" {
			t.Errorf("FormatNumeric() = %#v, want \"1234.56\"", got)
		}
	})

	t.Run("invalid returns nil", func(t *testing.T) {
		t.Parallel()
		n := pgtype.Numeric{Valid: false}
		got := bichatsql.FormatNumeric(n)
		if got != nil {
			t.Errorf("FormatNumeric(invalid) = %#v, want nil", got)
		}
	})

	t.Run("integral float64 path", func(t *testing.T) {
		t.Parallel()
		var n pgtype.Numeric
		if err := n.Scan("42"); err != nil {
			t.Fatalf("scan: %v", err)
		}
		got := bichatsql.FormatNumeric(n)
		if got != "42" {
			t.Errorf("FormatNumeric() = %#v, want \"42\"", got)
		}
	})
}
