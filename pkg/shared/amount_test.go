package shared

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDecoder_Amount(t *testing.T) {
	t.Parallel()

	type form struct {
		Amount Amount
	}

	tests := []struct {
		name      string
		values    url.Values
		wantValue string
		wantEmpty bool
		wantErr   bool
	}{
		{"spaces and comma", url.Values{"Amount": {"15 000 000,50"}}, "15000000.5", false, false},
		{"no-break spaces", url.Values{"Amount": {"15 000 000.50"}}, "15000000.5", false, false},
		{"empty", url.Values{"Amount": {""}}, "0", true, false},
		{"absent", url.Values{}, "0", true, false},
		{"not a number", url.Values{"Amount": {"abc"}}, "0", false, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var dst form
			require.NoError(t, Decoder.Decode(&dst, tt.values))
			assert.Equal(t, tt.wantValue, dst.Amount.Decimal().String())
			assert.Equal(t, tt.wantEmpty, dst.Amount.IsEmpty())
			assert.Equal(t, tt.wantErr, dst.Amount.Err() != nil)
		})
	}
}
