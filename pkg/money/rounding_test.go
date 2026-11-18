package money

import (
	"encoding/json"
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewFromDecimal_RoundingModes(t *testing.T) {
	cases := []struct {
		amount string
		mode   RoundingMode
		want   int64
	}{
		{"0.125", RoundHalfUp, 13},
		{"-0.125", RoundHalfUp, -13},
		{"0.1249", RoundHalfUp, 12},
		{"0.125", RoundHalfEven, 12},
		{"0.135", RoundHalfEven, 14},
		{"-0.125", RoundHalfEven, -12},
		{"0.129", RoundDown, 12},
		{"-0.129", RoundDown, -12},
		{"0.121", RoundUp, 13},
		{"-0.121", RoundUp, -13},
		{"0.129", RoundFloor, 12},
		{"-0.121", RoundFloor, -13},
		{"0.121", RoundCeiling, 13},
		{"-0.129", RoundCeiling, -12},
		{"19.99", RoundDown, 1999},
	}
	for _, tc := range cases {
		t.Run(tc.amount+"/"+tc.mode.String(), func(t *testing.T) {
			got := NewFromDecimal(decimal.RequireFromString(tc.amount), UZS, tc.mode)
			assert.Equal(t, tc.want, got.Amount())
			assert.Equal(t, UZS, got.Currency().Code)
		})
	}
}

func TestNewFromDecimal_ComputedAmounts(t *testing.T) {
	// premium * rate: 1234.57 * 0.075 = 92.59275 → 92.59
	premium := decimal.RequireFromString("1234.57")
	got := NewFromDecimal(premium.Mul(decimal.RequireFromString("0.075")), UZS, DefaultRoundingMode)
	assert.Equal(t, int64(9259), got.Amount())

	// sum * pct / 100: 100.10 * 15 / 100 = 15.015 → 15.02
	sum := decimal.RequireFromString("100.10")
	got = NewFromDecimal(sum.Mul(decimal.NewFromInt(15)).Div(decimal.NewFromInt(100)), UZS, DefaultRoundingMode)
	assert.Equal(t, int64(1502), got.Amount())
}

func TestNewFromDecimal_CurrencyFraction(t *testing.T) {
	assert.Equal(t, int64(20), NewFromDecimal(decimal.RequireFromString("19.5"), JPY, RoundHalfUp).Amount())
	assert.Equal(t, int64(1235), NewFromDecimal(decimal.RequireFromString("1.2345"), KWD, RoundHalfUp).Amount())
	assert.Equal(t, int64(1234), NewFromDecimal(decimal.RequireFromString("1.2345"), KWD, RoundHalfEven).Amount())
}

func TestNewFromDecimal_BigValue(t *testing.T) {
	got := NewFromDecimal(decimal.RequireFromString("99999999999999999999.995"), UZS, RoundHalfUp)
	assert.Equal(t, "10000000000000000000000", got.BigAmount().String())
}

func TestNewFromDecimal_UnknownModePanics(t *testing.T) {
	assert.PanicsWithValue(t, "money: unknown rounding mode RoundingMode(99)", func() {
		NewFromDecimal(decimal.NewFromInt(1), UZS, RoundingMode(99))
	})
}

func TestNewFromFloat_UsesDefaultRoundingMode(t *testing.T) {
	assert.Equal(t, RoundHalfUp, DefaultRoundingMode)
	assert.Equal(t, RoundHalfUp, RoundingMode(0))
	assert.Equal(t, int64(13), NewFromFloat(0.125, UZS).Amount())
	assert.Equal(t, int64(12), NewFromFloatRounded(0.125, UZS, RoundDown).Amount())
	assert.Equal(t, int64(12), NewFromFloatRounded(0.125, UZS, RoundHalfEven).Amount())
	// 2.675 is 2.67499999… in binary; it is read as its shortest decimal form.
	assert.Equal(t, int64(268), NewFromFloat(2.675, USD).Amount())
}

func TestMoney_AsDecimal(t *testing.T) {
	assert.Equal(t, "19.99", New(1999, USD).AsDecimal().String())
	assert.Equal(t, "1999", New(1999, JPY).AsDecimal().String())
	assert.Equal(t, "0", (&Money{}).AsDecimal().String())

	m := NewFromDecimal(decimal.RequireFromString("-1.005"), KWD, RoundHalfUp)
	assert.True(t, m.AsDecimal().Equal(decimal.RequireFromString("-1.005")))
}

func TestMoney_RoundModes(t *testing.T) {
	cases := []struct {
		amount int64
		mode   RoundingMode
		want   int64
	}{
		{150, RoundHalfUp, 200},
		{-150, RoundHalfUp, -200},
		{149, RoundHalfUp, 100},
		{150, RoundHalfEven, 200},
		{250, RoundHalfEven, 200},
		{199, RoundDown, 100},
		{101, RoundUp, 200},
		{-101, RoundFloor, -200},
		{-199, RoundCeiling, -100},
	}
	for _, tc := range cases {
		assert.Equal(t, tc.want, New(tc.amount, EUR).RoundWith(tc.mode).Amount(), "%d %s", tc.amount, tc.mode)
	}
	assert.Equal(t, int64(200), New(150, EUR).Round().Amount())
	assert.Equal(t, int64(7), New(7, JPY).Round().Amount())
}

func TestMoney_ZeroValue(t *testing.T) {
	var z Money
	other := New(100, "")

	require.NotPanics(t, func() {
		assert.Empty(t, z.Currency().Code)
		assert.True(t, z.IsZero())
		assert.Equal(t, int64(0), z.Amount())
		assert.True(t, z.SameCurrency(&Money{}))
		assert.True(t, z.SameCurrency(other))
		assert.False(t, z.SameCurrency(New(0, USD)))

		eq, err := z.Equals(&Money{})
		require.NoError(t, err)
		assert.True(t, eq)

		less, err := z.LessThan(other)
		require.NoError(t, err)
		assert.True(t, less)

		cmp, err := z.Compare(&Money{})
		require.NoError(t, err)
		assert.Equal(t, 0, cmp)

		sum, err := z.Add(other, &Money{})
		require.NoError(t, err)
		assert.Equal(t, int64(100), sum.Amount())

		diff, err := z.Subtract(other)
		require.NoError(t, err)
		assert.Equal(t, int64(-100), diff.Amount())

		_, err = z.Add(New(1, USD))
		require.ErrorIs(t, err, ErrCurrencyMismatch)

		assert.Equal(t, int64(0), z.Multiply(3).Amount())
		assert.Equal(t, int64(0), z.Round().Amount())
		assert.Equal(t, int64(0), z.Absolute().Amount())
		assert.Equal(t, int64(0), z.Negative().Amount())

		parts, err := z.Split(3)
		require.NoError(t, err)
		assert.Len(t, parts, 3)

		parts, err = z.Allocate(1, 1)
		require.NoError(t, err)
		assert.Len(t, parts, 2)

		assert.NotEmpty(t, z.Display())
		assert.NotEmpty(t, z.DisplayCompact())
		assert.InDelta(t, 0, z.AsMajorUnits(), 0)
		assert.True(t, z.AsDecimal().IsZero())

		b, err := json.Marshal(z)
		require.NoError(t, err)
		assert.JSONEq(t, `{"amount":0,"currency":""}`, string(b))
	})
}

func TestMoney_MarshalJSON_EscapesCurrencyCode(t *testing.T) {
	b, err := json.Marshal(New(1, `X"Y`))
	require.NoError(t, err)

	var back Money
	require.NoError(t, json.Unmarshal(b, &back))
	assert.Equal(t, `X"Y`, back.Currency().Code)
	assert.Equal(t, int64(1), back.Amount())
}
