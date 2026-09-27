package spotlight

import (
	"context"
	"testing"

	"github.com/iota-uz/go-i18n/v2/i18n"
	"github.com/iota-uz/iota-sdk/pkg/intl"
	"github.com/stretchr/testify/require"
	"golang.org/x/text/language"
)

func TestRegisterTranslations_LocalizesSpotlightDefaults(t *testing.T) {
	bundle := i18n.NewBundle(language.Russian)
	require.NoError(t, RegisterTranslations(bundle))

	tests := []struct {
		lang string
		key  string
		want string
	}{
		{lang: "en", key: "Spotlight.Badge.Action", want: "Action"},
		{lang: "ru", key: "Spotlight.Match.Exact", want: "Точное совпадение"},
		{lang: "uz", key: "Spotlight.Badge.Navigate", want: "Navigatsiya"},
		{lang: "uz-Cyrl", key: "Spotlight.Group.Knowledge", want: "Билим базаси"},
		{lang: "zh", key: "Spotlight.Match.Best", want: "最佳匹配"},
	}

	for _, tt := range tests {
		t.Run(tt.lang+"/"+tt.key, func(t *testing.T) {
			localizer := i18n.NewLocalizer(bundle, tt.lang)
			got, err := localizer.Localize(&i18n.LocalizeConfig{MessageID: tt.key})
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestRegisterTranslations_LocalizesMoreResults(t *testing.T) {
	bundle := i18n.NewBundle(language.Russian)
	require.NoError(t, RegisterTranslations(bundle))

	ctx := intl.WithLocalizer(context.Background(), i18n.NewLocalizer(bundle, "ru"))
	require.Equal(t, "Ещё 1 результат", moreResultsLabel(ctx, 1))
	require.Equal(t, "Ещё 2 результата", moreResultsLabel(ctx, 2))
	require.Equal(t, "Ещё 5 результатов", moreResultsLabel(ctx, 5))
}

func TestRegisterTranslations_AllowsConsumerOverrides(t *testing.T) {
	bundle := i18n.NewBundle(language.Russian)
	require.NoError(t, RegisterTranslations(bundle))
	bundle.MustAddMessages(language.Russian, &i18n.Message{
		ID:    "Spotlight.Badge.Action",
		Other: "Пользовательское действие",
	})

	localizer := i18n.NewLocalizer(bundle, "ru")
	got, err := localizer.Localize(&i18n.LocalizeConfig{MessageID: "Spotlight.Badge.Action"})
	require.NoError(t, err)
	require.Equal(t, "Пользовательское действие", got)
}
