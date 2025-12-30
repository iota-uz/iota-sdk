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

// Application interface for accessing app config needed by localizer
type Application interface {
	Bundle() *i18n.Bundle
	GetSupportedLanguages() []string
}

// languageTagsFromCodes converts language codes to language.Tag slice
func languageTagsFromCodes(codes []string) []language.Tag {
	supported := intl.GetSupportedLanguages(codes)
	tags := make([]language.Tag, len(supported))
	for i, lang := range supported {
		tags[i] = lang.Tag
	}
	return tags
}

// useLocaleFromUser obtains the current user's UI language from the context and parses it into a language.Tag.
// If retrieving the user from the context or parsing the UI language fails, it returns language.Und and the encountered error.
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

// useLocaleFromHeader determines the best matching supported language.Tag for the request's
// Accept-Language header. If the header is missing or cannot be parsed it returns
// defaultLocale; otherwise it matches the parsed tags against the provided supported
// list and returns the chosen supported tag.
func useLocaleFromHeader(r *http.Request, defaultLocale language.Tag, supported []language.Tag) language.Tag {
	tags, _, err := language.ParseAcceptLanguage(r.Header.Get("Accept-Language"))
	if err != nil || len(tags) == 0 {
		return defaultLocale
	}
	matcher := language.NewMatcher(supported)
	_, idx, _ := matcher.Match(tags...)
	return supported[idx]
}

// useLocale selects the locale for the request.
// It first attempts to use the current user's UILanguage and, if that language matches one of the supported tags with confidence High or higher, returns the matched supported tag.
// If no sufficiently confident user locale is found, it falls back to parsing the request's Accept-Language header and returns the best match or the provided defaultLocale.
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

// ProvideLocalizer constructs a mux middleware that resolves a request locale and injects
// an i18n localizer and the resolved language.Tag into the request context.
//
// ProvideLocalizer uses the application's i18n bundle and supported languages to determine
// the best locale for each request. By default the middleware prefers a locale derived
// from the authenticated user when available; if opts is provided and opts[0].AcceptLanguageHighPriority
// is true, the middleware instead favors the request's Accept-Language header. When no suitable
// match is found the middleware falls back to English. The resolved locale is stored in the
// context alongside an i18n.Localizer for use by downstream handlers.
func ProvideLocalizer(app Application, opts ...LocaleOptions) mux.MiddlewareFunc {
	bundle := app.Bundle()
	supportedLanguages := languageTagsFromCodes(app.GetSupportedLanguages())

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