// Package appletutil provides shared helpers for SDK-backed applets.
package appletutil

import (
	"net/http"
	"sync"

	"github.com/gorilla/mux"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/middleware"
	"github.com/sirupsen/logrus"
)

// ProvideLocalizerFromAppContext extracts the application handle from the request
// context and installs the standard localizer middleware using the app bundle and
// supported languages.
//
// The wrapped handler is built lazily on first request and cached per-app so we
// don't reconstruct the middleware (and re-resolve the bundle/languages) on
// every hit. Different apps (rare in practice) get separate cache entries.
func ProvideLocalizerFromAppContext() mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		var cache sync.Map // application.Application -> http.Handler
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			app, err := application.UseApp(r.Context())
			if err != nil {
				logrus.StandardLogger().
					WithError(err).
					Error("applet localizer middleware missing app in request context")
				http.Error(w, "Internal server error", http.StatusInternalServerError)
				return
			}

			if cached, ok := cache.Load(app); ok {
				cached.(http.Handler).ServeHTTP(w, r)
				return
			}
			handler := middleware.ProvideLocalizer(app.Bundle(), app.GetSupportedLanguages())(next)
			actual, _ := cache.LoadOrStore(app, handler)
			actual.(http.Handler).ServeHTTP(w, r)
		})
	}
}
