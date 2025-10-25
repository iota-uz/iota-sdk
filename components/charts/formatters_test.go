package charts_test

import (
	"testing"

	"github.com/a-h/templ"
	"github.com/iota-uz/iota-sdk/components/charts"
	"github.com/stretchr/testify/assert"
)

func TestAbbreviatedCurrency(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		locale   string
		currency string
		expected string
	}{
		{
			name:     "Russian locale with UZS",
			locale:   "ru-RU",
			currency: "UZS",
			expected: "function(value) { return window.formatCurrency(value, 'ru-RU') + ' UZS'; }",
		},
		{
			name:     "US locale with USD",
			locale:   "en-US",
			currency: "USD",
			expected: "function(value) { return window.formatCurrency(value, 'en-US') + ' USD'; }",
		},
		{
			name:     "Uzbek locale with UZS",
			locale:   "uz-UZ",
			currency: "UZS",
			expected: "function(value) { return window.formatCurrency(value, 'uz-UZ') + ' UZS'; }",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := charts.AbbreviatedCurrency(tt.locale, tt.currency)
			assert.Equal(t, tt.expected, string(result))
		})
	}
}

func TestFullCurrency(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		locale   string
		currency string
		expected string
	}{
		{
			name:     "Russian locale with UZS",
			locale:   "ru-RU",
			currency: "UZS",
			expected: "function(value) { return Math.round(value).toLocaleString('ru-RU') + ' UZS'; }",
		},
		{
			name:     "US locale with USD",
			locale:   "en-US",
			currency: "USD",
			expected: "function(value) { return Math.round(value).toLocaleString('en-US') + ' USD'; }",
		},
		{
			name:     "Uzbek locale with EUR",
			locale:   "uz-UZ",
			currency: "EUR",
			expected: "function(value) { return Math.round(value).toLocaleString('uz-UZ') + ' EUR'; }",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := charts.FullCurrency(tt.locale, tt.currency)
			assert.Equal(t, tt.expected, string(result))
		})
	}
}

func TestCount(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		locale   string
		expected string
	}{
		{
			name:     "Russian locale",
			locale:   "ru-RU",
			expected: "function(value) { return Math.round(value).toLocaleString('ru-RU'); }",
		},
		{
			name:     "US locale",
			locale:   "en-US",
			expected: "function(value) { return Math.round(value).toLocaleString('en-US'); }",
		},
		{
			name:     "Uzbek locale",
			locale:   "uz-UZ",
			expected: "function(value) { return Math.round(value).toLocaleString('uz-UZ'); }",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := charts.Count(tt.locale)
			assert.Equal(t, tt.expected, string(result))
		})
	}
}

func TestLabeledCount(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		locale   string
		label    string
		expected string
	}{
		{
			name:     "English Policies",
			locale:   "en-US",
			label:    "Policies",
			expected: "function(value) { return Math.round(value).toLocaleString('en-US') + ' Policies'; }",
		},
		{
			name:     "Russian Contracts",
			locale:   "ru-RU",
			label:    "Контрактов",
			expected: "function(value) { return Math.round(value).toLocaleString('ru-RU') + ' Контрактов'; }",
		},
		{
			name:     "Uzbek Clients",
			locale:   "uz-UZ",
			label:    "Mijozlar",
			expected: "function(value) { return Math.round(value).toLocaleString('uz-UZ') + ' Mijozlar'; }",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := charts.LabeledCount(tt.locale, tt.label)
			assert.Equal(t, tt.expected, string(result))
		})
	}
}

func TestConditional(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		condition   string
		trueBranch  string
		falseBranch string
		expected    string
	}{
		{
			name:        "Large vs small values",
			condition:   "value >= 1000000",
			trueBranch:  "function(value) { return window.formatCurrency(value, 'en-US') + ' USD'; }",
			falseBranch: "function(value) { return Math.round(value).toLocaleString('en-US') + ' USD'; }",
			expected:    "function(value) { if (value >= 1000000) { return window.formatCurrency(value, 'en-US') + ' USD'; } else { return Math.round(value).toLocaleString('en-US') + ' USD'; } }",
		},
		{
			name:        "Positive vs negative",
			condition:   "value >= 0",
			trueBranch:  "function(value) { return Math.round(value).toLocaleString('ru-RU') + ' UZS'; }",
			falseBranch: "function(value) { return '(' + Math.round(Math.abs(value)).toLocaleString('ru-RU') + ' UZS)'; }",
			expected:    "function(value) { if (value >= 0) { return Math.round(value).toLocaleString('ru-RU') + ' UZS'; } else { return '(' + Math.round(Math.abs(value)).toLocaleString('ru-RU') + ' UZS)'; } }",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			// Convert raw strings to templ.JSExpression
			result := charts.Conditional(
				tt.condition,
				templ.JSExpression(tt.trueBranch),
				templ.JSExpression(tt.falseBranch),
			)
			assert.Equal(t, tt.expected, string(result))
		})
	}
}

func TestPercentage(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		decimals int
		expected string
	}{
		{
			name:     "No decimals",
			decimals: 0,
			expected: "function(value) { return value.toFixed(0) + '%'; }",
		},
		{
			name:     "One decimal",
			decimals: 1,
			expected: "function(value) { return value.toFixed(1) + '%'; }",
		},
		{
			name:     "Two decimals",
			decimals: 2,
			expected: "function(value) { return value.toFixed(2) + '%'; }",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := charts.Percentage(tt.decimals)
			assert.Equal(t, tt.expected, string(result))
		})
	}
}

func TestConditional_Integration(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		condition string
		locale    string
		currency  string
		threshold int64
	}{
		{
			name:      "Abbreviated above 1M",
			condition: "value >= 1000000",
			locale:    "en-US",
			currency:  "USD",
			threshold: 1000000,
		},
		{
			name:      "Abbreviated above 100K",
			condition: "value >= 100000",
			locale:    "ru-RU",
			currency:  "UZS",
			threshold: 100000,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := charts.Conditional(
				tt.condition,
				charts.AbbreviatedCurrency(tt.locale, tt.currency),
				charts.FullCurrency(tt.locale, tt.currency),
			)

			// Verify it contains key components
			resultStr := string(result)
			assert.Contains(t, resultStr, "function(value)")
			assert.Contains(t, resultStr, "if ("+tt.condition+")")
			assert.Contains(t, resultStr, "window.formatCurrency")
			assert.Contains(t, resultStr, "Math.round(value).toLocaleString")
			assert.Contains(t, resultStr, tt.currency)
		})
	}
}

func TestConditionalCurrency(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		locale    string
		currency  string
		threshold float64
	}{
		{
			name:      "1M threshold",
			locale:    "en-US",
			currency:  "USD",
			threshold: 1000000,
		},
		{
			name:      "100K threshold",
			locale:    "ru-RU",
			currency:  "UZS",
			threshold: 100000,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := charts.ConditionalCurrency(tt.locale, tt.currency, tt.threshold)

			// Verify it contains key components
			resultStr := string(result)
			assert.Contains(t, resultStr, "function(value)")
			assert.Contains(t, resultStr, "window.formatCurrency")
			assert.Contains(t, resultStr, "Math.round(value).toLocaleString")
			assert.Contains(t, resultStr, tt.currency)
		})
	}
}
