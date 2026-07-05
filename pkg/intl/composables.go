// Package intl provides this package.
package intl

import (
	"context"
	"errors"
	"sync"

	"github.com/go-playground/locales/en"
	"github.com/go-playground/locales/pt_BR"
	"github.com/go-playground/locales/ru"
	"github.com/go-playground/locales/zh_Hans_CN"
	"github.com/go-playground/validator/v10"
	en_translations "github.com/go-playground/validator/v10/translations/en"
	pt_BR_translations "github.com/go-playground/validator/v10/translations/pt_BR"
	ru_translations "github.com/go-playground/validator/v10/translations/ru"
	zh_translations "github.com/go-playground/validator/v10/translations/zh"
	"github.com/iota-uz/iota-sdk/pkg/constants"
	"golang.org/x/text/language"

	ut "github.com/go-playground/universal-translator"
	"github.com/iota-uz/go-i18n/v2/i18n"
)

type contextKey string

const (
	LocalizerKey contextKey = "localizer"
	LocaleKey    contextKey = "locale"
)

var (
	registerTranslations = map[string]func(v *validator.Validate, trans ut.Translator) error{
		"en":    en_translations.RegisterDefaultTranslations,
		"ru":    ru_translations.RegisterDefaultTranslations,
		"pt-BR": pt_BR_translations.RegisterDefaultTranslations,
		"zh":    zh_translations.RegisterDefaultTranslations,
	}
	translationLock = sync.Mutex{}
	ErrNoLocalizer  = errors.New("localizer not found")
)

func WithLocalizer(ctx context.Context, l *i18n.Localizer) context.Context {
	return context.WithValue(ctx, LocalizerKey, l)
}

func WithLocale(ctx context.Context, l language.Tag) context.Context {
	return context.WithValue(ctx, LocaleKey, l)
}

// UseLocalizer returns the localizer from the context.
// If the localizer is not found, the second return value will be false.
func UseLocalizer(ctx context.Context) (*i18n.Localizer, bool) {
	l, ok := ctx.Value(LocalizerKey).(*i18n.Localizer)
	if !ok {
		return nil, false
	}
	return l, true
}

// MustT returns the translation for the given message ID.
// If the translation is not found, it will panic.
func MustT(ctx context.Context, msgID string) string {
	l, ok := UseLocalizer(ctx)
	if !ok {
		panic("localizer not found in context")
	}
	return l.MustLocalize(&i18n.LocalizeConfig{
		MessageID: msgID,
	})
}

// T returns the translation for the given message ID without panicking.
// The second return value is false when the localizer is missing from the
// context or the message ID is not present in any registered translation
// file. Callers should provide their own fallback rather than relying on
// MustT, which is fragile in user-supplied LabelKey scenarios.
func T(ctx context.Context, msgID string) (string, bool) {
	l, ok := UseLocalizer(ctx)
	if !ok {
		return "", false
	}
	out, err := l.Localize(&i18n.LocalizeConfig{MessageID: msgID})
	if err != nil {
		return "", false
	}
	return out, true
}

// TWithDefault returns the translation for msgID or the supplied default
// when the localizer is missing or the key is unknown. Equivalent to T but
// avoids forcing every caller to write the same two-line fallback.
func TWithDefault(ctx context.Context, msgID, fallback string) string {
	if out, ok := T(ctx, msgID); ok {
		return out
	}
	return fallback
}

func loadUniTranslator() *ut.UniversalTranslator {
	enLocale := en.New()
	ruLocale := ru.New()
	ptLocale := pt_BR.New()
	zhLocale := zh_Hans_CN.New()
	return ut.New(enLocale, enLocale, ruLocale, ptLocale, zhLocale)
}

func UseUniLocalizer(ctx context.Context) (ut.Translator, error) {
	uni := loadUniTranslator()
	locale, ok := UseLocale(ctx)
	if !ok {
		return nil, errors.New("locale not found in context")
	}
	trans, _ := uni.GetTranslator(locale.String())
	translationLock.Lock()
	defer translationLock.Unlock()
	register, ok := registerTranslations[locale.String()]
	if !ok {
		return nil, ErrNoLocalizer
	}
	if err := register(constants.Validate, trans); err != nil {
		return nil, err
	}
	return trans, nil
}

// UseLocale returns the locale from the context.
// If the locale is not found, the second return value will be false.
func UseLocale(ctx context.Context) (language.Tag, bool) {
	locale, ok := ctx.Value(LocaleKey).(language.Tag)
	if !ok {
		return language.Und, false
	}
	return locale, true
}
