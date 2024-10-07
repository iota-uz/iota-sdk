package composables

import (
	"context"

	ut "github.com/go-playground/universal-translator"
	"github.com/nicksnyder/go-i18n/v2/i18n"
)

// UseLocalizer returns the localizer from the context.
// If the localizer is not found, the second return value will be false.
func UseLocalizer(ctx context.Context) (*i18n.Localizer, bool) {
	l, ok := ctx.Value("localizer").(*i18n.Localizer)
	if !ok {
		return nil, false
	}
	return l, true
}

func UseUniLocalizer(ctx context.Context) (ut.Translator, bool) {
	l, ok := ctx.Value("uni_localizer").(ut.Translator)
	if !ok {
		return nil, false
	}
	return l, true
}

func UseLocalizedOrFallback(ctx context.Context, key string, fallback string) string {
	l, ok := UseLocalizer(ctx)
	if !ok {
		return fallback
	}
	return l.MustLocalize(&i18n.LocalizeConfig{ //nolint:exhaustruct
		MessageID: key,
		DefaultMessage: &i18n.Message{ //nolint:exhaustruct
			ID: fallback,
		},
	})
}
