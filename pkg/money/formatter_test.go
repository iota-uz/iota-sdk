package money

import (
	"math/big"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFormatter_Format(t *testing.T) {
	tcs := []struct {
		fraction int
		decimal  string
		thousand string
		grapheme string
		template string
		amount   int64
		expected string
	}{
		{2, ".", ",", "$", "1 $", 0, "0.00 $"},
		{2, ".", ",", "$", "1 $", 1, "0.01 $"},
		{2, ".", ",", "$", "1 $", 12, "0.12 $"},
		{2, ".", ",", "$", "1 $", 123, "1.23 $"},
		{2, ".", ",", "$", "1 $", 1234, "12.34 $"},
		{2, ".", ",", "$", "1 $", 12345, "123.45 $"},
		{2, ".", ",", "$", "1 $", 123456, "1,234.56 $"},
		{2, ".", ",", "$", "1 $", 1234567, "12,345.67 $"},
		{2, ".", ",", "$", "1 $", 12345678, "123,456.78 $"},
		{2, ".", ",", "$", "1 $", 123456789, "1,234,567.89 $"},

		{2, ".", ",", "$", "1 $", -1, "-0.01 $"},
		{2, ".", ",", "$", "1 $", -12, "-0.12 $"},
		{2, ".", ",", "$", "1 $", -123, "-1.23 $"},
		{2, ".", ",", "$", "1 $", -1234, "-12.34 $"},
		{2, ".", ",", "$", "1 $", -12345, "-123.45 $"},
		{2, ".", ",", "$", "1 $", -123456, "-1,234.56 $"},
		{2, ".", ",", "$", "1 $", -1234567, "-12,345.67 $"},
		{2, ".", ",", "$", "1 $", -12345678, "-123,456.78 $"},
		{2, ".", ",", "$", "1 $", -123456789, "-1,234,567.89 $"},

		{3, ".", "", "$", "1 $", 1, "0.001 $"},
		{3, ".", "", "$", "1 $", 12, "0.012 $"},
		{3, ".", "", "$", "1 $", 123, "0.123 $"},
		{3, ".", "", "$", "1 $", 1234, "1.234 $"},
		{3, ".", "", "$", "1 $", 12345, "12.345 $"},
		{3, ".", "", "$", "1 $", 123456, "123.456 $"},
		{3, ".", "", "$", "1 $", 1234567, "1234.567 $"},
		{3, ".", "", "$", "1 $", 12345678, "12345.678 $"},
		{3, ".", "", "$", "1 $", 123456789, "123456.789 $"},

		{2, ".", ",", "£", "$1", 1, "£0.01"},
		{2, ".", ",", "£", "$1", 12, "£0.12"},
		{2, ".", ",", "£", "$1", 123, "£1.23"},
		{2, ".", ",", "£", "$1", 1234, "£12.34"},
		{2, ".", ",", "£", "$1", 12345, "£123.45"},
		{2, ".", ",", "£", "$1", 123456, "£1,234.56"},
		{2, ".", ",", "£", "$1", 1234567, "£12,345.67"},
		{2, ".", ",", "£", "$1", 12345678, "£123,456.78"},
		{2, ".", ",", "£", "$1", 123456789, "£1,234,567.89"},

		{0, ".", ",", "NT$", "$1", 1, "NT$1"},
		{0, ".", ",", "NT$", "$1", 12, "NT$12"},
		{0, ".", ",", "NT$", "$1", 123, "NT$123"},
		{0, ".", ",", "NT$", "$1", 1234, "NT$1,234"},
		{0, ".", ",", "NT$", "$1", 12345, "NT$12,345"},
		{0, ".", ",", "NT$", "$1", 123456, "NT$123,456"},
		{0, ".", ",", "NT$", "$1", 1234567, "NT$1,234,567"},
		{0, ".", ",", "NT$", "$1", 12345678, "NT$12,345,678"},
		{0, ".", ",", "NT$", "$1", 123456789, "NT$123,456,789"},

		{0, ".", ",", "NT$", "$1", -1, "-NT$1"},
		{0, ".", ",", "NT$", "$1", -12, "-NT$12"},
		{0, ".", ",", "NT$", "$1", -123, "-NT$123"},
		{0, ".", ",", "NT$", "$1", -1234, "-NT$1,234"},
		{0, ".", ",", "NT$", "$1", -12345, "-NT$12,345"},
		{0, ".", ",", "NT$", "$1", -123456, "-NT$123,456"},
		{0, ".", ",", "NT$", "$1", -1234567, "-NT$1,234,567"},
		{0, ".", ",", "NT$", "$1", -12345678, "-NT$12,345,678"},
		{0, ".", ",", "NT$", "$1", -123456789, "-NT$123,456,789"},
	}

	for _, tc := range tcs {
		formatter := NewFormatter(tc.fraction, tc.decimal, tc.thousand, tc.grapheme, tc.template)
		r := formatter.Format(tc.amount)

		if r != tc.expected {
			t.Errorf("Expected %d formatted to be %s got %s", tc.amount, tc.expected, r)
		}
	}
}

func TestFormatter_FormatCompact(t *testing.T) {
	// Test with default decimals (1)
	tcs := []struct {
		fraction int
		decimal  string
		thousand string
		grapheme string
		template string
		amount   int64
		decimals int
		expected string
	}{
		// Small amounts (should use standard format)
		{2, ".", ",", "UZS", "1 $", 123, 1, "1.23 UZS"},
		{2, ".", ",", "UZS", "1 $", 999, 1, "9.99 UZS"},

		// Thousands
		{2, ".", ",", "UZS", "1 $", 100000, 1, "1K UZS"},
		{2, ".", ",", "UZS", "1 $", 123400, 1, "1.2K UZS"},
		{2, ".", ",", "UZS", "1 $", 999900, 1, "10K UZS"},

		// Millions
		{2, ".", ",", "UZS", "1 $", 1000000, 1, "10K UZS"},
		{2, ".", ",", "UZS", "1 $", 1230000, 1, "12.3K UZS"},
		{2, ".", ",", "UZS", "1 $", 10000000, 1, "100K UZS"},
		{2, ".", ",", "UZS", "1 $", 100000000, 1, "1M UZS"},
		{2, ".", ",", "UZS", "1 $", 1200000000, 1, "12M UZS"},
		{2, ".", ",", "UZS", "1 $", 1250000000, 1, "12.5M UZS"},
		{2, ".", ",", "UZS", "1 $", 2252423200, 1, "22.5M UZS"}, // Example from requirement

		// Billions
		{2, ".", ",", "UZS", "1 $", 100000000000, 1, "1B UZS"},
		{2, ".", ",", "UZS", "1 $", 123000000000, 1, "1.2B UZS"},

		// Different fraction
		{0, ".", ",", "UZS", "1 $", 1234, 1, "1.2K UZS"},
		{0, ".", ",", "UZS", "1 $", 1000000, 1, "1M UZS"},
		{0, ".", ",", "UZS", "1 $", 1200000, 1, "1.2M UZS"},

		// Negative values
		{2, ".", ",", "UZS", "1 $", -1234567, 1, "-12.3K UZS"},
		{2, ".", ",", "UZS", "1 $", -1000000000, 1, "-10M UZS"},
		{2, ".", ",", "UZS", "1 $", -1200000000, 1, "-12M UZS"},

		// Different currency symbols
		{2, ".", ",", "$", "1 $", 1234567, 1, "12.3K $"},
		{2, ".", ",", "€", "1 $", 1234567, 1, "12.3K €"},

		// Test with 2 decimal places
		{2, ".", ",", "UZS", "1 $", 123400, 2, "1.23K UZS"},
		{2, ".", ",", "UZS", "1 $", 1234567, 2, "12.35K UZS"},
		{2, ".", ",", "UZS", "1 $", 123456789, 2, "1.23M UZS"},
		{2, ".", ",", "UZS", "1 $", 2252423200, 2, "22.52M UZS"}, // Example from requirement with 2 decimals

		// Test with 0 decimal places (should default to 1)
		{2, ".", ",", "UZS", "1 $", 123400, 0, "1.2K UZS"},
		{2, ".", ",", "UZS", "1 $", 1234567, 0, "12.3K UZS"},

		// Test with 3 decimal places
		{2, ".", ",", "UZS", "1 $", 123400, 3, "1.234K UZS"},
		{2, ".", ",", "UZS", "1 $", 1234567, 3, "12.346K UZS"},
	}

	for _, tc := range tcs {
		formatter := NewFormatter(tc.fraction, tc.decimal, tc.thousand, tc.grapheme, tc.template)
		r := formatter.FormatCompact(tc.amount, tc.decimals)

		if r != tc.expected {
			t.Errorf("Expected %d compact formatted with %d decimals to be %s got %s", tc.amount, tc.decimals, tc.expected, r)
		}
	}
}

func TestFormatter_ToMajorUnits(t *testing.T) {
	tcs := []struct {
		fraction int
		decimal  string
		thousand string
		grapheme string
		template string
		amount   int64
		expected float64
	}{
		{2, ".", ",", "$", "1 $", 0, 0.00},
		{2, ".", ",", "$", "1 $", 1, 0.01},
		{2, ".", ",", "$", "1 $", 12, 0.12},
		{2, ".", ",", "$", "1 $", 123, 1.23},
		{2, ".", ",", "$", "1 $", 1234, 12.34},
		{2, ".", ",", "$", "1 $", 12345, 123.45},
		{2, ".", ",", "$", "1 $", 123456, 1234.56},
		{2, ".", ",", "$", "1 $", 1234567, 12345.67},
		{2, ".", ",", "$", "1 $", 12345678, 123456.78},
		{2, ".", ",", "$", "1 $", 123456789, 1234567.89},

		{2, ".", ",", "$", "1 $", -1, -0.01},
		{2, ".", ",", "$", "1 $", -12, -0.12},
		{2, ".", ",", "$", "1 $", -123, -1.23},
		{2, ".", ",", "$", "1 $", -1234, -12.34},
		{2, ".", ",", "$", "1 $", -12345, -123.45},
		{2, ".", ",", "$", "1 $", -123456, -1234.56},
		{2, ".", ",", "$", "1 $", -1234567, -12345.67},
		{2, ".", ",", "$", "1 $", -12345678, -123456.78},
		{2, ".", ",", "$", "1 $", -123456789, -1234567.89},

		{3, ".", "", "$", "1 $", 1, 0.001},
		{3, ".", "", "$", "1 $", 12, 0.012},
		{3, ".", "", "$", "1 $", 123, 0.123},
		{3, ".", "", "$", "1 $", 1234, 1.234},
		{3, ".", "", "$", "1 $", 12345, 12.345},
		{3, ".", "", "$", "1 $", 123456, 123.456},
		{3, ".", "", "$", "1 $", 1234567, 1234.567},
		{3, ".", "", "$", "1 $", 12345678, 12345.678},
		{3, ".", "", "$", "1 $", 123456789, 123456.789},

		{2, ".", ",", "£", "$1", 1, 0.01},
		{2, ".", ",", "£", "$1", 12, 0.12},
		{2, ".", ",", "£", "$1", 123, 1.23},
		{2, ".", ",", "£", "$1", 1234, 12.34},
		{2, ".", ",", "£", "$1", 12345, 123.45},
		{2, ".", ",", "£", "$1", 123456, 1234.56},
		{2, ".", ",", "£", "$1", 1234567, 12345.67},
		{2, ".", ",", "£", "$1", 12345678, 123456.78},
		{2, ".", ",", "£", "$1", 123456789, 1234567.89},

		{0, ".", ",", "NT$", "$1", 1, 1},
		{0, ".", ",", "NT$", "$1", 12, 12},
		{0, ".", ",", "NT$", "$1", 123, 123},
		{0, ".", ",", "NT$", "$1", 1234, 1234},
		{0, ".", ",", "NT$", "$1", 12345, 12345},
		{0, ".", ",", "NT$", "$1", 123456, 123456},
		{0, ".", ",", "NT$", "$1", 1234567, 1234567},
		{0, ".", ",", "NT$", "$1", 12345678, 12345678},
		{0, ".", ",", "NT$", "$1", 123456789, 123456789},

		{0, ".", ",", "NT$", "$1", -1, -1},
		{0, ".", ",", "NT$", "$1", -12, -12},
		{0, ".", ",", "NT$", "$1", -123, -123},
		{0, ".", ",", "NT$", "$1", -1234, -1234},
		{0, ".", ",", "NT$", "$1", -12345, -12345},
		{0, ".", ",", "NT$", "$1", -123456, -123456},
		{0, ".", ",", "NT$", "$1", -1234567, -1234567},
		{0, ".", ",", "NT$", "$1", -12345678, -12345678},
		{0, ".", ",", "NT$", "$1", -123456789, -123456789},
	}

	for _, tc := range tcs {
		formatter := NewFormatter(tc.fraction, tc.decimal, tc.thousand, tc.grapheme, tc.template)
		r := formatter.ToMajorUnits(tc.amount)

		if r != tc.expected {
			t.Errorf("Expected %d formatted to major units to be %f got %f", tc.amount, tc.expected, r)
		}
	}
}

func TestFormatBigInt(t *testing.T) {
	tests := []struct {
		name     string
		frac     int
		decimal  string
		thousand string
		grapheme string
		template string
		amount   *big.Int
		expected string
	}{
		{"thousands", 2, ".", ",", "$", "1 $", setBigInt("123456789"), "1,234,567.89 $"},
		{"negative", 2, ".", ",", "$", "1 $", setBigInt("-123456789"), "-1,234,567.89 $"},
		{"currency template", 2, ".", ",", "£", "$1", setBigInt("123456789"), "£1,234,567.89"},
		{"very large value", 2, ".", ",", "$", "1 $", setBigInt("99999999999999999999"), "999,999,999,999,999,999.99 $"},
		{"nil amount", 2, ".", ",", "$", "1 $", nil, "0.00 $"},
		{"zero fraction", 0, ".", ",", "NT$", "$1", setBigInt("1234567"), "NT$1,234,567"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			formatter := NewFormatter(tt.frac, tt.decimal, tt.thousand, tt.grapheme, tt.template)
			result := formatter.FormatBigInt(tt.amount)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFormatCompactBigInt(t *testing.T) {
	tests := []struct {
		name     string
		frac     int
		grapheme string
		amount   *big.Int
		decimals int
		expected string
	}{
		{"small fallback", 2, "UZS", big.NewInt(123), 1, "1.23 UZS"},
		{"thousands", 2, "UZS", big.NewInt(1234000), 1, "12.3K UZS"},
		{"millions", 2, "UZS", setBigInt("125000000"), 1, "1.2M UZS"},
		{"billions", 2, "UZS", setBigInt("1200000000000"), 1, "12B UZS"},
		{"negative", 2, "UZS", setBigInt("-100000000"), 1, "-1M UZS"},
		{"nil amount", 2, "UZS", nil, 1, "0.00 UZS"},
		{"huge value no Inf", 2, "UZS", setBigInt("100000000000000000000"), 1, "1000000000B UZS"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			formatter := NewFormatter(tt.frac, ".", ",", tt.grapheme, "1 $")
			result := formatter.FormatCompactBigInt(tt.amount, tt.decimals)
			assert.NotContains(t, result, "Inf", "result must not contain Inf")
			assert.NotContains(t, result, "NaN", "result must not contain NaN")
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestToMajorUnitsBigFloat(t *testing.T) {
	tests := []struct {
		name      string
		frac      int
		amount    *big.Int
		expectStr string
	}{
		{"with fraction", 2, setBigInt("12345"), "123.45"},
		{"no fraction", 0, setBigInt("12345"), "12345"},
		{"nil input", 2, nil, "0"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			formatter := NewFormatter(tt.frac, ".", ",", "$", "1 $")
			result := formatter.ToMajorUnitsBigFloat(tt.amount)
			assert.NotNil(t, result)
			expected, _, _ := new(big.Float).Parse(tt.expectStr, 10)
			diff := new(big.Float).Sub(result, expected)
			abs := new(big.Float).Abs(diff)
			tolerance := new(big.Float).SetFloat64(0.001)
			assert.LessOrEqual(t, abs.Cmp(tolerance), 0, "expected ~%s, got %s", tt.expectStr, result.String())
		})
	}
}

func TestAbsBigInt(t *testing.T) {
	formatter := NewFormatter(2, ".", ",", "$", "1 $")

	t.Run("negative to positive", func(t *testing.T) {
		result := formatter.absBigInt(big.NewInt(-42))
		assert.Equal(t, int64(42), result.Int64())
	})

	t.Run("nil returns zero", func(t *testing.T) {
		result := formatter.absBigInt(nil)
		assert.Equal(t, int64(0), result.Int64())
	})
}
