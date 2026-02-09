package applet

import (
	"github.com/iota-uz/go-i18n/v2/i18n"
	"github.com/iota-uz/iota-sdk/pkg/i18nutil"
	"golang.org/x/text/language"
)

// ValidateI18n checks that every required message ID localizes successfully for every given locale.
// Use with Config.I18n.RequiredKeys (e.g. nav item keys) in CI to fail fast when a key is missing.
// Returns an error listing any missing keys per locale.
func ValidateI18n(bundle *i18n.Bundle, requiredKeys []string, locales ...language.Tag) error {
	return i18nutil.ValidateRequiredKeys(bundle, requiredKeys, locales...)
}
