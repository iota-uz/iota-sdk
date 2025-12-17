package commands

import (
	"fmt"
	"strings"

	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/commands/common"
	"github.com/iota-uz/iota-sdk/pkg/configuration"
	"github.com/sirupsen/logrus"
	"golang.org/x/text/language"
)

func CheckTrKeys(allowedLanguages []string, mods ...application.Module) error {
	conf := configuration.Use()
	app, pool, err := common.NewApplicationWithDefaults(mods...)
	if err != nil {
		return fmt.Errorf("failed to initialize application: %w", err)
	}
	defer pool.Close()

	messages := app.Bundle().Messages()

	// If allowedLanguages is provided, create a whitelist map for validation
	var allowedLocales map[string]language.Tag
	if len(allowedLanguages) > 0 {
		allowedLocales = make(map[string]language.Tag)
		for _, code := range allowedLanguages {
			// Parse language code to tag
			tag, err := language.Parse(code)
			if err != nil {
				return fmt.Errorf("invalid language code in whitelist: %s: %w", code, err)
			}
			allowedLocales[code] = tag
		}

		// Validate that all allowed languages exist in the bundle
		for code, tag := range allowedLocales {
			if messages[tag] == nil {
				return fmt.Errorf("language %s (%s) is in whitelist but not found in bundle", code, tag)
			}
		}
	}

	// Store all keys for each locale
	allKeys := make(map[string]map[language.Tag]bool)
	locales := make([]language.Tag, 0)

	// First pass: collect all keys from locales (filtered by allowedLanguages if provided)
	for locale, message := range messages {
		if message == nil {
			continue
		}

		// If allowedLanguages is specified, only process those locales
		if len(allowedLocales) > 0 {
			isAllowed := false
			for _, allowedTag := range allowedLocales {
				if locale == allowedTag {
					isAllowed = true
					break
				}
			}
			if !isAllowed {
				continue
			}
		}

		locales = append(locales, locale)

		// Process keys in this locale
		for key := range message {
			if allKeys[key] == nil {
				allKeys[key] = make(map[language.Tag]bool)
			}
			allKeys[key][locale] = true
		}
	}

	// No locales found
	if len(locales) == 0 {
		return fmt.Errorf("no locales found in the application bundle")
	}

	// Second pass: check for missing keys
	missingKeys := false
	for key, localeMap := range allKeys {
		// If the key is not present in all locales
		if len(localeMap) != len(locales) {
			missingKeys = true

			// Find which locales have the key
			present := make([]string, 0)
			missing := make([]string, 0)

			for _, locale := range locales {
				if localeMap[locale] {
					present = append(present, locale.String())
				} else {
					missing = append(missing, locale.String())
				}
			}

			// Log detailed error about the missing key using WithFields
			conf.Logger().WithFields(logrus.Fields{
				"key":          key,
				"present_in":   strings.Join(present, ", "),
				"missing_from": strings.Join(missing, ", "),
			}).Error("Translation key mismatch")
		}
	}

	if missingKeys {
		return fmt.Errorf("some translation keys are not consistent across all locales, see logs for details")
	}

	conf.Logger().WithFields(logrus.Fields{
		"locale_count": len(locales),
		"key_count":    len(allKeys),
	}).Info("All translation keys are consistent across all locales")

	return nil
}
