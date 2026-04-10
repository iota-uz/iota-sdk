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
				// Prevent browsers from caching authenticated pages so that
				// the back button after logout cannot display stale content.
				w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate")
				w.Header().Set("Pragma", "no-cache")
				w.Header().Set("Expires", "0")
				ctx := composables.WithPageCtx(r.Context(), types.NewPageContext(locale, r.URL, localizer))
				next.ServeHTTP(w, r.WithContext(ctx))
			},
		)
	}
}
