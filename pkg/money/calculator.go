// Package money provides this package.
package money

import "math/big"

type calculator struct{}

func (c *calculator) add(a, b *big.Int) *big.Int {
	return new(big.Int).Add(a, b)
}

func (c *calculator) subtract(a, b *big.Int) *big.Int {
	return new(big.Int).Sub(a, b)
}

func (c *calculator) multiply(a *big.Int, m int64) *big.Int {
	return new(big.Int).Mul(a, big.NewInt(m))
}

func (c *calculator) divide(a *big.Int, d int64) *big.Int {
	return new(big.Int).Quo(a, big.NewInt(d))
}

func (c *calculator) modulus(a *big.Int, d int64) *big.Int {
	return new(big.Int).Rem(a, big.NewInt(d))
}

func (c *calculator) allocate(a *big.Int, r, s int64) *big.Int {
	if a.Sign() == 0 || s == 0 {
		return big.NewInt(0)
	}
	result := new(big.Int).Mul(a, big.NewInt(r))
	return result.Div(result, big.NewInt(s))
}

func (c *calculator) absolute(a *big.Int) *big.Int {
	return new(big.Int).Abs(a)
}

func (c *calculator) negative(a *big.Int) *big.Int {
	if a.Sign() > 0 {
		return new(big.Int).Neg(a)
	}
	return new(big.Int).Set(a)
}

func (c *calculator) round(a *big.Int, e int) *big.Int {
	if a.Sign() == 0 {
		return big.NewInt(0)
	}
	exp := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(e)), nil)
	half := new(big.Int).Div(exp, big.NewInt(2))
	absA := new(big.Int).Abs(a)
	mod := new(big.Int).Mod(absA, exp)
	if mod.Cmp(half) > 0 {
		absA.Add(absA, exp)
	}
	absA.Div(absA, exp).Mul(absA, exp)
	if a.Sign() < 0 {
		return absA.Neg(absA)
	}
	return absA
}
