package middleware

import (
	"context"
	"github.com/gorilla/mux"
	"github.com/iota-agency/iota-erp/internal/modules"
	"github.com/iota-agency/iota-erp/pkg/composables"
	"github.com/iota-agency/iota-erp/pkg/constants"
	"net/http"
)

func NavItems() mux.MiddlewareFunc {
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
				items := modules.GetNavItems(localizer, user)
				ctx := context.WithValue(r.Context(), constants.NavItemsKey, items)
				next.ServeHTTP(w, r.WithContext(ctx))
			},
		)
	}
}
