package composables_test

import (
	"context"
	"testing"
	"time"

	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/intl"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/text/language"
)

func TestFormatDateTime(t *testing.T) {
	t.Parallel()

	tashkent, err := time.LoadLocation("Asia/Tashkent")
	require.NoError(t, err)
	instant := time.Date(2026, 11, 13, 12, 30, 0, 0, time.UTC)

	t.Run("no location configured", func(t *testing.T) {
		t.Parallel()
		text, ok := composables.FormatDateTime(context.Background(), instant)
		assert.False(t, ok)
		assert.Empty(t, text)
	})

	tests := []struct {
		name   string
		locale language.Tag
		want   string
	}{
		{name: "russian uses dotted date", locale: language.Russian, want: "13.11.2026 17:30"},
		{name: "uzbek uses dotted date", locale: language.Uzbek, want: "13.11.2026 17:30"},
		{name: "english uses ISO date", locale: language.English, want: "2026-11-13 17:30"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctx := intl.WithLocale(composables.WithLocation(context.Background(), tashkent), tt.locale)
			text, ok := composables.FormatDateTime(ctx, instant)
			require.True(t, ok)
			assert.Equal(t, tt.want, text)
		})
	}
}
