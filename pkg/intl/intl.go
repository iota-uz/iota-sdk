package intl

import (
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
