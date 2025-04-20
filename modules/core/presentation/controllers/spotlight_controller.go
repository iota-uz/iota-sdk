package controllers

import (
	"net/http"

	"github.com/a-h/templ"
	"github.com/gorilla/mux"
	spotlightui "github.com/iota-uz/iota-sdk/components/spotlight"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/middleware"
)

type SpotlightController struct {
	app      application.Application
	basePath string
}

func NewSpotlightController(app application.Application) application.Controller {
	return &SpotlightController{
		app:      app,
		basePath: "/spotlight",
	}
}

// errorHandler returns a 500 response if templ rendering fails.
var errorHandler = func(r *http.Request, err error) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r2 *http.Request) {
		http.Error(w, "Failed to render Spotlight results", http.StatusInternalServerError)
	})
}

func (c *SpotlightController) Key() string {
	return c.basePath
}

func (c *SpotlightController) Register(r *mux.Router) {
	router := r.PathPrefix(c.basePath).Subrouter()
	router.Use(
		middleware.Authorize(),
		middleware.ProvideUser(),
		middleware.RedirectNotAuthenticated(),
		middleware.ProvideLocalizer(c.app.Bundle()),
	)
	router.HandleFunc("/search", c.Get).Methods(http.MethodGet)
}

func (c *SpotlightController) Get(w http.ResponseWriter, r *http.Request) {
	// Prevent caching of dynamic search results
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Expires", "0")

	q := r.URL.Query().Get("q")
	if q == "" {
		templ.Handler(
			spotlightui.NotFound(),
			templ.WithStreaming(),
			templ.WithErrorHandler(errorHandler),
		).ServeHTTP(w, r)
		return
	}

	items := make([]templ.Component, 0, 10)
	for item := range c.app.Spotlight().Find(r.Context(), q) {
		items = append(items, item)
	}

	templ.Handler(spotlightui.SpotlightItems(items, 0)).ServeHTTP(w, r)

	// TODO: Enable for streaming. Does not work properly yet
	//	i := 0
	//	for item := range c.app.Spotlight().Find(r.Context(), q) {
	//		ctx := templ.WithChildren(r.Context(), item)
	//		spotlightui.SpotlightItem(i).Render(ctx, w)
	//		w.(http.Flusher).Flush()
	//		i++
	//	}
	//	if i == 0 {
	//		templ.Handler(
	//			spotlightui.NotFound(),
	//			templ.WithStreaming(),
	//			templ.WithErrorHandler(errorHandler),
	//		).ServeHTTP(w, r)
	//		return
	//	}

	// closeNotify := w.(http.CloseNotifier).CloseNotify()
	// <-closeNotify
}
