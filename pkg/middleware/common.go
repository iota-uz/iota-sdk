package middleware

import (
	"github.com/gorilla/mux"
	model "github.com/iota-agency/iota-erp/graph/gqlmodels"
	"github.com/iota-agency/iota-erp/sdk/composables"
	"net/http"
)

func PermissionCheck() mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, ok := composables.UseSession[model.Session](r.Context())
			if !ok {
				next.ServeHTTP(w, r)
				return
			}
		})
	}
}
