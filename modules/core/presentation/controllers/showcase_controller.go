//go:build dev

package controllers

import (
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
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/appconfig"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/dbconfig"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/httpconfig"
	"github.com/iota-uz/iota-sdk/pkg/di"
	"github.com/iota-uz/iota-sdk/pkg/htmx"
	"github.com/iota-uz/iota-sdk/pkg/lens"
	lensbuild "github.com/iota-uz/iota-sdk/pkg/lens/build"
	"github.com/iota-uz/iota-sdk/pkg/lens/datasource"
	lensdocument "github.com/iota-uz/iota-sdk/pkg/lens/document"
	"github.com/iota-uz/iota-sdk/pkg/lens/panel"
	lenspostgres "github.com/iota-uz/iota-sdk/pkg/lens/postgres"
	"github.com/iota-uz/iota-sdk/pkg/lens/runtime"
	lensserve "github.com/iota-uz/iota-sdk/pkg/lens/serve"
	"github.com/iota-uz/iota-sdk/pkg/middleware"
)

type ShowcaseController struct {
	basePath  string
	httpCfg   *httpconfig.Config
	appCfg    *appconfig.Config
	ds        datasource.DataSource
	runtime   *runtime.Runtime
	snapshots lensdocument.SnapshotStore
}

func NewShowcaseController(dbCfg *dbconfig.Config, httpCfg *httpconfig.Config, appCfg *appconfig.Config) application.Controller {
	ds, err := lenspostgres.New(lenspostgres.Config{
		ConnectionString: dbCfg.ConnectionString(),
		MaxConnections:   5,
		MinConnections:   1,
		QueryTimeout:     30 * time.Second,
		RequiredParams:   []string{"tenant_id"},
	})
	if err != nil {
		log.Printf("Failed to create lens data source for showcase: %v", err)
		return &ShowcaseController{
			basePath: "/_dev", httpCfg: httpCfg, appCfg: appCfg,
			runtime: runtime.New(runtime.Options{}), snapshots: lensdocument.NewMemoryStore(30*time.Minute, 128),
		}
	}
	return &ShowcaseController{
		basePath: "/_dev", httpCfg: httpCfg, appCfg: appCfg, ds: ds,
		runtime: runtime.New(runtime.Options{}), snapshots: lensdocument.NewMemoryStore(30*time.Minute, 128),
	}
}

func (c *ShowcaseController) Descriptor() application.ControllerDescriptor {
	return application.Descriptor("core.showcase", 0, application.Route("", c.basePath))
}

func (c *ShowcaseController) Register(r *mux.Router) {
	router := r.PathPrefix(c.basePath).Subrouter()
	router.Use(
		middleware.ProvideUser(),
		middleware.ProvideDynamicLogo(),
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
	router.HandleFunc("/lens/document", c.LensDocument).Methods(http.MethodGet)
	router.HandleFunc("/lens/lens/query", c.LensQuery).Methods(http.MethodPost)
	router.HandleFunc("/lens/export", c.LensExport).Methods(http.MethodGet)
	router.HandleFunc("/error-pages/403", di.H(c.Error403Page)).Methods(http.MethodGet)
	router.HandleFunc("/error-pages/404", di.H(c.Error404Page)).Methods(http.MethodGet)
	router.HandleFunc("/error-preview/403", di.H(c.Error403Preview)).Methods(http.MethodGet)
	router.HandleFunc("/error-preview/404", di.H(c.Error404Preview)).Methods(http.MethodGet)
	router.HandleFunc("/api/showcase/toast-example", di.H(c.ToastExample)).Methods(http.MethodPost)

	if c.appCfg != nil {
		log.Printf(
			"See %s%s for docs\n",
			c.httpCfg.Origin(c.appCfg),
			c.basePath,
		)
	}
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
	dash := showcaseLensDashboard(params)
	available := params != nil && c.ds != nil
	if !available {
		logger.Warn("lens showcase dashboard data source is unavailable")
	}

	props := showcase.LensPageProps{
		SidebarProps: c.getSidebarProps(),
		Dashboard:    dash,
		DocumentURL:  c.basePath + "/lens/document",
		Available:    available,
	}
	templ.Handler(showcase.LensPage(props)).ServeHTTP(w, r)
}

func showcaseLensDashboard(params map[string]lens.ParamValue) lens.DashboardSpec {
	return lensbuild.Dashboard("sdk-core-analytics", "IOTA SDK Core Analytics",
		lensbuild.Row(
			panel.TimeSeries("user-registrations", "User Registrations Over Time", "user-registrations").Span(6).Build(),
			panel.Bar("user-languages", "User Interface Languages", "user-languages").Span(6).Build(),
		),
		lensbuild.Row(
			panel.Pie("user-types", "User Type Distribution", "user-types").Legend().Span(4).Build(),
			panel.Gauge("session-activity", "Active Sessions", "session-activity").Span(4).Build(),
			panel.Table("recent-users", "Recently Registered Users", "recent-users").Span(4).Build(),
		),
	).Datasets(
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
	).Build()
}

func (c *ShowcaseController) LensDocument(w http.ResponseWriter, r *http.Request) {
	handlers, err := c.lensHandlers(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	handlers.Document(w, r)
}

func (c *ShowcaseController) LensQuery(w http.ResponseWriter, r *http.Request) {
	handlers, err := c.lensHandlers(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	handlers.Query(w, r)
}

func (c *ShowcaseController) LensExport(w http.ResponseWriter, r *http.Request) {
	handlers, err := c.lensHandlers(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	handlers.Export(w, r)
}

func (c *ShowcaseController) lensHandlers(r *http.Request) (*lensserve.Handlers, error) {
	if c.ds == nil {
		return nil, fmt.Errorf("showcase data source is unavailable")
	}
	tenantID, err := composables.UseTenantID(r.Context())
	if err != nil {
		return nil, fmt.Errorf("showcase tenant lookup: %w", err)
	}
	spec := showcaseLensDashboard(map[string]lens.ParamValue{"tenant_id": {Literal: tenantID}})
	return lensserve.New(lensserve.Config{
		Spec: spec, Engine: c.runtime, Snapshots: c.snapshots,
		BasePath: c.basePath + "/lens", InlineDepth: 1,
		ViewsEndpoint: "/lens/share/views", SchedulesEndpoint: "/lens/share/schedules",
		Request: func(*http.Request) runtime.Request {
			return runtime.Request{
				DataSources:          map[string]datasource.DataSource{"primary": c.ds},
				DataSourceIdentities: map[string]string{"primary": "core-postgres:v1"},
				DataScope:            tenantID.String(),
				Namespace:            "core.showcase",
			}
		},
	})
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
	spec := lensbuild.QueryDataset(name, "primary", text)
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
