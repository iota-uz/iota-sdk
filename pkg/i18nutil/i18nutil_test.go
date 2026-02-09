package i18nutil

import (
	"fmt"
	"testing"

	"github.com/iota-uz/go-i18n/v2/i18n"
	"github.com/stretchr/testify/require"
	"golang.org/x/text/language"
)

func TestMustLocalize_success(t *testing.T) {
	bundle := i18n.NewBundle(language.English)
	require.NoError(t, bundle.AddMessages(language.English, &i18n.Message{ID: "ok", Other: "OK"}))
	l := i18n.NewLocalizer(bundle, "en")
	out := MustLocalize(l, &i18n.LocalizeConfig{MessageID: "ok"})
	require.Equal(t, "OK", out)
}

func TestMustLocalize_panic(t *testing.T) {
	bundle := i18n.NewBundle(language.English)
	l := i18n.NewLocalizer(bundle, "en")
	var panicVal interface{}
	func() {
		defer func() { panicVal = recover() }()
		_ = MustLocalize(l, &i18n.LocalizeConfig{MessageID: "missing"})
	}()
	require.NotNil(t, panicVal)
	s := fmt.Sprintf("%v", panicVal)
	require.Contains(t, s, "message_id=\"missing\"", "panic must include message_id")
	require.Contains(t, s, "callsite=", "panic must include callsite")
	require.Contains(t, s, "remediation=", "panic must include remediation")
}

func TestValidateRequiredKeys_success(t *testing.T) {
	bundle := i18n.NewBundle(language.English)
	require.NoError(t, bundle.AddMessages(language.English, &i18n.Message{ID: "a", Other: "A"}))
	require.NoError(t, bundle.AddMessages(language.English, &i18n.Message{ID: "b", Other: "B"}))
	err := ValidateRequiredKeys(bundle, []string{"a", "b"}, language.English)
	require.NoError(t, err)
}

func TestValidateRequiredKeys_missing(t *testing.T) {
	bundle := i18n.NewBundle(language.English)
	require.NoError(t, bundle.AddMessages(language.English, &i18n.Message{ID: "a", Other: "A"}))
	err := ValidateRequiredKeys(bundle, []string{"a", "missing"}, language.English)
	require.Error(t, err)
	require.Contains(t, err.Error(), "en:missing")
}

func TestValidateRequiredKeys_nilBundle(t *testing.T) {
	err := ValidateRequiredKeys(nil, []string{"a"}, language.English)
	require.Error(t, err)
	require.Contains(t, err.Error(), "bundle is nil")
}
