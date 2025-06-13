package middleware

import (
	"net/http"

	"github.com/gorilla/mux"
	"github.com/iota-uz/iota-sdk/pkg/composables"
)

// WithTransaction is deprecated and will be removed in the future.
func WithTransaction() mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			pool, err := composables.UsePool(r.Context())
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			tx, err := pool.Begin(r.Context())
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			defer func() {
				if err := tx.Rollback(r.Context()); err != nil {
					logger := composables.UseLogger(r.Context())
					logger.WithError(err).Error("failed to rollback transaction")
				}
			}()
			r = r.WithContext(composables.WithTx(r.Context(), tx))
			next.ServeHTTP(w, r)
			if err := tx.Commit(r.Context()); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
		})
	}
}
