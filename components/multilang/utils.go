package multilang

import (
	"context"
	"github.com/iota-uz/go-i18n/v2/i18n"
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
