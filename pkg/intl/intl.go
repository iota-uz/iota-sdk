package intl

import (
	"fmt"
	"runtime"
	"strings"

	"github.com/iota-uz/go-i18n/v2/i18n"
	"golang.org/x/text/language"
)

type SupportedLanguage struct {
	Code        string
	VerboseName string
	Tag         language.Tag
}

var (
	// allSupportedLanguages is the master list of all languages the SDK supports
	allSupportedLanguages = []SupportedLanguage{
		{
			Code:        "ru",
			VerboseName: "Русский",
			Tag:         language.Russian,
		},
		{
			Code:        "en",
			VerboseName: "English",
			Tag:         language.English,
		},
		{
			Code:        "uz",
			VerboseName: "O'zbekcha",
			Tag:         language.Uzbek,
		},
	}

	// SupportedLanguages is the default list (all languages) for backward compatibility
	SupportedLanguages = allSupportedLanguages
)

// GetSupportedLanguages returns a filtered list of supported languages based on the whitelist.
// If whitelist is nil or empty, returns all supported languages.
// If whitelist is provided, only languages with codes in the whitelist are returned.
func GetSupportedLanguages(whitelist []string) []SupportedLanguage {
	// If no whitelist provided, return all languages (backward compatible)
	if len(whitelist) == 0 {
		return allSupportedLanguages
	}

	// Create a map for fast lookup
	whitelistMap := make(map[string]bool)
	for _, code := range whitelist {
		whitelistMap[code] = true
	}

	// Filter languages based on whitelist
	filtered := make([]SupportedLanguage, 0, len(whitelist))
	for _, lang := range allSupportedLanguages {
		if whitelistMap[lang.Code] {
			filtered = append(filtered, lang)
		}
	}

	return filtered
}

// MustLocalize localizes the message and panics with an actionable error if the key is missing.
// Use in shared request-path code (nav, layout, sidebar) so failures are easy to trace.
// The panic includes message_id, locale, callsite, and remediation.
func MustLocalize(localizer *i18n.Localizer, cfg *i18n.LocalizeConfig) string {
	s, err := localizer.Localize(cfg)
	if err != nil {
		_, file, line, _ := runtime.Caller(1)
		msgID := ""
		if cfg != nil {
			msgID = cfg.MessageID
		}
		panic(fmt.Sprintf("i18n missing translation: message_id=%q callsite=%s:%d error=%q hint=missing or not a leaf string (e.g. parent/category key) remediation=\"run: go test ./... -run TestI18nRequiredKeys or add the key to locale files\"",
			msgID, file, line, err))
	}
	return s
}

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
