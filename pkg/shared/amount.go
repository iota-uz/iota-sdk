package shared

import (
	"strings"

	"github.com/iota-uz/iota-sdk/pkg/money"
	"github.com/shopspring/decimal"
)

// Amount is a monetary amount as a person typed it into a form field.
//
// Decoding never fails: text that is not a number is kept in Text with Err set,
// so a form can show the error next to the field and keep what was typed.
// See money.ParseAmount for the accepted notations.
type Amount struct {
	text  string
	value decimal.Decimal
	err   error
}

// NewAmount returns the amount for an already known value.
func NewAmount(value decimal.Decimal) Amount {
	return Amount{text: value.String(), value: value}
}

// ParseAmount reads text typed by a person.
func ParseAmount(text string) Amount {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return Amount{}
	}
	value, err := money.ParseAmount(trimmed)
	return Amount{text: text, value: value, err: err}
}

// Text is the amount as it was typed.
func (a Amount) Text() string { return a.text }

// Decimal is the parsed value; zero when the field is empty or invalid.
func (a Amount) Decimal() decimal.Decimal { return a.value }

// Float64 is the parsed value as a float.
func (a Amount) Float64() float64 {
	f, _ := a.value.Float64()
	return f
}

// Err is non-nil when the field holds text that is not a number.
func (a Amount) Err() error { return a.err }

// IsEmpty reports whether nothing was typed.
func (a Amount) IsEmpty() bool { return strings.TrimSpace(a.text) == "" }

// IsPositive reports whether the field holds a valid amount above zero.
func (a Amount) IsPositive() bool { return a.err == nil && a.value.IsPositive() }

// IsNegative reports whether the field holds a valid amount below zero.
func (a Amount) IsNegative() bool { return a.err == nil && a.value.IsNegative() }
