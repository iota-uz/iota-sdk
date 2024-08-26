package middleware

import (
	"net/http"

	"github.com/gorilla/mux"
	"github.com/iota-agency/iota-erp/sdk/composables"
	"github.com/iota-agency/iota-erp/sdk/middleware"
	"github.com/nicksnyder/go-i18n/v2/i18n"
	"golang.org/x/text/language"
)

func WithLocalizer(bundle *i18n.Bundle) mux.MiddlewareFunc {
	return middleware.ContextKeyValue("localizer", func(r *http.Request, w http.ResponseWriter) interface{} {
		locale := composables.UseLocale(r.Context(), language.English)
		return i18n.NewLocalizer(bundle, locale.String())
	})
}
