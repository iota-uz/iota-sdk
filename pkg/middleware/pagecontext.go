package middleware

import (
	"net/http"

	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/intl"
	"github.com/iota-uz/iota-sdk/pkg/types"

	"github.com/gorilla/mux"
)

func WithPageContext() mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {
				localizer, found := intl.UseLocalizer(r.Context())
				if !found {
					panic(intl.ErrNoLocalizer)
				}
				locale, ok := intl.UseLocale(r.Context())
				if !ok {
					panic("locale not found")
				}
				pageCtx := &types.PageContext{
					URL:       r.URL,
					Localizer: localizer,
					Locale:    locale,
				}
				next.ServeHTTP(w, r.WithContext(composables.WithPageCtx(r.Context(), pageCtx)))
			},
		)
	}
}
