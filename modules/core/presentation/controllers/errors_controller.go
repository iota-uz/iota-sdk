package controllers

import (
	"net/http"

	"github.com/iota-uz/iota-sdk/modules/core/presentation/templates/pages/error_pages"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/middleware"
)

// RenderForbidden is a helper function that can be used directly in controllers
// to render the 403 forbidden page when permission checks fail
func RenderForbidden(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusForbidden)
	if err := error_pages.ForbiddenContent().Render(r.Context(), w); err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

func handler404(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusNotFound)
	if err := error_pages.NotFoundContent().Render(r.Context(), w); err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

func NotFound(app application.Application) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		handler := middleware.WithPageContext()(http.HandlerFunc(handler404))
		handler = middleware.ProvideLocalizer(app.Bundle())(handler)
		handler.ServeHTTP(w, r)
	}
}

func MethodNotAllowed() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}
