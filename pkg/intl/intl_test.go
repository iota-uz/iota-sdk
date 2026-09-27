package intl

import (
	"fmt"
	"testing"

	"github.com/iota-uz/go-i18n/v2/i18n"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/text/language"
)

func TestGetSupportedLanguagesIncludesUzCyrl(t *testing.T) {
	t.Parallel()

	languages := GetSupportedLanguages(nil)

	for _, lang := range languages {
		if lang.Code == "uz-Cyrl" {
			require.Equal(t, "Ўзбекча", lang.VerboseName)
			require.Equal(t, "uz-Cyrl", lang.Tag.String())
			return
		}
	}

	t.Fatalf("expected uz-Cyrl in supported languages, got %+v", languages)
}

func TestMustLocalizePanicsOnMissingKey(t *testing.T) {
	t.Parallel()

	bundle := i18n.NewBundle(language.English)
	localizer := i18n.NewLocalizer(bundle, language.English.String())

	var recovered any
	func() {
		defer func() {
			recovered = recover()
		}()
		MustLocalize(localizer, &i18n.LocalizeConfig{MessageID: "Missing.Key"})
	}()

	require.NotNil(t, recovered)
	assert.Contains(t, fmt.Sprint(recovered), `message_id="Missing.Key"`)
	assert.Contains(t, fmt.Sprint(recovered), `Missing.Key`)
	assert.Contains(t, fmt.Sprint(recovered), `not found in language`)
}

func TestTReturnsFalseOnMissingKeyOrLocalizer(t *testing.T) {
	t.Parallel()

	got, ok := T(t.Context(), "Missing.Key")
	require.False(t, ok)
	require.Empty(t, got)

	bundle := i18n.NewBundle(language.English)
	bundle.MustAddMessages(language.English, &i18n.Message{ID: "Spotlight.Present", Other: "found"})
	localizer := i18n.NewLocalizer(bundle, language.English.String())
	ctx := WithLocalizer(t.Context(), localizer)

	got, ok = T(ctx, "Spotlight.Present")
	require.True(t, ok)
	require.Equal(t, "found", got)

	got, ok = T(ctx, "Spotlight.NotPresent")
	require.False(t, ok)
	require.Empty(t, got)
}

func TestTWithDefaultFallsBackOnMissing(t *testing.T) {
	t.Parallel()

	bundle := i18n.NewBundle(language.English)
	bundle.MustAddMessages(language.English, &i18n.Message{ID: "Spotlight.Greeting", Other: "hi"})
	localizer := i18n.NewLocalizer(bundle, language.English.String())
	ctx := WithLocalizer(t.Context(), localizer)

	require.Equal(t, "hi", TWithDefault(ctx, "Spotlight.Greeting", "fallback"))
	require.Equal(t, "fallback", TWithDefault(ctx, "Spotlight.Absent", "fallback"))
	require.Equal(t, "noctx", TWithDefault(t.Context(), "Spotlight.Greeting", "noctx"))
}

func TestValidateRequiredKeys(t *testing.T) {
	t.Parallel()

	bundle := i18n.NewBundle(language.English)
	bundle.MustAddMessages(language.English, &i18n.Message{ID: "Lens.Ready", Other: "ready"})

	require.NoError(t, ValidateRequiredKeys(bundle, []string{"Lens.Ready"}, language.English))
	require.EqualError(t, ValidateRequiredKeys(bundle, []string{"Lens.Ready", "Lens.Missing"}, language.English), "i18n missing required keys: en:Lens.Missing")
	require.EqualError(t, ValidateRequiredKeys(nil, []string{"Lens.Ready"}, language.English), "bundle is nil")
}
