package multilang

import (
	"context"
	"strings"

	"github.com/iota-uz/go-i18n/v2/i18n"
	"github.com/iota-uz/iota-sdk/pkg/crud/models"
	"github.com/iota-uz/iota-sdk/pkg/intl"
)

// localizeWithDefault localizes a message with a fallback default
func localizeWithDefault(ctx context.Context, messageID string, defaultMessage string) string {
	l, ok := intl.UseLocalizer(ctx)
	if !ok {
		return defaultMessage
	}

	result, err := l.Localize(&i18n.LocalizeConfig{
		MessageID: messageID,
		DefaultMessage: &i18n.Message{
			ID:    messageID,
			Other: defaultMessage,
		},
	})
	if err != nil {
		return defaultMessage
	}
	return result
}

// getCurrentLocaleCode returns the current locale code from context with fallback
func getCurrentLocaleCode(ctx context.Context) string {
	locale, ok := intl.UseLocale(ctx)
	if !ok {
		return "en" // fallback to English
	}

	// Convert language.Tag to string and return as is to support variants like uz-Cyrl
	localeStr := locale.String()

	// Normalize to lowercase for consistency
	if localeStr != "" {
		return strings.ToLower(localeStr)
	}

	return "en" // fallback to English
}

// getLocalizedText returns text in user's locale with custom fallback priority
func getLocalizedText(ctx context.Context, ml models.MultiLang) string {
	if ml.IsEmpty() {
		return ""
	}

	// Get current user locale
	currentLocale := getCurrentLocaleCode(ctx)

	// Try current locale first
	if value, err := ml.Get(currentLocale); err == nil && value != "" {
		return value
	}

	// Define fallback priority based on current locale
	var fallbackPriorities []string
	switch currentLocale {
	case "ru":
		fallbackPriorities = []string{"uz", "uz-cyrl", "en"}
	case "uz":
		fallbackPriorities = []string{"uz-cyrl", "ru", "en"}
	case "uz-cyrl":
		fallbackPriorities = []string{"uz", "ru", "en"}
	default: // "en" or any other
		fallbackPriorities = []string{"uz", "uz-cyrl", "ru"}
	}

	// Try fallback priorities
	for _, locale := range fallbackPriorities {
		if value, err := ml.Get(locale); err == nil && value != "" {
			return value
		}
	}

	// If nothing found, return first available value
	for _, value := range ml.GetAll() {
		if value != "" {
			return value
		}
	}

	return ""
}
