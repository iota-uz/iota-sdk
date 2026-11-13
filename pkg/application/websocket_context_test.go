package application

import (
	"context"
	"testing"
	"time"

	"github.com/iota-uz/go-i18n/v2/i18n"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/internet"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/intl"
	"github.com/stretchr/testify/require"
	"golang.org/x/text/language"
)

func TestHuberConnectionContext(t *testing.T) {
	email, err := internet.NewEmail("ws@example.com")
	require.NoError(t, err)
	usr := user.New("WS", "User", email, user.UILanguageRU)
	h := &huber{bundle: i18n.NewBundle(language.Russian)}

	t.Run("carries locale without a configured time zone", func(t *testing.T) {
		ctx := h.connectionContext(context.Background(), usr)
		locale, ok := intl.UseLocale(ctx)
		require.True(t, ok)
		require.Equal(t, language.Russian, locale)
		_, hasLocation := composables.UseLocation(ctx)
		require.False(t, hasLocation)
	})

	t.Run("carries the application time zone", func(t *testing.T) {
		tashkent, err := time.LoadLocation("Asia/Tashkent")
		require.NoError(t, err)
		h.SetLocation(tashkent)
		ctx := h.connectionContext(context.Background(), usr)
		loc, ok := composables.UseLocation(ctx)
		require.True(t, ok)
		require.Equal(t, "Asia/Tashkent", loc.String())
		text, ok := composables.FormatDateTime(ctx, time.Date(2026, 11, 13, 12, 30, 0, 0, time.UTC))
		require.True(t, ok)
		require.Equal(t, "13.11.2026 17:30", text)
	})
}

func TestNew_PropagatesLocationToHub(t *testing.T) {
	hub := NewHub(&HuberOptions{})
	_, err := New(&ApplicationOptions{Bundle: LoadBundle(), Huber: hub, Timezone: "Asia/Tashkent"})
	require.NoError(t, err)
	got := hub.(*huber).location.Load()
	require.NotNil(t, got)
	require.Equal(t, "Asia/Tashkent", got.String())
}
