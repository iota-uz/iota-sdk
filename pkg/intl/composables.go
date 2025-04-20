package intl

import (
	"context"
	"errors"
	"sync"

	"github.com/go-playground/locales/en"
	"github.com/go-playground/locales/ru"
	"github.com/go-playground/validator/v10"
	en_translations "github.com/go-playground/validator/v10/translations/en"
	ru_translations "github.com/go-playground/validator/v10/translations/ru"
	"github.com/iota-uz/iota-sdk/pkg/constants"
	"golang.org/x/text/language"

	ut "github.com/go-playground/universal-translator"
	"github.com/nicksnyder/go-i18n/v2/i18n"
)

type contextKey string

const (
	LocalizerKey contextKey = "localizer"
	LocaleKey    contextKey = "locale"
)

var (
	registerTranslations = map[string]func(v *validator.Validate, trans ut.Translator) error{
		"en": en_translations.RegisterDefaultTranslations,
		"ru": ru_translations.RegisterDefaultTranslations,
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

func loadUniTranslator() *ut.UniversalTranslator {
	enLocale := en.New()
	ruLocale := ru.New()
	return ut.New(enLocale, enLocale, ruLocale)
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
