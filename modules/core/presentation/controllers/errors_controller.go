// Package controllers provides this package.
package controllers

import (
	"net/http"

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
	if err := error_pages.NotFoundContent().Render(r.Context(), w); err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// NotFound returns the 404 handler. ProvideLocalizer is now installed as a
// global middleware in pkg/server/builder.go, so this only needs to attach
// the page-context middleware before invoking the renderer.
func NotFound() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		handler := middleware.WithPageContext()(http.HandlerFunc(handler404))
		handler.ServeHTTP(w, r)
	}
}

func MethodNotAllowed() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}
