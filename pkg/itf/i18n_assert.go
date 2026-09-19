package itf

import (
	"testing"

	"github.com/iota-uz/go-i18n/v2/i18n"
	"golang.org/x/text/language"
)

// messageHint explains the usual reasons a message id does not resolve, so a
// failing assertion points at the fix instead of just the symptom.
const messageHint = `message id did not resolve; check:
  1. the id exists in the locale file as a nested path — go-i18n reads "A.B.c"
     as table [A.B] with key "c" in TOML (a flat key "Bc" or a table/key mix-up
     with the same spelling does NOT match), and as nested objects in JSON;
  2. the component owning that locale file is part of the harness — the bundle
     is built only from the LocaleFS of the registered components;
  3. the key is present for every locale the test asserts on, and the locale
     itself is one the bundle carries translations for.`

// resolveMessage localizes messageID for locale and reports whether the
// resolution is genuine: a non-empty message, no error, and a resolved
// language that IS the requested locale. The last check matters because the
// localizer happily serves the bundle's default language for an unsupported
// locale ("fr" falls back to "en" without an error), which would otherwise
// make a missing translation look resolved.
func resolveMessage(bundle *i18n.Bundle, locale, messageID string) (string, bool) {
	requested, err := language.Parse(locale)
	if err != nil {
		return "", false
	}
	localizer := i18n.NewLocalizer(bundle, locale)
	msg, tag, err := localizer.LocalizeWithTag(&i18n.LocalizeConfig{MessageID: messageID})
	if err != nil || msg == "" || tag != requested {
		return "", false
	}
	return msg, true
}

// MessageResolves reports whether messageID localizes to a non-empty message
// genuinely written for the given locale — neither an error fallback nor the
// bundle's default language served for an unsupported locale. Use it directly
// when a test needs the boolean; use RequireMessage to fail with the
// diagnostic hint.
func MessageResolves(bundle *i18n.Bundle, locale, messageID string) bool {
	_, ok := resolveMessage(bundle, locale, messageID)
	return ok
}

// RequireMessage fails the test unless messageID resolves to a non-empty
// message in the given locale, returning the resolved message.
//
// Use it in tests that render localized output (templates, exports) to pin the
// message ids the code actually looks up. A silent fallback — the localizer
// returning the id itself, an empty string, or another language's message —
// is exactly the production bug this assertion exists to catch.
func RequireMessage(tb testing.TB, bundle *i18n.Bundle, locale, messageID string) string {
	tb.Helper()
	msg, ok := resolveMessage(bundle, locale, messageID)
	if !ok {
		localizer := i18n.NewLocalizer(bundle, locale)
		fallback, _, _ := localizer.LocalizeWithTag(&i18n.LocalizeConfig{MessageID: messageID})
		tb.Fatalf(
			"RequireMessage: %q did not resolve for locale %q (best-effort lookup returned %q)\n%s",
			messageID, locale, fallback, messageHint,
		)
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
