package i18nutil

import (
	"fmt"
	"runtime"

	"github.com/iota-uz/go-i18n/v2/i18n"
)

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
		panic(fmt.Sprintf("i18n missing translation: message_id=%q callsite=%s:%d hint=missing or not a leaf string (e.g. parent/category key) remediation=\"run: go test ./... -run TestI18nRequiredKeys or add the key to locale files\"",
			msgID, file, line))
	}
	return s
}
