package serrors_test

import (
	"testing"

	"github.com/iota-uz/go-i18n/v2/i18n"
	serrors "github.com/iota-uz/iota-sdk/pkg/serrors/v2"
	"github.com/stretchr/testify/require"
	"golang.org/x/text/language"
)

func TestMessageFromConfig_LocalizationAndPlural(t *testing.T) {
	t.Parallel()
	bundle := i18n.NewBundle(language.English)
	require.NoError(t, bundle.AddMessages(language.English,
		&i18n.Message{ID: "count", One: "{{.PluralCount}} item", Other: "{{.PluralCount}} items"},
		&i18n.Message{ID: "name", Other: "Name"},
		&i18n.Message{ID: "validation", Other: "{{.Field}}: {{.Value}}"},
	))
	l := i18n.NewLocalizer(bundle, "en")
	message, err := serrors.MessageFromConfig(&i18n.LocalizeConfig{MessageID: "count", PluralCount: "2"})
	require.NoError(t, err)
	require.Equal(t, "2 items", serrors.Localize(message, l, "fallback"))
	message, err = serrors.MessageFromConfig(&i18n.LocalizeConfig{MessageID: "validation", TemplateData: map[string]any{"Field": serrors.Message{ID: "name"}, "Value": "kept"}})
	require.NoError(t, err)
	require.Equal(t, "Name: kept", serrors.Localize(message, l, "fallback"))
}

func TestMessageFromConfig_RejectsArbitraryObjects(t *testing.T) {
	t.Parallel()
	for _, cfg := range []*i18n.LocalizeConfig{
		{TemplateData: struct{ Password string }{"secret"}},
		{TemplateData: map[string]any{"nested": []string{"secret"}}},
		{TemplateData: map[string]any{"ptr": new(string)}},
		{PluralCount: uint64(1 << 63)},
		{PluralCount: 1.5},
	} {
		message, err := serrors.MessageFromConfig(cfg)
		require.Error(t, err)
		require.Equal(t, "safe", serrors.Localize(message, nil, "safe"))
	}
}

func TestExplicitLiteral_MissingTranslationStillUsesSafeFallback(t *testing.T) {
	t.Parallel()
	require.Equal(t, "Approved literal", serrors.Localize(serrors.Message{Text: "Approved literal"}, nil, "safe"))
	require.Equal(t, "safe", serrors.Localize(serrors.Message{ID: "missing", Text: "Approved literal"}, nil, "safe"))
	require.Equal(t, "safe", serrors.Localize(serrors.Message{}, nil, "safe"))
}
