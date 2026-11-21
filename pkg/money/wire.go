package money

import (
	"errors"
	"fmt"
)

// Wire is the payload form of a monetary value in a versioned business event
// (a native event, see pkg/extruntime/native): the amount in minor units as a
// decimal string, so it never travels as a float, and the ISO 4217 code. Use it
// as a field of an event payload struct; native.TypeOf derives the contract
// type from the tags.
type Wire struct {
	// Amount is the value in minor units (1999 is 19.99 for a currency with
	// two decimals). It is a decimal string on the wire.
	Amount int64 `json:"amount,string" wire:"int64"`
	// Currency is the ISO 4217 code.
	Currency string `json:"currency"`
}

// ErrWireAmount means the value does not fit the int64 amount of Wire.
var ErrWireAmount = errors.New("money: amount does not fit the wire int64")

// ToWire converts m to its payload form. A nil value or a value without a
// currency is an error: an event must not publish an amount nobody can read,
// and a value beyond int64 is refused rather than clamped.
func ToWire(m *Money) (Wire, error) {
	if m == nil {
		return Wire{}, errors.New("money: no value to put on the wire")
	}
	code := m.Currency().Code
	if code == "" {
		return Wire{}, errors.New("money: value has no currency")
	}
	if m.amount != nil && !m.amount.IsInt64() {
		return Wire{}, fmt.Errorf("%w: %s %s", ErrWireAmount, m.amount, code)
	}
	return Wire{Amount: m.Amount(), Currency: code}, nil
}

// Money returns the value of w.
func (w Wire) Money() *Money { return New(w.Amount, w.Currency) }
