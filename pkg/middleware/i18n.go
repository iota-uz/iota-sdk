// Package middleware provides this package.
package middleware

import (
	"context"
	"net/http"
	"slices"

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

func useLocaleFromHeader(r *http.Request, defaultLocale language.Tag, supported []language.Tag, fallbackToDefault bool) language.Tag {
	tags, _, err := language.ParseAcceptLanguage(r.Header.Get("Accept-Language"))
	if err != nil || len(tags) == 0 {
		return defaultLocale
	}
	matcher := language.NewMatcher(supported)
	_, idx, confidence := matcher.Match(tags...)
	if fallbackToDefault && confidence == language.No {
		return defaultLocale
	}
	return supported[idx]
}

// withDefaultFirst moves the configured default language to the front of the
// supported list. The language matcher falls back to its first entry when the
// request matches nothing, so this makes the default the fallback.
func withDefaultFirst(configured string, supported []language.Tag) []language.Tag {
	if configured == "" {
		return supported
	}
	tag, err := language.Parse(configured)
	if err != nil {
		return supported
	}
	idx := slices.Index(supported, tag)
	if idx <= 0 {
		return supported
	}
	ordered := make([]language.Tag, 0, len(supported))
	ordered = append(ordered, tag)
	ordered = append(ordered, supported[:idx]...)
	return append(ordered, supported[idx+1:]...)
}

// resolveDefaultLocale picks the fallback locale: the configured default when
// it is enabled (explicit), otherwise English when enabled, otherwise the first
// enabled language.
func resolveDefaultLocale(configured string, supported []language.Tag) (language.Tag, bool) {
	if len(supported) == 0 {
		return language.English, false
	}
	if configured != "" {
		if tag, err := language.Parse(configured); err == nil && slices.Contains(supported, tag) {
			return tag, true
		}
	}
	if slices.Contains(supported, language.English) {
		return language.English, false
	}
	return supported[0], false
}

func useLocale(r *http.Request, defaultLocale language.Tag, supported []language.Tag, fallbackToDefault bool) language.Tag {
	tag, err := useLocaleFromUser(r.Context())
	if err == nil {
		matcher := language.NewMatcher(supported)
		_, idx, confidence := matcher.Match(tag)
		if confidence >= language.High {
			return supported[idx]
		}
	}
	return useLocaleFromHeader(r, defaultLocale, supported, fallbackToDefault)
}

type LocaleOptions struct {
	AcceptLanguageHighPriority bool
	// DefaultLanguage is the language code used when neither the user's saved
	// preference nor Accept-Language selects an enabled language. When empty
	// or not enabled, English is used if enabled, otherwise the first enabled
	// language.
	DefaultLanguage string
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
	configuredDefault := ""
	if len(opts) > 0 {
		acceptLanguageHighPriority = opts[0].AcceptLanguageHighPriority
		configuredDefault = opts[0].DefaultLanguage
	}
	supportedLanguages = withDefaultFirst(configuredDefault, supportedLanguages)
	defaultLocale, fallbackToDefault := resolveDefaultLocale(configuredDefault, supportedLanguages)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {
				var locale language.Tag
				if acceptLanguageHighPriority {
					locale = useLocaleFromHeader(r, defaultLocale, supportedLanguages, fallbackToDefault)
				} else {
					locale = useLocale(r, defaultLocale, supportedLanguages, fallbackToDefault)
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
