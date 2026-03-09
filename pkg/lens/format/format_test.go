package format

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestApplyParsesNumericStrings(t *testing.T) {
	t.Parallel()

	count := Count()
	money := MoneyCompact("UZS")
	percent := Percent(1)

	require.Equal(t, "42", Apply(&count, "42", "", ""))
	require.Equal(t, "12.50K UZS", Apply(&money, "12500", "", ""))
	require.Equal(t, "7.5%", Apply(&percent, "7.5", "", ""))
}
