package money

import (
	"fmt"
	"math/big"

	"github.com/shopspring/decimal"
)

// RoundingMode says how a value that falls between two minor units is
// brought onto one of them.
//
// The zero value is RoundHalfUp, which is also DefaultRoundingMode.
type RoundingMode int

const (
	// RoundHalfUp rounds to the nearest minor unit; an exact half goes away
	// from zero (0.125 → 0.13, -0.125 → -0.13). This is commercial rounding
	// and the package default.
	RoundHalfUp RoundingMode = iota
	// RoundHalfEven rounds to the nearest minor unit; an exact half goes to
	// the even neighbour (0.125 → 0.12, 0.135 → 0.14). Banker's rounding.
	RoundHalfEven
	// RoundDown drops the digits beyond the currency's precision, that is
	// rounds toward zero (0.129 → 0.12, -0.129 → -0.12).
	RoundDown
	// RoundUp rounds away from zero whenever any digit is dropped
	// (0.121 → 0.13, -0.121 → -0.13).
	RoundUp
	// RoundFloor rounds toward negative infinity (0.129 → 0.12, -0.121 → -0.13).
	RoundFloor
	// RoundCeiling rounds toward positive infinity (0.121 → 0.13, -0.129 → -0.12).
	RoundCeiling
)

// DefaultRoundingMode is the mode NewFromFloat, Round and the other
// operations without an explicit mode use.
const DefaultRoundingMode = RoundHalfUp

// String returns the mode's name.
func (r RoundingMode) String() string {
	switch r {
	case RoundHalfUp:
		return "HalfUp"
	case RoundHalfEven:
		return "HalfEven"
	case RoundDown:
		return "Down"
	case RoundUp:
		return "Up"
	case RoundFloor:
		return "Floor"
	case RoundCeiling:
		return "Ceiling"
	default:
		return fmt.Sprintf("RoundingMode(%d)", int(r))
	}
}

// roundToInt rounds d to a whole number with mode. It panics on a mode that
// is not one of the declared constants: such a value can only come from a
// conversion of an arbitrary integer, which is a programming error.
func roundToInt(d decimal.Decimal, mode RoundingMode) *big.Int {
	var rounded decimal.Decimal
	switch mode {
	case RoundHalfUp:
		rounded = d.Round(0)
	case RoundHalfEven:
		rounded = d.RoundBank(0)
	case RoundDown:
		rounded = d.RoundDown(0)
	case RoundUp:
		rounded = d.RoundUp(0)
	case RoundFloor:
		rounded = d.RoundFloor(0)
	case RoundCeiling:
		rounded = d.RoundCeil(0)
	default:
		panic(fmt.Sprintf("money: unknown rounding mode %s", mode))
	}
	return rounded.BigInt()
}

// NewFromDecimal creates Money from an amount in major units (19.99 dollars,
// not 1999 cents). Digits beyond the currency's Fraction are rounded with
// mode, so computed amounts such as `premium * rate` or `sum * pct / 100`
// keep their nearest minor unit instead of losing it.
//
//	NewFromDecimal(decimal.RequireFromString("0.125"), "UZS", RoundHalfUp)   // 13 tiyin
//	NewFromDecimal(decimal.RequireFromString("0.125"), "UZS", RoundHalfEven) // 12 tiyin
//	NewFromDecimal(decimal.RequireFromString("0.129"), "UZS", RoundDown)     // 12 tiyin
func NewFromDecimal(amount decimal.Decimal, code string, mode RoundingMode) *Money {
	currency := newCurrency(code).get()
	return &Money{
		amount:   roundToInt(amount.Shift(int32(currency.Fraction)), mode),
		currency: currency,
	}
}

// NewFromFloatRounded creates Money from a float64 amount in major units,
// rounding digits beyond the currency's Fraction with mode.
//
// The float is first read as its shortest decimal representation, so 19.99
// is 19.99 and not 19.989999…, and 0.125 is exactly a half tiyin.
func NewFromFloatRounded(amount float64, code string, mode RoundingMode) *Money {
	return NewFromDecimal(decimal.NewFromFloat(amount), code, mode)
}

// AsDecimal returns the amount in major units as an exact decimal
// (1999 cents → 19.99).
func (m *Money) AsDecimal() decimal.Decimal {
	return decimal.NewFromBigInt(amountOrZero(m.amount), -int32(m.cur().Fraction))
}
