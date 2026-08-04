// Package middleware provides this package.
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

// languageTagsFromCodes converts language codes to language.Tag slice
func languageTagsFromCodes(codes []string) []language.Tag {
	supported := intl.GetSupportedLanguages(codes)
	tags := make([]language.Tag, len(supported))
	for i, lang := range supported {
		tags[i] = lang.Tag
	}
	return tags
}

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

func useLocaleFromHeader(r *http.Request, defaultLocale language.Tag, supported []language.Tag) language.Tag {
	tags, _, err := language.ParseAcceptLanguage(r.Header.Get("Accept-Language"))
	if err != nil || len(tags) == 0 {
		return defaultLocale
	}
	matcher := language.NewMatcher(supported)
	_, idx, _ := matcher.Match(tags...)
	return supported[idx]
}

func useLocale(r *http.Request, defaultLocale language.Tag, supported []language.Tag) language.Tag {
	tag, err := useLocaleFromUser(r.Context())
	if err == nil {
		matcher := language.NewMatcher(supported)
		_, idx, confidence := matcher.Match(tag)
		if confidence >= language.High {
			return supported[idx]
		}
	}
	return useLocaleFromHeader(r, defaultLocale, supported)
}

type LocaleOptions struct {
	AcceptLanguageHighPriority bool
}

// ProvideLocalizer returns middleware that resolves the request locale and
// attaches an i18n.Localizer to the request context. Bundle and supported
// language codes are captured at construction time so the middleware does
// no per-request DI lookup.
//
// The bundle and supported-language list are snapshotted when the middleware
// is installed — runtime changes to either (e.g. adding a language pack or
// swapping the bundle on the application handle) are not observed. Rebuild
// the middleware if that's required.
//
// When installed globally (before any per-route auth middleware), the locale
// is derived from Accept-Language because the user is not yet loaded into
// the context. ProvideUser runs later and re-derives the locale from the
// authenticated user's saved UI language preference — see
// refreshLocalizerForUser in pkg/middleware/auth.go.
func ProvideLocalizer(bundle *i18n.Bundle, supportedLanguageCodes []string, opts ...LocaleOptions) mux.MiddlewareFunc {
	supportedLanguages := languageTagsFromCodes(supportedLanguageCodes)

	acceptLanguageHighPriority := false
	if len(opts) > 0 {
		acceptLanguageHighPriority = opts[0].AcceptLanguageHighPriority
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {
				var locale language.Tag
				if acceptLanguageHighPriority {
					locale = useLocaleFromHeader(r, language.English, supportedLanguages)
				} else {
					locale = useLocale(r, language.English, supportedLanguages)
				}
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
