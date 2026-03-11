package money

import (
	"math/big"
	"testing"
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

func TestFormatBigInt_Thousands(t *testing.T) {
	formatter := NewFormatter(2, ".", ",", "$", "1 $")
	bi := new(big.Int)
	bi.SetString("123456789", 10)
	result := formatter.FormatBigInt(bi)
	expected := "1,234,567.89 $"
	if result != expected {
		t.Errorf("Expected %s, got %s", expected, result)
	}
}

func TestFormatBigInt_Negative(t *testing.T) {
	formatter := NewFormatter(2, ".", ",", "$", "1 $")
	bi := new(big.Int)
	bi.SetString("-123456789", 10)
	result := formatter.FormatBigInt(bi)
	expected := "-1,234,567.89 $"
	if result != expected {
		t.Errorf("Expected %s, got %s", expected, result)
	}
}

func TestFormatBigInt_CurrencyTemplate(t *testing.T) {
	formatter := NewFormatter(2, ".", ",", "£", "$1")
	bi := new(big.Int)
	bi.SetString("123456789", 10)
	result := formatter.FormatBigInt(bi)
	expected := "£1,234,567.89"
	if result != expected {
		t.Errorf("Expected %s, got %s", expected, result)
	}
}

func TestFormatBigInt_VeryLargeValue(t *testing.T) {
	formatter := NewFormatter(2, ".", ",", "$", "1 $")
	bi := new(big.Int)
	bi.SetString("99999999999999999999", 10)
	result := formatter.FormatBigInt(bi)
	if result == "" {
		t.Error("Expected non-empty result for very large value")
	}
}

func TestFormatCompactBigInt(t *testing.T) {
	formatter := NewFormatter(2, ".", ",", "UZS", "1 $")
	bi := new(big.Int)
	bi.SetString("100000000000000000000", 10) // huge value
	result := formatter.FormatCompactBigInt(bi, 1)
	if result == "" {
		t.Error("Expected non-empty result for big compact format")
	}
}

func TestToMajorUnitsBigFloat(t *testing.T) {
	formatter := NewFormatter(2, ".", ",", "$", "1 $")
	bi := new(big.Int)
	bi.SetString("12345", 10)
	result := formatter.ToMajorUnitsBigFloat(bi)

	expected := new(big.Float).SetFloat64(123.45)
	// Compare with small tolerance
	diff := new(big.Float).Sub(result, expected)
	abs := new(big.Float).Abs(diff)
	tolerance := new(big.Float).SetFloat64(0.001)
	if abs.Cmp(tolerance) > 0 {
		t.Errorf("Expected ~123.45, got %s", result.String())
	}
}

func TestToMajorUnitsBigFloat_NoFraction(t *testing.T) {
	formatter := NewFormatter(0, ".", ",", "NT$", "$1")
	bi := new(big.Int)
	bi.SetString("12345", 10)
	result := formatter.ToMajorUnitsBigFloat(bi)

	expected := new(big.Float).SetFloat64(12345)
	if result.Cmp(expected) != 0 {
		t.Errorf("Expected 12345, got %s", result.String())
	}
}

func TestToMajorUnitsBigFloat_Nil(t *testing.T) {
	formatter := NewFormatter(2, ".", ",", "$", "1 $")
	result := formatter.ToMajorUnitsBigFloat(nil)
	if result == nil {
		t.Error("Expected non-nil result for nil input")
	}
}

func TestFormatBigInt_Nil(t *testing.T) {
	formatter := NewFormatter(2, ".", ",", "$", "1 $")
	result := formatter.FormatBigInt(nil)
	expected := "0.00 $"
	if result != expected {
		t.Errorf("Expected %s, got %s", expected, result)
	}
}

func TestFormatBigInt_ZeroFraction(t *testing.T) {
	formatter := NewFormatter(0, ".", ",", "NT$", "$1")
	bi := new(big.Int)
	bi.SetString("1234567", 10)
	result := formatter.FormatBigInt(bi)
	expected := "NT$1,234,567"
	if result != expected {
		t.Errorf("Expected %s, got %s", expected, result)
	}
}

func TestFormatCompactBigInt_Small(t *testing.T) {
	formatter := NewFormatter(2, ".", ",", "UZS", "1 $")
	bi := big.NewInt(123) // small value, should fall back to FormatBigInt
	result := formatter.FormatCompactBigInt(bi, 1)
	expected := "1.23 UZS"
	if result != expected {
		t.Errorf("Expected %s, got %s", expected, result)
	}
}

func TestFormatCompactBigInt_Billions(t *testing.T) {
	formatter := NewFormatter(2, ".", ",", "UZS", "1 $")
	bi := new(big.Int)
	bi.SetString("1200000000000", 10) // 12 billion major units with fraction=2
	result := formatter.FormatCompactBigInt(bi, 1)
	if result == "" {
		t.Error("Expected non-empty result for billions")
	}
}

func TestFormatCompactBigInt_Negative(t *testing.T) {
	formatter := NewFormatter(2, ".", ",", "UZS", "1 $")
	bi := new(big.Int)
	bi.SetString("-100000000", 10)
	result := formatter.FormatCompactBigInt(bi, 1)
	if result == "" {
		t.Error("Expected non-empty result for negative big compact")
	}
	if result[0] != '-' {
		t.Errorf("Expected negative prefix, got %s", result)
	}
}

func TestFormatCompactBigInt_Nil(t *testing.T) {
	formatter := NewFormatter(2, ".", ",", "UZS", "1 $")
	result := formatter.FormatCompactBigInt(nil, 1)
	// nil treated as 0, should use FormatBigInt fallback
	if result == "" {
		t.Error("Expected non-empty result for nil")
	}
}

func TestAbsBigInt(t *testing.T) {
	formatter := NewFormatter(2, ".", ",", "$", "1 $")
	result := formatter.absBigInt(big.NewInt(-42))
	if result.Int64() != 42 {
		t.Errorf("Expected 42, got %d", result.Int64())
	}

	result = formatter.absBigInt(nil)
	if result.Int64() != 0 {
		t.Errorf("Expected 0 for nil, got %d", result.Int64())
	}
}

func TestFormatCompactBigInt_Thousands(t *testing.T) {
	formatter := NewFormatter(2, ".", ",", "UZS", "1 $")
	bi := big.NewInt(1234000) // 12,340.00 major units -> 12.3K
	result := formatter.FormatCompactBigInt(bi, 1)
	expected := "12.3K UZS"
	if result != expected {
		t.Errorf("Expected %s, got %s", expected, result)
	}
}

func TestFormatCompactBigInt_Millions(t *testing.T) {
	formatter := NewFormatter(2, ".", ",", "UZS", "1 $")
	bi := new(big.Int)
	bi.SetString("125000000", 10) // 1,250,000.00 major units -> 1.2M
	result := formatter.FormatCompactBigInt(bi, 1)
	expected := "1.2M UZS"
	if result != expected {
		t.Errorf("Expected %s, got %s", expected, result)
	}
}
