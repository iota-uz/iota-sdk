package controllers

import (
	"net/http"

	"github.com/iota-uz/iota-sdk/modules/core/presentation/templates/pages/error_pages"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/middleware"
)

func handler404(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusNotFound)
	error_pages.NotFoundContent().Render(r.Context(), w)
}

func NotFound(app application.Application) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		handler := middleware.WithPageContext()(http.HandlerFunc(handler404))
		handler = middleware.WithLocalizer(app.Bundle())(handler)
		handler.ServeHTTP(w, r)
	}
}

func MethodNotAllowed() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}
