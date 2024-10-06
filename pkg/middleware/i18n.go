package middleware

import (
	"github.com/go-playground/locales/en"
	"github.com/go-playground/locales/ru"
	ut "github.com/go-playground/universal-translator"
	"github.com/go-playground/validator/v10"
	en_translations "github.com/go-playground/validator/v10/translations/en"
	ru_translations "github.com/go-playground/validator/v10/translations/ru"
	"github.com/iota-agency/iota-erp/pkg/constants"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/iota-agency/iota-erp/sdk/composables"
	"github.com/iota-agency/iota-erp/sdk/middleware"
	"github.com/nicksnyder/go-i18n/v2/i18n"
	"golang.org/x/text/language"
)

var (
	registerTranslations = map[string]func(v *validator.Validate, trans ut.Translator) error{
		"en": en_translations.RegisterDefaultTranslations,
		"ru": ru_translations.RegisterDefaultTranslations,
	}
)

func loadUniTranslator() *ut.UniversalTranslator {
	enLocale := en.New()
	ruLocale := ru.New()
	return ut.New(enLocale, enLocale, ruLocale)
}

func WithLocalizer(bundle *i18n.Bundle) mux.MiddlewareFunc {
	return middleware.ContextKeyValue("localizer", func(r *http.Request, w http.ResponseWriter) interface{} {
		locale := composables.UseLocale(r.Context(), language.English)
		return i18n.NewLocalizer(bundle, locale.String())
	})
}

func WithUniLocalizer() mux.MiddlewareFunc {
	return middleware.ContextKeyValue("uni_localizer", func(r *http.Request, w http.ResponseWriter) interface{} {
		uni := loadUniTranslator()
		locale, _ := composables.UseLocale(r.Context(), language.English).Base()
		trans, _ := uni.GetTranslator(locale.String())
		if register, ok := registerTranslations[locale.String()]; ok {
			if err := register(constants.Validate, trans); err != nil {
				panic(err)
			}
		} else {
			panic("No translations found for locale " + locale.String())
		}
		return trans
	})
}
