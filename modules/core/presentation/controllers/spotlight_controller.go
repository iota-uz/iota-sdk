package controllers

import (
	"fmt"
	"net/http"

	"github.com/a-h/templ"
	"github.com/gorilla/mux"
	spotlightui "github.com/iota-uz/iota-sdk/components/spotlight"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/middleware"
	"github.com/iota-uz/iota-sdk/pkg/spotlight"
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
		middleware.ProvideLocalizer(c.app),
	)
	router.HandleFunc("/search", c.Get).Methods(http.MethodGet)
}

func (c *SpotlightController) Get(w http.ResponseWriter, r *http.Request) {
	// Prevent caching of dynamic search results
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Expires", "0")

	q := r.URL.Query().Get("q")
	logger := composables.UseLogger(r.Context())
	if q == "" {
		templ.Handler(
			spotlightui.NotFound(),
			templ.WithStreaming(),
			templ.WithErrorHandler(errorHandler),
		).ServeHTTP(w, r)
		return
	}

	tenantID, err := composables.UseTenantID(r.Context())
	if err != nil {
		logger.WithError(err).WithField("query", q).Warn("spotlight tenant resolution failed")
		templ.Handler(
			spotlightui.NotFound(),
			templ.WithStreaming(),
			templ.WithErrorHandler(errorHandler),
		).ServeHTTP(w, r)
		return
	}

	userID := ""
	roles := make([]string, 0, 8)
	permissions := make([]string, 0, 32)
	if user, userErr := composables.UseUser(r.Context()); userErr == nil {
		userID = fmt.Sprintf("%d", user.ID())
		for _, role := range user.Roles() {
			roles = append(roles, role.Name())
		}
		for _, permission := range user.Permissions() {
			permissions = append(permissions, permission.Name())
		}
	}

	intent := spotlight.SearchIntentMixed
	if spotlight.IsHowQuery(q) {
		intent = spotlight.SearchIntentHelp
	}

	resp, err := c.app.Spotlight().Search(r.Context(), spotlight.SearchRequest{
		Query:       q,
		TenantID:    tenantID,
		UserID:      userID,
		Roles:       roles,
		Permissions: permissions,
		TopK:        30,
		Intent:      intent,
	})
	if err != nil {
		logger.WithError(err).WithField("query", q).Error("spotlight search failed")
		http.Error(w, "Failed to search Spotlight", http.StatusInternalServerError)
		return
	}
	view := spotlight.ToViewResponse(resp)

	items := make([]templ.Component, 0, 64)
	index := 0

	appendGroup := func(title string, hits []spotlight.SearchHit) {
		if len(hits) == 0 {
			return
		}
		groupItems := make([]templ.Component, 0, len(hits))
		for _, hit := range hits {
			groupItems = append(groupItems, spotlight.HitToComponent(hit))
		}
		items = append(items, spotlight.GroupToComponent(title, groupItems, index))
		index += len(groupItems)
	}

	for _, group := range view.Groups {
		appendGroup(group.Title, group.Hits)
	}

	if view.Agent != nil {
		actionComponents := make([]templ.Component, 0, len(view.Agent.Actions))
		for _, action := range view.Agent.Actions {
			actionComponents = append(actionComponents, spotlight.ActionToComponent(action))
		}
		items = append(items, spotlightui.AIAnswer(view.Agent.Summary, actionComponents))
	}

	if len(items) == 0 {
		templ.Handler(
			spotlightui.NotFound(),
			templ.WithStreaming(),
			templ.WithErrorHandler(errorHandler),
		).ServeHTTP(w, r)
		return
	}

	templ.Handler(
		spotlightui.SpotlightItems(items, 0),
		templ.WithStreaming(),
		templ.WithErrorHandler(errorHandler),
	).ServeHTTP(w, r)

}
