package excel

import (
	"math/big"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

// False green if decimals become numeric-looking text: require a native float,
// while nullable and unsafe exact values must remain blank or fail explicitly.
func TestNormalizeNumericUsesSharedWorkbookPrecision(t *testing.T) {
	t.Parallel()
	for _, value := range []any{decimal.RequireFromString("1.234"), pgtype.Numeric{Int: big.NewInt(1234), Exp: -3, Valid: true}} {
		got, err := NormalizeNumeric(value)
		require.NoError(t, err)
		number, ok := got.(float64)
		require.True(t, ok)
		require.InEpsilon(t, 1.234, number, 1e-15)
	}
	blank, err := NormalizeNumeric(pgtype.Numeric{})
	require.NoError(t, err)
	require.Nil(t, blank)
	for _, value := range []any{decimal.RequireFromString("1234567890123456.78"), pgtype.Numeric{Int: big.NewInt(9007199254740993), Valid: true}, "000123"} {
		_, err := NormalizeNumeric(value)
		require.Error(t, err)
	}
}
