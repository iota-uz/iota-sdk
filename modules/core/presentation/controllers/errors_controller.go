// Package controllers provides this package.
package controllers

import (
	"net/http"

	"github.com/gorilla/mux"
	"github.com/iota-uz/iota-sdk/modules/core/presentation/templates/pages/error_pages"
	"github.com/iota-uz/iota-sdk/pkg/middleware"
)

// RenderForbidden is a helper function that can be used directly in controllers
// to render the 403 forbidden page when permission checks fail.
// When called inside an authenticated route (with sidebar context available),
// it renders the error inside the authenticated layout with sidebar and navbar.
// Otherwise it falls back to a standalone page.
func RenderForbidden(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusForbidden)
	if err := error_pages.Forbidden().Render(r.Context(), w); err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

func handler404(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusNotFound)
	if err := error_pages.NotFound().Render(r.Context(), w); err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// NotFound returns the 404 handler. ProvideLocalizer is installed as a
// global middleware in pkg/server/builder.go. The handler resolves the session
// softly: signed-in users get the 404 page inside the authenticated layout
// (sidebar and navbar), anonymous visitors get the standalone page.
func NotFound() http.HandlerFunc {
	chain := []mux.MiddlewareFunc{
		middleware.Authorize(),
		middleware.ProvideUser(),
		middleware.ProvideDynamicLogo(),
		middleware.NavItems(),
		middleware.WithPageContext(),
	}
	var handler http.Handler = http.HandlerFunc(handler404)
	for i := len(chain) - 1; i >= 0; i-- {
		handler = chain[i](handler)
	}
	return handler.ServeHTTP
}

func MethodNotAllowed() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}
