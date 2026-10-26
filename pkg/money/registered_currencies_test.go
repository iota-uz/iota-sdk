package money

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// False green if only one snapshot is checked: order must be stable across
// calls, and changing a returned entry must leave both the registry and next snapshot intact.
func TestRegisteredCurrenciesReturnsOrderedIndependentMetadata(t *testing.T) {
	t.Parallel()
	first := RegisteredCurrencies()
	require.NotEmpty(t, first)
	require.Equal(t, first, RegisteredCurrencies())
	fractions := map[string]int{JPY: 0, KWD: 3, USD: 2}
	seen := make(map[string]Currency, len(first))
	for i, currency := range first {
		if i > 0 {
			require.Less(t, first[i-1].Code, currency.Code)
		}
		require.Equal(t, *GetCurrency(currency.Code), currency)
		seen[currency.Code] = currency
	}
	for code, fraction := range fractions {
		currency, ok := seen[code]
		require.True(t, ok, "missing registered currency %s", code)
		require.Equal(t, fraction, currency.Fraction, code)
	}
	require.Contains(t, seen, USDSPACE, "formatting variants are part of the registered catalog")
	original := first[0]
	first[0] = Currency{Code: "changed", Fraction: 99}
	require.Equal(t, original, *GetCurrency(original.Code))
	require.Equal(t, original, RegisteredCurrencies()[0])
}
