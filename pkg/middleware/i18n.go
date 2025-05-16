package middleware

import (
	"context"
	"net/http"

	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/intl"

	"github.com/gorilla/mux"
	"github.com/iota-uz/go-i18n/v2/i18n"
	"golang.org/x/text/language"
)

func useLocaleFromUser(ctx context.Context) (language.Tag, error) {
	user, err := composables.UseUser(ctx)
	if err != nil {
		return language.Und, err
	}
	tag, err := language.Parse(string(user.UILanguage()))
	if err != nil {
		return language.Und, err
	}
	return tag, nil
}

func useLocale(r *http.Request, defaultLocale language.Tag) language.Tag {
	tag, err := useLocaleFromUser(r.Context())
	if err == nil {
		return tag
	}
	tags, _, err := language.ParseAcceptLanguage(r.Header.Get("Accept-Language"))
	if err != nil {
		return defaultLocale
	}
	if len(tags) == 0 {
		return defaultLocale
	}
	return tags[0]
}

func ProvideLocalizer(bundle *i18n.Bundle) mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {
				locale := useLocale(r, language.English)
				ctx := intl.WithLocalizer(
					r.Context(),
					i18n.NewLocalizer(bundle, locale.String()),
				)
				ctx = intl.WithLocale(ctx, locale)
				next.ServeHTTP(w, r.WithContext(ctx))
			},
		)
	}
}
