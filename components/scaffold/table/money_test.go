package table

import (
	"bytes"
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/text/language"
)

func render(t *testing.T, amount MoneyAmount) string {
	t.Helper()
	var buf bytes.Buffer
	require.NoError(t, MoneyValue(amount).Render(context.Background(), &buf))
	return buf.String()
}

func TestMoneyFromMinor(t *testing.T) {
	ru := language.MustParse("ru")
	tests := []struct {
		name     string
		minor    int64
		currency string
		value    string
	}{
		{"whole", 500000, "UZS", "5000.00"},
		{"cents", 5, "USD", "0.05"},
		{"negative cents", -5, "USD", "-0.05"},
		{"negative whole", -123456, "UZS", "-1234.56"},
		{"zero", 0, "UZS", "0.00"},
		{"unknown currency defaults to two digits", 1999, "XXY", "19.99"},
		{"min int64 does not overflow", -9223372036854775808, "UZS", "-92233720368547758.08"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MoneyFromMinor(tt.minor, tt.currency, ru)
			assert.Equal(t, tt.value, got.Value)
			assert.Equal(t, tt.currency, got.Currency)
		})
	}
}

// Falsely green if Text is asserted: the locale's group separator is a
// no-break space, so only Value may be compared.
func TestMoneyFromMinorTextIsLocalizedAndValueIsNot(t *testing.T) {
	got := MoneyFromMinor(123456789, "UZS", language.MustParse("ru"))
	assert.Equal(t, "1234567.89", got.Value)
	assert.Contains(t, got.Text, " ")
}

func TestMoneyValueRendersExactValueAndCurrency(t *testing.T) {
	out := render(t, MoneyAmount{Value: "50.00", Text: "50,00", Currency: "UZS", TestID: "amount"})
	assert.Contains(t, out, `data-testid="amount"`)
	assert.Contains(t, out, `data-value="50.00"`)
	assert.Contains(t, out, ">UZS</small>")
	assert.Contains(t, out, "tabular-nums")
}

// Falsely green if an unknown currency is silently printed as a default:
// the note must name the gap and must not invent a code.
func TestMoneyValueStatesMissingCurrency(t *testing.T) {
	out := render(t, MoneyAmount{Value: "1.00", Text: "1,00", MissingCurrency: "currency unknown", MissingCurrencyTestID: "missing"})
	assert.Contains(t, out, `data-testid="missing"`)
	assert.Contains(t, out, "currency unknown")
	assert.NotContains(t, out, "UZS")
	bare := render(t, MoneyAmount{Value: "1.00", Text: "1,00"})
	assert.NotContains(t, bare, "<small")
}

func TestMoneyColumnAndCellAlignRight(t *testing.T) {
	col := MoneyColumn("amount", "Amount", WithSortable())
	assert.Contains(t, col.Class(), "text-right")
	assert.True(t, col.Sortable())
	cell := MoneyCell(MoneyAmount{Value: "1.00", Text: "1,00"})
	assert.Contains(t, cell.Classes().String(), "text-right")
}
