// Package controllers provides this package.
package controllers

import (
	"errors"
	"net/http"
	"slices"
	"time"

	"github.com/a-h/templ"
	"github.com/gorilla/mux"
	icons "github.com/iota-uz/icons/phosphor"
	"github.com/iota-uz/iota-sdk/components/scaffold/filterbuilder"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/permission"
	"github.com/iota-uz/iota-sdk/modules/core/presentation/templates/pages/error_pages"
	"github.com/iota-uz/iota-sdk/modules/logging/domain/aggregates/businessevent"
	"github.com/iota-uz/iota-sdk/modules/logging/history"
	"github.com/iota-uz/iota-sdk/modules/logging/permissions"
	historypage "github.com/iota-uz/iota-sdk/modules/logging/presentation/templates/pages/history"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/filterq"
	"github.com/iota-uz/iota-sdk/pkg/intl"
	"github.com/iota-uz/iota-sdk/pkg/middleware"
	"golang.org/x/text/collate"
	"golang.org/x/text/language"
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
	registry := c.filterRegistry(r)
	filters := historypage.ParseFilters(r.URL.Query(), registry.Schema())
	props := &historypage.PageProps{
		BasePath:  c.basePath,
		EventsURL: c.basePath + "/events?" + r.URL.RawQuery,
		Filters:   filters,
		Scope:     c.scopeTitle(r, filters),
		Builder:   filterbuilder.Props{Registry: registry, Filters: filters.Conditions, ID: "history-filters", Class: nil, ShowResetAlways: false},
	}
	templ.Handler(historypage.Index(props), templ.WithStreaming()).ServeHTTP(w, r)
}

// Events renders the timeline. It only reads, so the error state may retry it freely.
func (c *HistoryController) Events(w http.ResponseWriter, r *http.Request) {
	filters := historypage.ParseFilters(r.URL.Query(), c.filterRegistry(r).Schema())
	props := &historypage.EventsProps{BasePath: c.basePath, RetryURL: r.URL.RequestURI(), Filters: filters}
	loc, _ := composables.UseLocation(r.Context())
	result, err := c.service.History(r.Context(), filters.Query(loc, time.Now()))
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

// filterRegistry lists the chip builder's fields. A page pinned to one object has no resource field.
func (c *HistoryController) filterRegistry(r *http.Request) *filterbuilder.Registry {
	t := func(key string) string { return intl.MustT(r.Context(), key) }
	is := []filterq.Operator{filterq.OpIs}
	fields := []filterbuilder.FieldDef{{
		Key:       historypage.FieldPeriod,
		Type:      filterq.FieldTypeDate,
		Label:     t("BusinessHistory.Filters.Period"),
		Operators: []filterq.Operator{filterq.OpBetween, filterq.OpOn, filterq.OpAfter, filterq.OpBefore},
		Presets:   []filterq.DatePreset{filterq.PresetThisMonth, filterq.PresetLastMonth, filterq.PresetLast30D, filterq.PresetThisYear, filterq.PresetLastYear},
	}}
	if r.URL.Query().Get("id") == "" {
		fields = append(fields, filterbuilder.FieldDef{
			Key: historypage.FieldResource, Type: filterq.FieldTypeReference, Label: t("BusinessHistory.Filters.Resource"),
			Operators: is, Options: c.resourceOptions(r),
		})
	}
	return filterbuilder.NewRegistry(append(fields,
		filterbuilder.FieldDef{
			Key: historypage.FieldEventType, Type: filterq.FieldTypeReference, Label: t("BusinessHistory.Filters.Action"),
			Operators: is, Options: c.actionOptions(r),
		},
		filterbuilder.FieldDef{
			Key: historypage.FieldChannel, Type: filterq.FieldTypeReference, Label: t("BusinessHistory.Filters.Channel"),
			Operators: is, Options: c.channelOptions(r),
		},
		filterbuilder.FieldDef{
			Key: historypage.FieldOutcome, Type: filterq.FieldTypeReference, Label: t("BusinessHistory.Filters.Outcome"),
			Operators: is, Options: c.outcomeOptions(r),
		},
	)...)
}

func (c *HistoryController) scopeTitle(r *http.Request, f historypage.Filters) string {
	ctx := r.Context()
	if f.ID == "" {
		return intl.MustT(ctx, "BusinessHistory.Meta.Title")
	}
	return c.resourceLabel(r, f.Resource) + " #" + f.ID
}

func (c *HistoryController) resourceLabel(r *http.Request, name string) string {
	for _, res := range c.registry.Resources() {
		if res.Name == name && res.LabelKey != "" {
			return intl.TWithDefault(r.Context(), res.LabelKey, name)
		}
	}
	return name
}

func (c *HistoryController) resourceOptions(r *http.Request) []filterbuilder.Option {
	resources := c.registry.Resources()
	options := make([]filterbuilder.Option, 0, len(resources))
	for _, res := range resources {
		if c.service.Visible(r.Context(), res.Name) {
			options = append(options, filterbuilder.Opt(res.Name, c.resourceLabel(r, res.Name)))
		}
	}
	c.sortOptions(r, options)
	return options
}

// actionOptions lists the event types, grouped under the resource they are about.
func (c *HistoryController) actionOptions(r *http.Request) []filterbuilder.Option {
	ctx := r.Context()
	other := intl.MustT(ctx, "BusinessHistory.Filters.OtherActions")
	seen := map[string]bool{}
	definitions := c.registry.Definitions()
	options := make([]filterbuilder.Option, 0, len(definitions))
	for _, d := range definitions {
		if seen[d.Type] || d.Category == businessevent.CategoryLegacy {
			continue
		}
		seen[d.Type] = true
		group := other
		if len(d.Resources) > 0 {
			i := slices.IndexFunc(d.Resources, func(res history.Resource) bool { return c.service.Visible(ctx, res.Name) })
			if i < 0 {
				continue
			}
			group = c.resourceLabel(r, d.Resources[i].Name)
		}
		option := filterbuilder.Opt(d.Type, intl.TWithDefault(ctx, d.TitleKey, d.Type))
		option.Group = group
		options = append(options, option)
	}
	collator := c.collator(r)
	slices.SortStableFunc(options, func(a, b filterbuilder.Option) int {
		if a.Group != b.Group {
			return collator.CompareString(a.Group, b.Group)
		}
		return collator.CompareString(a.Label, b.Label)
	})
	return options
}

func (c *HistoryController) channelOptions(r *http.Request) []filterbuilder.Option {
	options := make([]filterbuilder.Option, len(historypage.Channels))
	for i, channel := range historypage.Channels {
		options[i] = filterbuilder.Opt(channel, intl.MustT(r.Context(), "BusinessHistory.Channels."+channel))
	}
	return options
}

func (c *HistoryController) outcomeOptions(r *http.Request) []filterbuilder.Option {
	options := make([]filterbuilder.Option, len(historypage.Outcomes))
	for i, outcome := range historypage.Outcomes {
		options[i] = filterbuilder.Opt(string(outcome), intl.MustT(r.Context(), "BusinessHistory.Outcomes."+string(outcome)))
	}
	return options
}

func (c *HistoryController) sortOptions(r *http.Request, options []filterbuilder.Option) {
	collator := c.collator(r)
	slices.SortStableFunc(options, func(a, b filterbuilder.Option) int { return collator.CompareString(a.Label, b.Label) })
}

func (c *HistoryController) collator(r *http.Request) *collate.Collator {
	tag := language.Russian
	if locale, ok := intl.UseLocale(r.Context()); ok {
		tag = locale
	}
	return collate.New(tag)
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
