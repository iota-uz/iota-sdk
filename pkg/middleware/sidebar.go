package middleware

import (
	"context"
	"github.com/gorilla/mux"
	"github.com/iota-agency/iota-sdk/modules"
	"github.com/iota-agency/iota-sdk/pkg/application"
	"github.com/iota-agency/iota-sdk/pkg/composables"
	"github.com/iota-agency/iota-sdk/pkg/constants"
	"net/http"
)

func NavItems(app application.Application) mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {
				localizer, ok := composables.UseLocalizer(r.Context())
				if !ok {
					panic("localizer not found")
				}
				user, err := composables.UseUser(r.Context())
				if err != nil {
					next.ServeHTTP(w, r)
					return
				}
				items := modules.GetNavItems(app, localizer, user)
				ctx := context.WithValue(r.Context(), constants.NavItemsKey, items)
				next.ServeHTTP(w, r.WithContext(ctx))
			},
		)
	}
}
