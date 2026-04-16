// Package appletutil provides shared helpers for SDK-backed applets.
package appletutil

import (
	"net/http"

	"github.com/gorilla/mux"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/configuration"
	"github.com/iota-uz/iota-sdk/pkg/middleware"
)

// ProvideLocalizerFromAppContext extracts the application handle from the request
// context and installs the standard localizer middleware using the app bundle and
// supported languages.
func ProvideLocalizerFromAppContext() mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			app, err := application.UseApp(r.Context())
			if err != nil {
				configuration.Use().Logger().
					WithError(err).
					Error("applet localizer middleware missing app in request context")
				http.Error(w, "Internal server error", http.StatusInternalServerError)
				return
			}

			localizerMiddleware := middleware.ProvideLocalizer(app.Bundle(), app.GetSupportedLanguages())
			localizerMiddleware(next).ServeHTTP(w, r)
		})
	}
}
