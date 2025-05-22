package commands

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/iota-uz/iota-sdk/modules"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/configuration"
	"github.com/iota-uz/iota-sdk/pkg/eventbus"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sirupsen/logrus"
	"golang.org/x/text/language"
)

func CheckTrKeys(mods ...application.Module) error {
	conf := configuration.Use()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()
	pool, err := pgxpool.New(ctx, conf.Database.Opts)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer pool.Close()
	app := application.New(&application.ApplicationOptions{
		Pool:     pool,
		EventBus: eventbus.NewEventPublisher(conf.Logger()),
		Logger:   conf.Logger(),
	})
	if err := modules.Load(app, mods...); err != nil {
		return err
	}

	messages := app.Bundle().Messages()

	// Store all keys for each locale
	allKeys := make(map[string]map[language.Tag]bool)
	locales := make([]language.Tag, 0)

	// First pass: collect all keys from all locales
	for locale, message := range messages {
		if message == nil {
			continue
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
