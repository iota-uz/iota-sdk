// Package money provides this package.
package money

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"math/big"

	"github.com/iota-uz/iota-sdk/pkg/serrors"
	"github.com/shopspring/decimal"
)

// Injection points for backward compatibility.
// If you need to keep your JSON marshal/unmarshal way, overwrite them like below.
//
//	money.UnmarshalJSON = func (m *Money, b []byte) error { ... }
//	money.MarshalJSON = func (m Money) ([]byte, error) { ... }
var (
	// UnmarshalJSON is injection point of json.Unmarshaller for money.Money
	UnmarshalJSON = defaultUnmarshalJSON
	// MarshalJSON is injection point of json.Marshaller for money.Money
	MarshalJSON = defaultMarshalJSON

	// ErrCurrencyMismatch happens when two compared Money don't have the same currency.
	ErrCurrencyMismatch = errors.New("currencies don't match")

	// ErrInvalidJSONUnmarshal happens when the default money.UnmarshalJSON fails to unmarshal Money because of invalid data.
	ErrInvalidJSONUnmarshal = errors.New("invalid json unmarshal")
)

// maxJSONAmountExponent bounds the decimal exponent accepted for a JSON
// amount such as 1e30, so a hostile 1e999999999 cannot make the decoder
// build an enormous integer.
const maxJSONAmountExponent = 100

// parseJSONAmount reads a JSON amount in minor units. The amount must be a
// whole number: 123, 123.0 and 1.23e2 are 123 minor units, while 123.7 is
// rejected rather than silently truncated, since a fraction of a minor unit
// means the sender used another unit.
func parseJSONAmount(n json.Number) (*big.Int, error) {
	if amount, ok := new(big.Int).SetString(n.String(), 10); ok {
		return amount, nil
	}
	d, err := decimal.NewFromString(n.String())
	if err != nil || !d.IsInteger() || d.Exponent() > maxJSONAmountExponent {
		return nil, ErrInvalidJSONUnmarshal
	}
	return d.BigInt(), nil
}

const opUnmarshalJSON = serrors.Op("money.defaultUnmarshalJSON")

func defaultUnmarshalJSON(m *Money, b []byte) error {
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.UseNumber()
	data := make(map[string]interface{})
	if err := dec.Decode(&data); err != nil {
		return serrors.Wrap(opUnmarshalJSON, err)
	}

	var amount *big.Int
	if amountRaw, ok := data["amount"]; ok {
		v, ok := amountRaw.(json.Number)
		if !ok {
			return serrors.Wrap(opUnmarshalJSON, ErrInvalidJSONUnmarshal)
		}
		parsed, err := parseJSONAmount(v)
		if err != nil {
			return serrors.Wrap(opUnmarshalJSON, err)
		}
		amount = parsed
	}

	var currency string
	if currencyRaw, ok := data["currency"]; ok {
		switch v := currencyRaw.(type) {
		case string:
			currency = v
		default:
			return serrors.Wrap(opUnmarshalJSON, ErrInvalidJSONUnmarshal)
		}
	}

	if amount == nil && currency == "" {
		*m = Money{}
		return nil
	}

	if amount == nil {
		amount = big.NewInt(0)
	}

	if amount.Sign() == 0 && currency == "" {
		*m = Money{}
		return nil
	}

	*m = Money{amount: amount, currency: newCurrency(currency).get()}
	return nil
}

func defaultMarshalJSON(m Money) ([]byte, error) {
	code, err := json.Marshal(m.Currency().Code)
	if err != nil {
		return nil, err
	}
	return []byte(fmt.Sprintf(`{"amount":%s,"currency":%s}`, amountOrZero(m.amount).String(), code)), nil
}

// Money represents monetary value information, stores
// currency and amount value.
type Money struct {
	amount   *big.Int  `db:"amount"`
	currency *Currency `db:"currency"`
}

// New creates and returns new instance of Money.
func New(amount int64, code string) *Money {
	return &Money{
		amount:   big.NewInt(amount),
		currency: newCurrency(code).get(),
	}
}

// NewFromFloat creates and returns new instance of Money from a float64
// amount in major units.
//
// The float is read as its shortest decimal representation (19.99 is 1999
// cents), and digits beyond the currency's Fraction are rounded with
// DefaultRoundingMode, half away from zero: 0.125 UZS is 13 tiyin and
// -0.125 UZS is -13 tiyin. Use NewFromFloatRounded or NewFromDecimal to pick
// another mode, for example RoundDown to truncate.
func NewFromFloat(amount float64, code string) *Money {
	return NewFromFloatRounded(amount, code, DefaultRoundingMode)
}

// NewFromBigInt creates and returns new instance of Money from a *big.Int (minor units).
func NewFromBigInt(amount *big.Int, code string) *Money {
	a := big.NewInt(0)
	if amount != nil {
		a = new(big.Int).Set(amount)
	}
	return &Money{
		amount:   a,
		currency: newCurrency(code).get(),
	}
}

// amountOrZero returns the given big.Int, or zero if it is nil.
func amountOrZero(a *big.Int) *big.Int {
	if a == nil {
		return big.NewInt(0)
	}
	return a
}

// zeroCurrency is the currency of the zero value Money{}: an empty code with
// the default formatting.
var zeroCurrency = newCurrency("").get()

// cur returns the currency of m, or the empty-code currency for the zero
// value Money{}, so operations on Money{} do not dereference nil.
func (m *Money) cur() *Currency {
	if m.currency == nil {
		return zeroCurrency
	}
	return m.currency
}

// Currency returns the currency used by Money. For the zero value Money{} it
// is a currency with an empty Code.
func (m *Money) Currency() *Currency {
	return m.cur()
}

// Amount returns a copy of the internal monetary value as an int64.
// If the value overflows int64, it returns math.MaxInt64 or math.MinInt64.
func (m *Money) Amount() int64 {
	if m.amount == nil {
		return 0
	}
	if m.amount.IsInt64() {
		return m.amount.Int64()
	}
	if m.amount.Sign() > 0 {
		return math.MaxInt64
	}
	return math.MinInt64
}

// BigAmount returns a copy of the internal value as *big.Int.
func (m *Money) BigAmount() *big.Int {
	if m.amount == nil {
		return big.NewInt(0)
	}
	return new(big.Int).Set(m.amount)
}

// SameCurrency check if given Money is equals by currency.
func (m *Money) SameCurrency(om *Money) bool {
	return m.cur().equals(om.cur())
}

func (m *Money) assertSameCurrency(om *Money) error {
	if !m.SameCurrency(om) {
		return ErrCurrencyMismatch
	}

	return nil
}

func (m *Money) compare(om *Money) int {
	return amountOrZero(m.amount).Cmp(amountOrZero(om.amount))
}

// Equals checks equality between two Money types.
func (m *Money) Equals(om *Money) (bool, error) {
	if err := m.assertSameCurrency(om); err != nil {
		return false, err
	}

	return m.compare(om) == 0, nil
}

// GreaterThan checks whether the value of Money is greater than the other.
func (m *Money) GreaterThan(om *Money) (bool, error) {
	if err := m.assertSameCurrency(om); err != nil {
		return false, err
	}

	return m.compare(om) == 1, nil
}

// GreaterThanOrEqual checks whether the value of Money is greater or equal than the other.
func (m *Money) GreaterThanOrEqual(om *Money) (bool, error) {
	if err := m.assertSameCurrency(om); err != nil {
		return false, err
	}

	return m.compare(om) >= 0, nil
}

// LessThan checks whether the value of Money is less than the other.
func (m *Money) LessThan(om *Money) (bool, error) {
	if err := m.assertSameCurrency(om); err != nil {
		return false, err
	}

	return m.compare(om) == -1, nil
}

// LessThanOrEqual checks whether the value of Money is less or equal than the other.
func (m *Money) LessThanOrEqual(om *Money) (bool, error) {
	if err := m.assertSameCurrency(om); err != nil {
		return false, err
	}

	return m.compare(om) <= 0, nil
}

// IsZero returns boolean of whether the value of Money is equals to zero.
func (m *Money) IsZero() bool {
	return m.amount == nil || m.amount.Sign() == 0
}

// IsPositive returns boolean of whether the value of Money is positive.
func (m *Money) IsPositive() bool {
	return m.amount != nil && m.amount.Sign() > 0
}

// IsNegative returns boolean of whether the value of Money is negative.
func (m *Money) IsNegative() bool {
	return m.amount != nil && m.amount.Sign() < 0
}

// Absolute returns new Money struct from given Money using absolute monetary value.
func (m *Money) Absolute() *Money {
	return &Money{amount: mutate.calc.absolute(m.amount), currency: m.cur()}
}

// Negative returns new Money struct from given Money using negative monetary value.
func (m *Money) Negative() *Money {
	return &Money{amount: mutate.calc.negative(m.amount), currency: m.cur()}
}

// Add returns new Money struct with value representing sum of Self and Other Money.
func (m *Money) Add(ms ...*Money) (*Money, error) {
	if len(ms) == 0 {
		return m, nil
	}

	k := New(0, m.cur().Code)

	for _, m2 := range ms {
		if err := m.assertSameCurrency(m2); err != nil {
			return nil, err
		}

		k.amount = mutate.calc.add(k.amount, m2.amount)
	}

	return &Money{amount: mutate.calc.add(m.amount, k.amount), currency: m.cur()}, nil
}

// Subtract returns new Money struct with value representing difference of Self and Other Money.
func (m *Money) Subtract(ms ...*Money) (*Money, error) {
	if len(ms) == 0 {
		return m, nil
	}

	k := New(0, m.cur().Code)

	for _, m2 := range ms {
		if err := m.assertSameCurrency(m2); err != nil {
			return nil, err
		}

		k.amount = mutate.calc.add(k.amount, m2.amount)
	}

	return &Money{amount: mutate.calc.subtract(m.amount, k.amount), currency: m.cur()}, nil
}

// Multiply returns new Money struct with value representing Self multiplied value by multiplier.
func (m *Money) Multiply(muls ...int64) *Money {
	if len(muls) == 0 {
		panic("At least one multiplier is required to multiply")
	}

	result := new(big.Int).Set(amountOrZero(m.amount))
	for _, mul := range muls {
		result.Mul(result, big.NewInt(mul))
	}

	return &Money{amount: result, currency: m.cur()}
}

// Round returns new Money with the value rounded to whole major units
// (1250 cents → 1300 cents) with DefaultRoundingMode, half away from zero.
func (m *Money) Round() *Money {
	return m.RoundWith(DefaultRoundingMode)
}

// RoundWith returns new Money with the value rounded to whole major units
// with mode.
func (m *Money) RoundWith(mode RoundingMode) *Money {
	c := m.cur()
	return &Money{amount: mutate.calc.round(amountOrZero(m.amount), c.Fraction, mode), currency: c}
}

// Split returns slice of Money structs with split Self value in given number.
// After division leftover pennies will be distributed round-robin amongst the parties.
// This means that parties listed first will likely receive more pennies than ones that are listed later.
func (m *Money) Split(n int) ([]*Money, error) {
	if n <= 0 {
		return nil, errors.New("split must be higher than zero")
	}

	amt := amountOrZero(m.amount)
	a := mutate.calc.divide(amt, int64(n))
	ms := make([]*Money, n)

	for i := 0; i < n; i++ {
		ms[i] = &Money{amount: new(big.Int).Set(a), currency: m.cur()}
	}

	r := mutate.calc.modulus(amt, int64(n))
	l := new(big.Int).Abs(r)
	// Add leftovers to the first parties.

	v := big.NewInt(1)
	if amt.Sign() < 0 {
		v = big.NewInt(-1)
	}
	for p := 0; l.Sign() != 0; p++ {
		ms[p].amount = mutate.calc.add(ms[p].amount, v)
		l.Sub(l, big.NewInt(1))
	}

	return ms, nil
}

// Allocate returns slice of Money structs with split Self value in given ratios.
// It lets split money by given ratios without losing pennies and as Split operations distributes
// leftover pennies amongst the parties with round-robin principle.
func (m *Money) Allocate(rs ...int) ([]*Money, error) {
	if len(rs) == 0 {
		return nil, errors.New("no ratios specified")
	}

	// Calculate sum of ratios.
	var sum int64
	for _, r := range rs {
		if r < 0 {
			return nil, errors.New("negative ratios not allowed")
		}
		if int64(r) > (math.MaxInt64 - sum) {
			return nil, errors.New("sum of given ratios exceeds max int")
		}
		sum += int64(r)
	}

	amt := amountOrZero(m.amount)
	total := big.NewInt(0)
	ms := make([]*Money, 0, len(rs))
	for _, r := range rs {
		party := &Money{
			amount:   mutate.calc.allocate(amt, int64(r), sum),
			currency: m.cur(),
		}

		ms = append(ms, party)
		total.Add(total, party.amount)
	}

	// if the sum of all ratios is zero, then we just returns zeros and don't do anything
	// with the leftover
	if sum == 0 {
		return ms, nil
	}

	// Calculate leftover value and divide to first parties.
	lo := new(big.Int).Sub(amt, total)
	sub := big.NewInt(1)
	if lo.Sign() < 0 {
		sub = big.NewInt(-1)
	}

	for p := 0; lo.Sign() != 0; p++ {
		ms[p].amount = mutate.calc.add(ms[p].amount, sub)
		lo.Sub(lo, sub)
	}

	return ms, nil
}

// Display lets represent Money struct as string in given Currency value.
func (m *Money) Display() string {
	c := m.cur().get()
	if m.amount != nil && !m.amount.IsInt64() {
		return c.Formatter().FormatBigInt(m.amount)
	}
	return c.Formatter().Format(m.Amount())
}

// DisplayCompact lets represent Money struct as a compact string for large values
// with the specified number of decimal places (e.g., 22.5M UZS with decimals=1, 22.52M UZS with decimals=2).
// If decimals is not specified (0), defaults to 1 decimal place.
func (m *Money) DisplayCompact(decimals ...int) string {
	c := m.cur().get()
	d := 1 // Default is 1 decimal place
	if len(decimals) > 0 && decimals[0] > 0 {
		d = decimals[0]
	}
	if m.amount != nil && !m.amount.IsInt64() {
		return c.Formatter().FormatCompactBigInt(m.amount, d)
	}
	return c.Formatter().FormatCompact(m.Amount(), d)
}

// AsMajorUnits lets represent Money struct as subunits (float64) in given Currency value
func (m *Money) AsMajorUnits() float64 {
	if m.amount == nil {
		return 0
	}
	c := m.cur().get()
	v, _ := c.Formatter().ToMajorUnitsBigFloat(m.amount).Float64()
	return v
}

// UnmarshalJSON is implementation of json.Unmarshaller
func (m *Money) UnmarshalJSON(b []byte) error {
	return UnmarshalJSON(m, b)
}

// MarshalJSON is implementation of json.Marshaller
func (m Money) MarshalJSON() ([]byte, error) {
	return MarshalJSON(m)
}

// Compare function compares two money of the same type
//
//	if m.amount > om.amount returns (1, nil)
//	if m.amount == om.amount returns (0, nil
//	if m.amount < om.amount returns (-1, nil)
//
// If compare moneys from distinct currency, return (m.amount, ErrCurrencyMismatch)
func (m *Money) Compare(om *Money) (int, error) {
	if err := m.assertSameCurrency(om); err != nil {
		return 0, err
	}

	return m.compare(om), nil
}
