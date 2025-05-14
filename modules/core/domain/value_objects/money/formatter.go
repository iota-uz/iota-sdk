package money

import (
	"math"
	"strconv"
	"strings"
)

// Formatter stores Money formatting information.
type Formatter struct {
	Fraction int
	Decimal  string
	Thousand string
	Grapheme string
	Template string
}

// NewFormatter creates new Formatter instance.
func NewFormatter(fraction int, decimal, thousand, grapheme, template string) *Formatter {
	return &Formatter{
		Fraction: fraction,
		Decimal:  decimal,
		Thousand: thousand,
		Grapheme: grapheme,
		Template: template,
	}
}

// Format returns string of formatted integer using given currency template.
func (f *Formatter) Format(amount int64) string {
	// Work with absolute amount value
	sa := strconv.FormatInt(f.abs(amount), 10)

	if len(sa) <= f.Fraction {
		sa = strings.Repeat("0", f.Fraction-len(sa)+1) + sa
	}

	if f.Thousand != "" {
		for i := len(sa) - f.Fraction - 3; i > 0; i -= 3 {
			sa = sa[:i] + f.Thousand + sa[i:]
		}
	}

	if f.Fraction > 0 {
		sa = sa[:len(sa)-f.Fraction] + f.Decimal + sa[len(sa)-f.Fraction:]
	}
	sa = strings.Replace(f.Template, "1", sa, 1)
	sa = strings.Replace(sa, "$", f.Grapheme, 1)

	// Add minus sign for negative amount.
	if amount < 0 {
		sa = "-" + sa
	}

	return sa
}

// FormatCompact returns a compactly formatted string for large monetary values.
// For example:
// - 1,234,567 -> 1.2M
// - 22,524,232 -> 22.5M
// - 1,234 -> 1.2K
func (f *Formatter) FormatCompact(amount int64) string {
	// Work with absolute amount value
	absAmount := f.abs(amount)
	majorUnits := f.ToMajorUnits(absAmount)

	var value float64
	var suffix string

	switch {
	case majorUnits >= 1_000_000_000: // Billion
		value = majorUnits / 1_000_000_000
		suffix = "B"
	case majorUnits >= 1_000_000: // Million
		value = majorUnits / 1_000_000
		suffix = "M"
	case majorUnits >= 10_000: // Ten Thousand (for 10K+)
		value = majorUnits / 1_000
		suffix = "K"
	case majorUnits >= 1_000: // Thousand
		value = majorUnits / 1_000
		suffix = "K"
	default:
		// For small values, use the standard format
		return f.Format(amount)
	}

	// Format to one decimal place
	formattedValue := strconv.FormatFloat(math.Floor(value*10)/10, 'f', 1, 64)
	// Remove trailing .0
	formattedValue = strings.TrimSuffix(formattedValue, ".0")

	// Construct the result
	result := formattedValue + suffix + " " + f.Grapheme

	// Add minus sign for negative amount
	if amount < 0 {
		result = "-" + result
	}

	return result
}

// ToMajorUnits returns float64 representing the value in sub units using the currency data
func (f *Formatter) ToMajorUnits(amount int64) float64 {
	if f.Fraction == 0 {
		return float64(amount)
	}

	return float64(amount) / float64(math.Pow10(f.Fraction))
}

// abs return absolute value of given integer.
func (f Formatter) abs(amount int64) int64 {
	if amount < 0 {
		return -amount
	}

	return amount
}
