//go:build dev

package controllers

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/a-h/templ"
	"github.com/gorilla/mux"
	icons "github.com/iota-uz/icons/phosphor"
	"github.com/sirupsen/logrus"

	"github.com/iota-uz/iota-sdk/components/sidebar"
	"github.com/iota-uz/iota-sdk/modules/core/presentation/templates/pages/error_pages"
	showcase "github.com/iota-uz/iota-sdk/modules/core/presentation/templates/pages/showcase"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/configuration"
	"github.com/iota-uz/iota-sdk/pkg/di"
	"github.com/iota-uz/iota-sdk/pkg/htmx"
	"github.com/iota-uz/iota-sdk/pkg/lens"
	"github.com/iota-uz/iota-sdk/pkg/lens/datasource"
	"github.com/iota-uz/iota-sdk/pkg/lens/panel"
	lenspostgres "github.com/iota-uz/iota-sdk/pkg/lens/postgres"
	"github.com/iota-uz/iota-sdk/pkg/lens/runtime"
	"github.com/iota-uz/iota-sdk/pkg/middleware"
)

type ShowcaseController struct {
	app      application.Application
	basePath string
	ds       datasource.DataSource
}

func NewShowcaseController(app application.Application) application.Controller {
	config := configuration.Use()
	ds, err := lenspostgres.New(lenspostgres.Config{
		ConnectionString: config.Database.ConnectionString(),
		MaxConnections:   5,
		MinConnections:   1,
		QueryTimeout:     30 * time.Second,
	})
	if err != nil {
		log.Printf("Failed to create lens data source for showcase: %v", err)
		return &ShowcaseController{app: app, basePath: "/_dev"}
	}
	return &ShowcaseController{app: app, basePath: "/_dev", ds: ds}
}

func (c *ShowcaseController) Key() string {
	return c.basePath
}

func (c *ShowcaseController) Register(r *mux.Router) {
	router := r.PathPrefix(c.basePath).Subrouter()
	router.Use(
		middleware.ProvideUser(),
		middleware.ProvideDynamicLogo(c.app),
		middleware.ProvideLocalizer(c.app),
		middleware.NavItems(),
		middleware.WithPageContext(),
	)
	router.HandleFunc("", di.H(c.Overview)).Methods(http.MethodGet)
	router.HandleFunc("/components/form", di.H(c.Form)).Methods(http.MethodGet)
	router.HandleFunc("/components/other", di.H(c.Other)).Methods(http.MethodGet)
	router.HandleFunc("/components/kanban", di.H(c.Kanban)).Methods(http.MethodGet)
	router.HandleFunc("/components/loaders", di.H(c.Loaders)).Methods(http.MethodGet)
	router.HandleFunc("/components/charts", di.H(c.Charts)).Methods(http.MethodGet)
	router.HandleFunc("/components/tooltips", di.H(c.Tooltips)).Methods(http.MethodGet)
	router.HandleFunc("/components/subscription", di.H(c.Subscription)).Methods(http.MethodGet)
	router.HandleFunc("/lens", di.H(c.Lens)).Methods(http.MethodGet)
	router.HandleFunc("/error-pages/403", di.H(c.Error403Page)).Methods(http.MethodGet)
	router.HandleFunc("/error-pages/404", di.H(c.Error404Page)).Methods(http.MethodGet)
	router.HandleFunc("/error-preview/403", di.H(c.Error403Preview)).Methods(http.MethodGet)
	router.HandleFunc("/error-preview/404", di.H(c.Error404Preview)).Methods(http.MethodGet)
	router.HandleFunc("/api/showcase/toast-example", di.H(c.ToastExample)).Methods(http.MethodPost)

	log.Printf(
		"See %s%s for docs\n",
		configuration.Use().Origin,
		c.basePath,
	)
}

func (c *ShowcaseController) getSidebarProps() sidebar.Props {
	items := []sidebar.Item{
		sidebar.NewLink(c.basePath, "Overview", nil),
		sidebar.NewLink(fmt.Sprintf("%s/lens", c.basePath), "Lens Dashboard", icons.MagnifyingGlass(icons.Props{Size: "20"})),
		sidebar.NewLink(fmt.Sprintf("%s/crud", c.basePath), "Crud", icons.Buildings(icons.Props{Size: "20"})),
		sidebar.NewGroup(
			"Components",
			icons.PuzzlePiece(icons.Props{Size: "20"}),
			[]sidebar.Item{
				sidebar.NewLink(fmt.Sprintf("%s/components/form", c.basePath), "Form", nil),
				sidebar.NewLink(fmt.Sprintf("%s/components/loaders", c.basePath), "Loaders", nil),
				sidebar.NewLink(fmt.Sprintf("%s/components/charts", c.basePath), "Charts", nil),
				sidebar.NewLink(fmt.Sprintf("%s/components/tooltips", c.basePath), "Tooltips", nil),
				sidebar.NewLink(fmt.Sprintf("%s/components/subscription", c.basePath), "Subscription", nil),
				sidebar.NewLink(fmt.Sprintf("%s/components/other", c.basePath), "Other", nil),
				sidebar.NewLink(fmt.Sprintf("%s/components/kanban", c.basePath), "Kanban", nil),
			},
		),
		sidebar.NewGroup(
			"Error Pages",
			icons.Warning(icons.Props{Size: "20"}),
			[]sidebar.Item{
				sidebar.NewLink(fmt.Sprintf("%s/error-pages/403", c.basePath), "403 Forbidden", nil),
				sidebar.NewLink(fmt.Sprintf("%s/error-pages/404", c.basePath), "404 Not Found", nil),
			},
		),
	}

	tabGroups := sidebar.TabGroupCollection{
		Groups: []sidebar.TabGroup{
			{
				Label: "Showcase",
				Value: "showcase",
				Items: items,
			},
		},
		DefaultValue: "showcase",
	}

	return sidebar.Props{
		TabGroups: tabGroups,
	}
}

func (c *ShowcaseController) Overview(
	r *http.Request,
	w http.ResponseWriter,
	logger *logrus.Entry,
) {
	props := showcase.IndexPageProps{
		SidebarProps: c.getSidebarProps(),
	}
	templ.Handler(showcase.OverviewPage(props)).ServeHTTP(w, r)
}

func (c *ShowcaseController) Form(
	r *http.Request,
	w http.ResponseWriter,
	logger *logrus.Entry,
) {
	props := showcase.IndexPageProps{
		SidebarProps: c.getSidebarProps(),
	}
	templ.Handler(showcase.FormPage(props)).ServeHTTP(w, r)
}

func (c *ShowcaseController) Other(
	r *http.Request,
	w http.ResponseWriter,
	logger *logrus.Entry,
) {
	props := showcase.IndexPageProps{
		SidebarProps: c.getSidebarProps(),
	}
	templ.Handler(showcase.OtherPage(props)).ServeHTTP(w, r)
}

func (c *ShowcaseController) Kanban(
	r *http.Request,
	w http.ResponseWriter,
	logger *logrus.Entry,
) {
	props := showcase.IndexPageProps{
		SidebarProps: c.getSidebarProps(),
	}
	templ.Handler(showcase.KanbanPage(props)).ServeHTTP(w, r)
}

func (c *ShowcaseController) Loaders(
	r *http.Request,
	w http.ResponseWriter,
	logger *logrus.Entry,
) {
	props := showcase.IndexPageProps{
		SidebarProps: c.getSidebarProps(),
	}
	templ.Handler(showcase.LoadersPage(props)).ServeHTTP(w, r)
}

func (c *ShowcaseController) Charts(
	r *http.Request,
	w http.ResponseWriter,
	logger *logrus.Entry,
) {
	props := showcase.IndexPageProps{
		SidebarProps: c.getSidebarProps(),
	}
	templ.Handler(showcase.ChartsPage(props)).ServeHTTP(w, r)
}

func (c *ShowcaseController) Tooltips(
	r *http.Request,
	w http.ResponseWriter,
	logger *logrus.Entry,
) {
	props := showcase.IndexPageProps{
		SidebarProps: c.getSidebarProps(),
	}
	templ.Handler(showcase.TooltipsPage(props)).ServeHTTP(w, r)
}

func (c *ShowcaseController) Subscription(
	r *http.Request,
	w http.ResponseWriter,
	logger *logrus.Entry,
) {
	props := showcase.IndexPageProps{
		SidebarProps: c.getSidebarProps(),
	}
	templ.Handler(showcase.SubscriptionPage(props)).ServeHTTP(w, r)
}

func (c *ShowcaseController) Lens(
	r *http.Request,
	w http.ResponseWriter,
	logger *logrus.Entry,
) {
	params := tenantParams(r)
	dash := lens.Dashboard("sdk-core-analytics", "IOTA SDK Core Analytics",
		lens.Row(
			panel.TimeSeries("user-registrations", "User Registrations Over Time", "user-registrations").Span(6).Build(),
			panel.Bar("user-languages", "User Interface Languages", "user-languages").Span(6).Build(),
		),
		lens.Row(
			panel.Pie("user-types", "User Type Distribution", "user-types").Legend().Span(4).Build(),
			panel.Gauge("session-activity", "Active Sessions", "session-activity").Span(4).Build(),
			panel.Table("recent-users", "Recently Registered Users", "recent-users").Span(4).Build(),
		),
	).WithDatasets(
		queryDatasetWithParams(
			"user-registrations",
			"SELECT DATE(created_at) as label, COUNT(*)::float8 as value FROM users WHERE tenant_id = @tenant_id AND created_at >= NOW() - INTERVAL '30 days' GROUP BY DATE(created_at) ORDER BY label",
			params,
		),
		queryDatasetWithParams(
			"user-languages",
			"SELECT ui_language as label, COUNT(*)::float8 as value FROM users WHERE tenant_id = @tenant_id GROUP BY ui_language ORDER BY value DESC",
			params,
		),
		queryDatasetWithParams(
			"user-types",
			"SELECT type as label, COUNT(*)::float8 as value FROM users WHERE tenant_id = @tenant_id GROUP BY type",
			params,
		),
		queryDatasetWithParams(
			"session-activity",
			"SELECT COUNT(*)::float8 as value FROM sessions WHERE tenant_id = @tenant_id AND expires_at > NOW()",
			params,
		),
		queryDatasetWithParams(
			"recent-users",
			"SELECT first_name, last_name, email, ui_language, created_at FROM users WHERE tenant_id = @tenant_id ORDER BY created_at DESC LIMIT 10",
			params,
		),
	)

	var results *runtime.DashboardResult
	if params == nil {
		logger.Warn("skipping lens showcase dashboard execution because tenant context is missing")
	} else if c.ds != nil {
		ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
		defer cancel()
		executed, err := runtime.Execute(ctx, dash, runtime.Runtime{
			DataSources: map[string]datasource.DataSource{
				"primary": c.ds,
			},
		})
		if err != nil {
			logger.WithError(err).Error("failed to execute lens showcase dashboard")
		} else {
			results = executed
		}
	}

	props := showcase.LensPageProps{
		SidebarProps: c.getSidebarProps(),
		Dashboard:    dash,
		Results:      results,
	}
	templ.Handler(showcase.LensPage(props)).ServeHTTP(w, r)
}

func tenantParams(r *http.Request) map[string]lens.ParamValue {
	tenantID, err := composables.UseTenantID(r.Context())
	if err != nil {
		return nil
	}
	return map[string]lens.ParamValue{
		"tenant_id": {Literal: tenantID},
	}
}

func queryDatasetWithParams(name, text string, params map[string]lens.ParamValue) lens.DatasetSpec {
	spec := lens.QueryDataset(name, "primary", text)
	if spec.Query != nil {
		spec.Query.Params = params
	}
	return spec
}

func (c *ShowcaseController) Error403Page(
	r *http.Request,
	w http.ResponseWriter,
	logger *logrus.Entry,
) {
	props := showcase.IndexPageProps{
		SidebarProps: c.getSidebarProps(),
	}
	templ.Handler(showcase.Error403Page(props)).ServeHTTP(w, r)
}

func (c *ShowcaseController) Error404Page(
	r *http.Request,
	w http.ResponseWriter,
	logger *logrus.Entry,
) {
	props := showcase.IndexPageProps{
		SidebarProps: c.getSidebarProps(),
	}
	templ.Handler(showcase.Error404Page(props)).ServeHTTP(w, r)
}

func (c *ShowcaseController) Error403Preview(
	r *http.Request,
	w http.ResponseWriter,
	logger *logrus.Entry,
) {
	w.WriteHeader(http.StatusForbidden)
	templ.Handler(error_pages.ForbiddenContent()).ServeHTTP(w, r)
}

func (c *ShowcaseController) Error404Preview(
	r *http.Request,
	w http.ResponseWriter,
	logger *logrus.Entry,
) {
	w.WriteHeader(http.StatusNotFound)
	templ.Handler(error_pages.NotFoundContent()).ServeHTTP(w, r)
}

func (c *ShowcaseController) ToastExample(
	r *http.Request,
	w http.ResponseWriter,
	logger *logrus.Entry,
) {
	htmx.ToastSuccess(w, "Server Response", "This toast was triggered by an HTMX request!")
	w.WriteHeader(http.StatusOK)
}
