package components_test

import (
	"bytes"
	"context"
	"testing"

	"github.com/iota-uz/iota-sdk/modules/core/presentation/templates/components"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMoneyInput_MasksWhileTyping(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	err := components.MoneyInput(&components.MoneyInputProps{Name: "Amount", Value: "1 500 000", Suffix: "UZS"}).Render(context.Background(), &out)
	require.NoError(t, err)

	html := out.String()
	assert.Contains(t, html, `inputmode="decimal"`)
	assert.Contains(t, html, `format($el)`)
	assert.Contains(t, html, `name="Amount"`)
	assert.Contains(t, html, `value="1 500 000"`)
}

func TestFormatAmountInput(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "1\u00a0500\u00a0000", components.FormatAmountInput(context.Background(), decimal.NewFromInt(1500000)))
	assert.Equal(t, "1\u00a0500\u00a0000.50", components.FormatAmountInput(context.Background(), decimal.RequireFromString("1500000.5")))
	assert.Empty(t, components.FormatAmountInput(context.Background(), decimal.Zero))
}
