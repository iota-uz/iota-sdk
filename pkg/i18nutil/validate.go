package i18nutil

import (
	"fmt"
	"strings"

	"github.com/iota-uz/go-i18n/v2/i18n"
	"golang.org/x/text/language"
)

// ValidateRequiredKeys checks that every required message ID localizes successfully
// for every given locale. Returns an error listing any missing keys per locale.
func ValidateRequiredKeys(bundle *i18n.Bundle, required []string, locales ...language.Tag) error {
	if bundle == nil {
		return fmt.Errorf("bundle is nil")
	}
	var missing []string
	for _, loc := range locales {
		l := i18n.NewLocalizer(bundle, loc.String())
		for _, key := range required {
			_, err := l.Localize(&i18n.LocalizeConfig{MessageID: key})
			if err != nil {
				missing = append(missing, fmt.Sprintf("%s:%s", loc.String(), key))
			}
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("i18n missing required keys: %s", strings.Join(missing, ", "))
	}
	return nil
}
