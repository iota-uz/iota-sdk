package middleware

import (
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/iota-uz/iota-sdk/pkg/composables"
)

// ProvideLocation attaches the application time zone to the request context so
// that server-rendered timestamps can be shown in it. A nil location is a no-op.
func ProvideLocation(loc *time.Location) mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		if loc == nil {
			return next
		}
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			next.ServeHTTP(w, r.WithContext(composables.WithLocation(r.Context(), loc)))
		})
	}
}
