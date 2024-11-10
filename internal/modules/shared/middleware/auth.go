package middleware

import (
	"github.com/gorilla/mux"
	"github.com/iota-agency/iota-erp/pkg/composables"
	"net/http"
)

func RequireAuthorization() mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {
				params, ok := composables.UseParams(r.Context())
				if !ok {
					panic("params not found. Add RequestParams middleware up the chain")
				}
				if !params.Authenticated {
					http.Redirect(w, r, "/login", http.StatusFound)
					return
				}
				next.ServeHTTP(w, r)
			},
		)
	}
}
