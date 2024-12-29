package middleware

import (
	"context"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/iota-agency/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/constants"
	"gorm.io/gorm"
)

func WithTransaction() mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {
				pool, err := composables.UsePool(r.Context())
				if err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
				}
				db := r.Context().Value(constants.DBKey).(*gorm.DB)
				tx, err := pool.Begin(r.Context())
				if err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
				}
				defer tx.Rollback(r.Context())
				r = r.WithContext(composables.WithPoolTx(r.Context(), tx))
				err = db.Transaction(func(tx *gorm.DB) error {
					next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), constants.TxKey, tx)))
					return nil
				})
				if err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
				}
			},
		)
	}
}
