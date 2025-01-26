package middleware

import (
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/types"
	"golang.org/x/text/language"
	"net/http"

	"github.com/gorilla/mux"
)

func WithPageContext() mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {
				localizer, found := composables.UseLocalizer(r.Context())
				if !found {
					panic("localizer not found")
				}
				pageCtx := &types.PageContext{
					URL:       r.URL,
					Localizer: localizer,
					Locale:    composables.UseLocale(r.Context(), language.English),
				}
				next.ServeHTTP(w, r.WithContext(composables.WithPageCtx(r.Context(), pageCtx)))
			},
		)
	}
}
