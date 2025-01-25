package controllers

import (
	"github.com/a-h/templ"
	"github.com/gorilla/mux"
	spotlightui "github.com/iota-uz/iota-sdk/components/spotlight"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/middleware"
	"net/http"
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

func (c *SpotlightController) Key() string {
	return c.basePath
}

func (c *SpotlightController) Register(r *mux.Router) {
	router := r.PathPrefix(c.basePath).Subrouter()
	router.Use(
		middleware.Authorize(),
		middleware.ProvideUser(),
		middleware.RedirectNotAuthenticated(),
		middleware.WithLocalizer(c.app.Bundle()),
	)
	router.HandleFunc("/search", c.Get).Methods(http.MethodGet)
}

func (c *SpotlightController) Get(w http.ResponseWriter, r *http.Request) {
	localizer, ok := composables.UseLocalizer(r.Context())
	if !ok {
		http.Error(w, composables.ErrNoLocalizer.Error(), http.StatusInternalServerError)
		return
	}
	q := r.URL.Query().Get("q")
	if q == "" {
		templ.Handler(spotlightui.SpotlightItems([]*spotlightui.Item{})).ServeHTTP(w, r)
		return
	}

	results := c.app.Spotlight().Find(localizer, q)
	items := make([]*spotlightui.Item, len(results))
	for i, result := range results {
		items[i] = &spotlightui.Item{
			Title: result.Localized(localizer),
			Icon:  result.Icon(),
			Link:  result.Link(),
		}
	}

	templ.Handler(spotlightui.SpotlightItems(items)).ServeHTTP(w, r)
}
