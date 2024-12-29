package controllers

import (
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/middleware"
	"net/http"

	"github.com/a-h/templ"
	"github.com/iota-uz/iota-sdk/modules/core/presentation/templates/pages/error_pages"
)

func NotFound(app application.Application) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		handler := templ.Handler(error_pages.NotFoundContent(), templ.WithStreaming())
		middleware.WithLocalizer(app.Bundle())(handler).ServeHTTP(w, r)
	}
}

func MethodNotAllowed() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}
