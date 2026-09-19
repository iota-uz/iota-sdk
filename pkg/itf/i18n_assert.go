package itf

import (
	"testing"

	"github.com/iota-uz/go-i18n/v2/i18n"
)

// messageHint explains the three usual reasons a message id does not resolve,
// so a failing assertion points at the fix instead of just the symptom.
const messageHint = `message id did not resolve; check:
  1. the id exists in the locale file as a nested path — go-i18n reads "A.B.c"
     as table [A.B] with key "c" in TOML (a flat key "Bc" or a table/key mix-up
     with the same spelling does NOT match), and as nested objects in JSON;
  2. the component owning that locale file is part of the harness — the bundle
     is built only from the LocaleFS of the registered components;
  3. the key is present for every locale the test asserts on.`

// MessageResolves reports whether messageID localizes to a non-empty message
// in the given locale without falling back to another language. Use it
// directly when a test needs the boolean; use RequireMessage to fail with the
// diagnostic hint.
func MessageResolves(bundle *i18n.Bundle, locale, messageID string) bool {
	localizer := i18n.NewLocalizer(bundle, locale)
	msg, err := localizer.Localize(&i18n.LocalizeConfig{MessageID: messageID})
	return err == nil && msg != ""
}

// RequireMessage fails the test unless messageID resolves to a non-empty
// message in the given locale, returning the resolved message.
//
// Use it in tests that render localized output (templates, exports) to pin the
// message ids the code actually looks up. A silent fallback — the localizer
// returning the id itself or an empty string — is exactly the production bug
// this assertion exists to catch.
func RequireMessage(tb testing.TB, bundle *i18n.Bundle, locale, messageID string) string {
	tb.Helper()
	localizer := i18n.NewLocalizer(bundle, locale)
	msg, err := localizer.Localize(&i18n.LocalizeConfig{MessageID: messageID})
	if err != nil {
		tb.Fatalf("RequireMessage: %q for locale %q: %v\n%s", messageID, locale, err, messageHint)
		return ""
	}
	if msg == "" {
		tb.Fatalf("RequireMessage: %q resolved to an empty message for locale %q\n%s", messageID, locale, messageHint)
		return ""
	}
	return msg
}

// RequireMessages is RequireMessage over a list of message ids.
func RequireMessages(tb testing.TB, bundle *i18n.Bundle, locale string, messageIDs ...string) {
	tb.Helper()
	for _, id := range messageIDs {
		RequireMessage(tb, bundle, locale, id)
	}
}

// RequireMessageAllLocales fails the test unless messageID resolves to a
// non-empty message in EVERY listed locale, so a key shipped in en but
// forgotten in the other translations cannot pass unnoticed.
func RequireMessageAllLocales(tb testing.TB, bundle *i18n.Bundle, messageID string, locales ...string) {
	tb.Helper()
	if len(locales) == 0 {
		tb.Fatal("RequireMessageAllLocales: at least one locale is required")
	}
	for _, locale := range locales {
		RequireMessage(tb, bundle, locale, messageID)
	}
}
