package money_test

import (
	"testing"

	"github.com/iota-uz/iota-sdk/pkg/money"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseAmount(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"plain integer", "15000000", "15000000"},
		{"dot decimal", "15000000.50", "15000000.5"},
		{"comma decimal", "15000000,50", "15000000.5"},
		{"spaces and comma", "15 000 000,50", "15000000.5"},
		{"no-break spaces and comma", "15 000 000,50", "15000000.5"},
		{"narrow no-break space", "15 000 000", "15000000"},
		{"comma groups and dot decimal", "15,000,000.50", "15000000.5"},
		{"dot groups and comma decimal", "15.000.000,50", "15000000.5"},
		{"repeated comma groups digits", "1,234,567", "1234567"},
		{"repeated dot groups digits", "1.234.567", "1234567"},
		{"lone comma is decimal", "1,5", "1.5"},
		{"apostrophe groups", "15'000'000", "15000000"},
		{"leading separator", ",5", "0.5"},
		{"trailing separator", "12,", "12"},
		{"surrounding whitespace", "  42 ", "42"},
		{"negative", "-1 500,25", "-1500.25"},
		{"unicode minus", "−1500", "-1500"},
		{"zero", "0", "0"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := money.ParseAmount(tt.input)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got.String())
		})
	}
}

func TestParseAmount_Invalid(t *testing.T) {
	t.Parallel()

	for _, input := range []string{"", "   ", "abc", "12abc", "1e5", "1,2,3.4,5", "1.2,3.4", "--5", "12-", "$100", "."} {
		t.Run(input, func(t *testing.T) {
			t.Parallel()
			_, err := money.ParseAmount(input)
			assert.ErrorIs(t, err, money.ErrInvalidAmount)
		})
	}
}
