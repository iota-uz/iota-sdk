package middleware

import (
	"context"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/iota-agency/iota-sdk/pkg/constants"
)

func LogoInContext(logoURL string, faviconURL string) mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {
				ctx := r.Context()
				ctx = context.WithValue(ctx, constants.FaviconKey, faviconURL)
				ctx = context.WithValue(ctx, constants.LogoKey, logoURL)
				next.ServeHTTP(w, r.WithContext(ctx))
			},
		)
	}
}
