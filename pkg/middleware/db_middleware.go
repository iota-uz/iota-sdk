package middleware

import (
	"net/http"

	"github.com/gorilla/mux"
	"github.com/iota-uz/iota-sdk/pkg/composables"
)

func WithTransaction() mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {
				pool, err := composables.UsePool(r.Context())
				if err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
				}
				tx, err := pool.Begin(r.Context())
				if err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
				}
				defer tx.Rollback(r.Context())
				r = r.WithContext(composables.WithTx(r.Context(), tx))
				if err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
				}
				next.ServeHTTP(w, r)
				if err := tx.Commit(r.Context()); err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
				}
			},
		)
	}
}
