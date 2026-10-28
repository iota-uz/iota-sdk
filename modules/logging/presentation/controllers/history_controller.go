// Package controllers provides this package.
package controllers

import (
	"errors"
	"net/http"
	"net/url"

	"github.com/a-h/templ"
	"github.com/gorilla/mux"
	icons "github.com/iota-uz/icons/phosphor"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/permission"
	"github.com/iota-uz/iota-sdk/modules/core/presentation/templates/pages/error_pages"
	"github.com/iota-uz/iota-sdk/modules/logging/domain/aggregates/businessevent"
	"github.com/iota-uz/iota-sdk/modules/logging/history"
	"github.com/iota-uz/iota-sdk/modules/logging/permissions"
	historypage "github.com/iota-uz/iota-sdk/modules/logging/presentation/templates/pages/history"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/intl"
	"github.com/iota-uz/iota-sdk/pkg/middleware"
)

const DefaultHistoryBasePath = "/admin/history"

type HistoryController struct {
	basePath string
	service  *history.Service
	registry *history.Registry
}

func NewHistoryController(basePath string, service *history.Service, registry *history.Registry) application.Controller {
	if basePath == "" {
		basePath = DefaultHistoryBasePath
	}
	return &HistoryController{basePath: basePath, service: service, registry: registry}
}

func (c *HistoryController) Descriptor() application.ControllerDescriptor {
	return application.Descriptor("logging.history", 0, application.Route("", c.basePath)).
		WithNav(application.NavNode{
			ID:         "logging.history",
			Parent:     "core.administration",
			TitleKey:   "NavigationLinks.BusinessHistory",
			Path:       c.basePath,
			Icon:       icons.ClockCounterClockwise(icons.Props{Size: "20"}),
			Order:      65,
			Visibility: &application.AuthPolicy{Permissions: []permission.Permission{permissions.ViewLogs}},
		})
}

func (c *HistoryController) Register(r *mux.Router) {
	router := r.PathPrefix(c.basePath).Subrouter()
	router.Use(
		middleware.Authorize(),
		middleware.RedirectNotAuthenticated(),
		middleware.ProvideUser(),
		middleware.NavItems(),
		middleware.WithPageContext(),
		middleware.ProvideDynamicLogo(),
	)
	router.HandleFunc("", c.Index).Methods(http.MethodGet)
	router.HandleFunc("/events", c.Events).Methods(http.MethodGet)
}

func (c *HistoryController) Index(w http.ResponseWriter, r *http.Request) {
	if err := composables.CanUser(r.Context(), permissions.ViewLogs); err != nil {
		w.WriteHeader(http.StatusForbidden)
		templ.Handler(error_pages.Forbidden()).ServeHTTP(w, r)
		return
	}
	filters := parseFilters(r.URL.Query())
	props := &historypage.PageProps{
		BasePath:  c.basePath,
		EventsURL: c.basePath + "/events?" + r.URL.RawQuery,
		Filters:   filters,
		Scope:     c.scopeTitle(r, filters),
		Resources: c.resourceOptions(r),
		Types:     c.typeOptions(r),
	}
	templ.Handler(historypage.Index(props), templ.WithStreaming()).ServeHTTP(w, r)
}

// Events renders the timeline. It only reads, so the error state may retry it freely.
func (c *HistoryController) Events(w http.ResponseWriter, r *http.Request) {
	filters := parseFilters(r.URL.Query())
	props := &historypage.EventsProps{BasePath: c.basePath, RetryURL: r.URL.RequestURI(), Filters: filters}
	result, err := c.service.History(r.Context(), filters.Query())
	props.Groups = historypage.Group(result.Items, c.fieldLabels(r))
	if result.NextCursor != nil {
		next := r.URL.Query()
		next.Set("cursor", *result.NextCursor)
		props.NextURL = c.basePath + "?" + next.Encode()
	}
	switch {
	case errors.Is(err, history.ErrForbidden):
		props.State = historypage.StateDenied
	case errors.Is(err, history.ErrInvalidInput):
		props.State = historypage.StateInvalid
	case err != nil:
		composables.UseLogger(r.Context()).WithError(err).Error("failed to read business history")
		props.State = historypage.StateError
	case len(props.Groups) == 0:
		props.State = historypage.StateEmpty
	}
	templ.Handler(historypage.Events(props)).ServeHTTP(w, r)
}

func parseFilters(values url.Values) historypage.Filters {
	return historypage.Filters{
		Resource:  values.Get("resource"),
		ID:        values.Get("id"),
		Actor:     values.Get("actor"),
		ActorName: values.Get("actor_name"),
		EventType: values.Get("event_type"),
		Channel:   values.Get("channel"),
		Outcome:   values.Get("outcome"),
		From:      values.Get("from"),
		To:        values.Get("to"),
		Cursor:    values.Get("cursor"),
	}
}

func (c *HistoryController) scopeTitle(r *http.Request, f historypage.Filters) string {
	ctx := r.Context()
	if f.Resource == "" {
		return intl.MustT(ctx, "BusinessHistory.Meta.Title")
	}
	label := c.resourceLabel(r, f.Resource)
	if f.ID != "" {
		return label + " #" + f.ID
	}
	return label
}

func (c *HistoryController) resourceLabel(r *http.Request, name string) string {
	for _, res := range c.registry.Resources() {
		if res.Name == name && res.LabelKey != "" {
			return intl.TWithDefault(r.Context(), res.LabelKey, name)
		}
	}
	return name
}

func (c *HistoryController) resourceOptions(r *http.Request) []historypage.Option {
	resources := c.registry.Resources()
	options := make([]historypage.Option, 0, len(resources))
	for _, res := range resources {
		if c.service.Visible(r.Context(), res.Name) {
			options = append(options, historypage.Option{Value: res.Name, Label: c.resourceLabel(r, res.Name)})
		}
	}
	return options
}

func (c *HistoryController) typeOptions(r *http.Request) []historypage.Option {
	seen := map[string]bool{}
	var options []historypage.Option
	for _, d := range c.registry.Definitions() {
		if seen[d.Type] || d.Category == businessevent.CategoryLegacy {
			continue
		}
		seen[d.Type] = true
		options = append(options, historypage.Option{Value: d.Type, Label: intl.TWithDefault(r.Context(), d.TitleKey, d.Type)})
	}
	return options
}

// fieldLabels resolves the localized field label of a change, keyed by event type, version, resource and field.
func (c *HistoryController) fieldLabels(r *http.Request) func(item history.Item, resource, field string) string {
	return func(item history.Item, resource, field string) string {
		def, ok := c.registry.Get(item.EventType, item.SchemaVersion)
		if !ok {
			return field
		}
		for _, res := range def.Resources {
			if res.Name != resource {
				continue
			}
			for _, f := range res.Fields {
				if f.Name == field {
					return intl.TWithDefault(r.Context(), f.LabelKey, field)
				}
			}
		}
		return field
	}
}
