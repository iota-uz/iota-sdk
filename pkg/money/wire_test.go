package money_test

import (
	"encoding/json"
	"math/big"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/pkg/money"
)

func TestWire_AmountTravelsAsDecimalStringInMinorUnits(t *testing.T) {
	w, err := money.ToWire(money.New(1999, money.USD))
	require.NoError(t, err)
	raw, err := json.Marshal(w)
	require.NoError(t, err)
	require.JSONEq(t, `{"amount":"1999","currency":"USD"}`, string(raw))

	var back money.Wire
	require.NoError(t, json.Unmarshal(raw, &back))
	require.Equal(t, w, back)
	require.EqualValues(t, 1999, back.Money().Amount())
	require.Equal(t, money.USD, back.Money().Currency().Code)
}

func TestToWire_RefusesWhatCannotBeRead(t *testing.T) {
	_, err := money.ToWire(nil)
	require.Error(t, err)

	huge := money.NewFromBigInt(new(big.Int).Lsh(big.NewInt(1), 70), money.USD)
	_, err = money.ToWire(huge)
	require.ErrorIs(t, err, money.ErrWireAmount)

	_, err = money.ToWire(&money.Money{})
	require.Error(t, err, "the zero value has no currency")
}
