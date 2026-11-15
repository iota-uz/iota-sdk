package money

import (
	"errors"
	"strings"
	"unicode"

	"github.com/shopspring/decimal"
)

// ErrInvalidAmount is returned by ParseAmount for text that is not a number.
var ErrInvalidAmount = errors.New("invalid amount")

// ParseAmount reads an amount typed by a person.
//
// Spaces of any kind (including no-break and narrow no-break) and apostrophes
// group digits and are dropped. Both comma and dot are accepted as the decimal
// separator: when both appear, the last one is the decimal separator and the
// other groups digits; a lone separator that appears once is the decimal
// separator (so "1,5" and "1.5" are one and a half), and one that repeats
// groups digits ("1,234,567" is 1234567).
func ParseAmount(text string) (decimal.Decimal, error) {
	var b strings.Builder
	for _, r := range strings.TrimSpace(text) {
		switch {
		case unicode.IsSpace(r), r == '\'', r == '’', r == '_':
		case r == '−':
			b.WriteByte('-')
		default:
			b.WriteRune(r)
		}
	}
	s := b.String()
	if s == "" {
		return decimal.Zero, ErrInvalidAmount
	}

	sign := ""
	if s[0] == '-' || s[0] == '+' {
		if s[0] == '-' {
			sign = "-"
		}
		s = s[1:]
	}
	digits := 0
	for _, r := range s {
		switch {
		case r >= '0' && r <= '9':
			digits++
		case r != ',' && r != '.':
			return decimal.Zero, ErrInvalidAmount
		}
	}
	if digits == 0 {
		return decimal.Zero, ErrInvalidAmount
	}

	normalized, ok := normalizeSeparators(s)
	if !ok {
		return decimal.Zero, ErrInvalidAmount
	}
	value, err := decimal.NewFromString(sign + normalized)
	if err != nil {
		return decimal.Zero, ErrInvalidAmount
	}
	return value, nil
}

// normalizeSeparators returns digits with an optional single dot as the decimal
// separator.
func normalizeSeparators(s string) (string, bool) {
	lastComma, lastDot := strings.LastIndexByte(s, ','), strings.LastIndexByte(s, '.')
	decimalAt := -1
	switch {
	case lastComma >= 0 && lastDot >= 0:
		decimalChar, groupChar := ",", "."
		if lastDot > lastComma {
			decimalChar, groupChar = ".", ","
		}
		if strings.Count(s, decimalChar) > 1 {
			return "", false
		}
		s = strings.ReplaceAll(s, groupChar, "")
		decimalAt = strings.Index(s, decimalChar)
	case lastComma >= 0 && strings.Count(s, ",") == 1:
		decimalAt = lastComma
	case lastDot >= 0 && strings.Count(s, ".") == 1:
		decimalAt = lastDot
	}

	if decimalAt < 0 {
		return strings.NewReplacer(",", "", ".", "").Replace(s), true
	}
	whole := strings.NewReplacer(",", "", ".", "").Replace(s[:decimalAt])
	fraction := s[decimalAt+1:]
	if whole == "" {
		whole = "0"
	}
	if fraction == "" {
		return whole, true
	}
	return whole + "." + fraction, true
}
