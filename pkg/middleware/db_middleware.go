package middleware

import (
	"context"
	"github.com/gorilla/mux"
	"github.com/iota-agency/iota-sdk/pkg/constants"
	"gorm.io/gorm"
	"net/http"
)

func WithTransaction() mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {
				db := r.Context().Value(constants.DBKey).(*gorm.DB)
				err := db.Transaction(func(tx *gorm.DB) error {
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
