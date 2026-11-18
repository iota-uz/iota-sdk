// Package money provides this package.
package money

import (
	"math/big"

	"github.com/shopspring/decimal"
)

// calculator does integer arithmetic on minor units. A nil operand is zero,
// which is what the zero value Money{} holds.
type calculator struct{}

func (c *calculator) add(a, b *big.Int) *big.Int {
	return new(big.Int).Add(amountOrZero(a), amountOrZero(b))
}

func (c *calculator) subtract(a, b *big.Int) *big.Int {
	return new(big.Int).Sub(amountOrZero(a), amountOrZero(b))
}

func (c *calculator) multiply(a *big.Int, m int64) *big.Int {
	return new(big.Int).Mul(amountOrZero(a), big.NewInt(m))
}

func (c *calculator) divide(a *big.Int, d int64) *big.Int {
	return new(big.Int).Quo(amountOrZero(a), big.NewInt(d))
}

func (c *calculator) modulus(a *big.Int, d int64) *big.Int {
	return new(big.Int).Rem(amountOrZero(a), big.NewInt(d))
}

func (c *calculator) allocate(a *big.Int, r, s int64) *big.Int {
	if a == nil || a.Sign() == 0 || s == 0 {
		return big.NewInt(0)
	}
	result := new(big.Int).Mul(a, big.NewInt(r))
	return result.Div(result, big.NewInt(s))
}

func (c *calculator) absolute(a *big.Int) *big.Int {
	return new(big.Int).Abs(amountOrZero(a))
}

func (c *calculator) negative(a *big.Int) *big.Int {
	a = amountOrZero(a)
	if a.Sign() > 0 {
		return new(big.Int).Neg(a)
	}
	return new(big.Int).Set(a)
}

// round rounds a (minor units) to a whole multiple of 10^e, the currency's
// major unit, with mode.
func (c *calculator) round(a *big.Int, e int, mode RoundingMode) *big.Int {
	if a == nil || a.Sign() == 0 {
		return big.NewInt(0)
	}
	units := roundToInt(decimal.NewFromBigInt(a, -int32(e)), mode)
	exp := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(e)), nil)
	return units.Mul(units, exp)
}
