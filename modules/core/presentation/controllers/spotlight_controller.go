package controllers

import (
	"github.com/a-h/templ"
	"github.com/gorilla/mux"
	spotlightui "github.com/iota-agency/iota-sdk/components/spotlight"
	"github.com/iota-agency/iota-sdk/modules/core/services"
	"github.com/iota-agency/iota-sdk/pkg/application"
	"github.com/iota-agency/iota-sdk/pkg/composables"
	"github.com/iota-agency/iota-sdk/pkg/middleware"
	"github.com/iota-agency/iota-sdk/pkg/types"
	"net/http"
)

func flatNavItems(items []types.NavigationItem) []types.NavigationItem {
	var flatItems []types.NavigationItem
	for _, item := range items {
		flatItems = append(flatItems, item)
		if item.Children != nil {
			for _, child := range flatNavItems(item.Children) {
				flatItems = append(flatItems, types.NavigationItem{
					Name:     child.Name,
					Href:     child.Href,
					Icon:     item.Icon,
					Children: child.Children,
				})
			}
		}
	}
	return flatItems
}

type SpotlightController struct {
	app        application.Application
	tabService *services.TabService
	basePath   string
}

func NewSpotlightController(app application.Application) application.Controller {
	return &SpotlightController{
		app:        app,
		tabService: app.Service(services.TabService{}).(*services.TabService),
		basePath:   "/spotlight",
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
