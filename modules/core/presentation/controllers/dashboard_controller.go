// Package controllers provides this package.
package controllers

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/a-h/templ"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
	icons "github.com/iota-uz/icons/phosphor"

	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/permission"
	"github.com/iota-uz/iota-sdk/modules/core/presentation/templates/pages/dashboard"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/dbconfig"
	"github.com/iota-uz/iota-sdk/pkg/lens"
	"github.com/iota-uz/iota-sdk/pkg/lens/datasource"
	lensdocument "github.com/iota-uz/iota-sdk/pkg/lens/document"
	lenspostgres "github.com/iota-uz/iota-sdk/pkg/lens/postgres"
	lenssolid "github.com/iota-uz/iota-sdk/pkg/lens/render/solid"
	"github.com/iota-uz/iota-sdk/pkg/lens/runtime"
	lensserve "github.com/iota-uz/iota-sdk/pkg/lens/serve"
	"github.com/iota-uz/iota-sdk/pkg/middleware"
	"github.com/iota-uz/iota-sdk/pkg/serrors/serrorhttp"
)

const dashboardLensBasePath = "/dashboard"

func NewDashboardController(dbCfg *dbconfig.Config, navPermissions ...[]permission.Permission) application.Controller {
	perms := firstPermissionSet(navPermissions)
	ds, err := lenspostgres.New(lenspostgres.Config{
		ConnectionString: dbCfg.ConnectionString(),
		MaxConnections:   5,
		MinConnections:   1,
		QueryTimeout:     30 * time.Second,
		RequiredParams:   []string{"tenant_id"},
	})
	if err != nil {
		log.Printf("Failed to create lens data source for dashboard: %v", err)
		return &DashboardController{
			navPermissions: perms,
			runtime:        runtime.New(runtime.Options{}),
			snapshots:      lensdocument.NewMemoryStore(30*time.Minute, 128),
		}
	}
	return &DashboardController{
		ds:             ds,
		navPermissions: perms,
		runtime:        runtime.New(runtime.Options{}),
		snapshots:      lensdocument.NewMemoryStore(30*time.Minute, 128),
	}
}

type DashboardController struct {
	ds             datasource.DataSource
	navPermissions []permission.Permission
	runtime        *runtime.Runtime
	snapshots      lensdocument.SnapshotStore
}

func (c *DashboardController) Descriptor() application.ControllerDescriptor {
	return application.Descriptor("core.dashboard", 0, application.Route("", "/", navRouteOptions(c.navPermissions)...)).
		WithNav(application.NavNode{
			ID:         "core.dashboard",
			TitleKey:   "NavigationLinks.Dashboard",
			Path:       "/",
			Icon:       icons.Gauge(icons.Props{Size: "20"}),
			Order:      10,
			Visibility: navAuthPolicy(c.navPermissions),
		})
}

func (c *DashboardController) Register(r *mux.Router) {
	lenssolid.NewStaticController().Register(r)
	router := r.NewRoute().Subrouter()
	router.Use(
		middleware.Authorize(),
		middleware.RedirectNotAuthenticated(),
		middleware.ProvideUser(),
		middleware.ProvideDynamicLogo(),
		middleware.NavItems(),
		middleware.WithPageContext(),
	)
	router.HandleFunc("/", c.Get).Methods(http.MethodGet)
	router.HandleFunc(dashboardLensBasePath+"/document", c.Document).Methods(http.MethodGet)
	router.HandleFunc(dashboardLensBasePath+"/lens/query", c.Query).Methods(http.MethodPost)
	router.HandleFunc(dashboardLensBasePath+"/export", c.Export).Methods(http.MethodGet)
	router.HandleFunc(financeOperationsDocument, c.operations(func(h *lensserve.Handlers) http.HandlerFunc { return h.Document })).Methods(http.MethodGet)
	router.HandleFunc(financeOperationsBasePath+"/lens/query", c.operations(func(h *lensserve.Handlers) http.HandlerFunc { return h.Query })).Methods(http.MethodPost)
	router.HandleFunc(financeOperationsBasePath+"/export", c.operations(func(h *lensserve.Handlers) http.HandlerFunc { return h.Export })).Methods(http.MethodGet)
	router.HandleFunc(financeOperationsBasePath+"/lens/release", c.operations(func(h *lensserve.Handlers) http.HandlerFunc { return h.Release })).Methods(http.MethodPost)
}

func (c *DashboardController) Get(w http.ResponseWriter, r *http.Request) {
	tenantID, err := composables.UseTenantID(r.Context())
	if err != nil {
		log.Printf("Dashboard tenant lookup failed: %v", err)
		http.Error(w, "tenant not found", http.StatusBadRequest)
		return
	}
	dash := financeDashboardSpec(tenantID, financeText(r.Context()))

	props := &dashboard.IndexPageProps{
		Dashboard:   dash,
		DocumentURL: dashboardLensBasePath + "/document",
		Available:   c.ds != nil,
	}
	templ.Handler(dashboard.Index(props)).ServeHTTP(w, r)
}

func (c *DashboardController) Document(w http.ResponseWriter, r *http.Request) {
	handlers, err := c.lensHandlers(r)
	if err != nil {
		serrorhttp.WriteTextContext(r.Context(), w, err, http.StatusServiceUnavailable, nil)
		return
	}
	handlers.Document(w, r)
}

func (c *DashboardController) Query(w http.ResponseWriter, r *http.Request) {
	handlers, err := c.lensHandlers(r)
	if err != nil {
		serrorhttp.WriteTextContext(r.Context(), w, err, http.StatusServiceUnavailable, nil)
		return
	}
	handlers.Query(w, r)
}

func (c *DashboardController) Export(w http.ResponseWriter, r *http.Request) {
	handlers, err := c.lensHandlers(r)
	if err != nil {
		serrorhttp.WriteTextContext(r.Context(), w, err, http.StatusServiceUnavailable, nil)
		return
	}
	handlers.Export(w, r)
}

func (c *DashboardController) lensHandlers(r *http.Request) (*lensserve.Handlers, error) {
	if c.ds == nil {
		return nil, fmt.Errorf("dashboard data source is unavailable")
	}
	tenantID, err := composables.UseTenantID(r.Context())
	if err != nil {
		return nil, fmt.Errorf("dashboard tenant lookup: %w", err)
	}
	return c.serve(financeDashboardSpec(tenantID, financeText(r.Context())), dashboardLensBasePath, "core.dashboard", tenantID, nil)
}

// operations serves the drawer document listing the account operations behind
// a dashboard mark. The drill keys are validated before any query runs.
func (c *DashboardController) operations(pick func(*lensserve.Handlers) http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		drill, err := parseOperationsDrill(r.URL.Query())
		if err != nil {
			serrorhttp.WriteTextContext(r.Context(), w, err, http.StatusBadRequest, nil)
			return
		}
		if c.ds == nil {
			serrorhttp.WriteTextContext(r.Context(), w, fmt.Errorf("dashboard data source is unavailable"), http.StatusServiceUnavailable, nil)
			return
		}
		tenantID, err := composables.UseTenantID(r.Context())
		if err != nil {
			serrorhttp.WriteTextContext(r.Context(), w, fmt.Errorf("dashboard tenant lookup: %w", err), http.StatusServiceUnavailable, nil)
			return
		}
		text := financeText(r.Context())
		spec := financeOperationsSpec(tenantID, drill, text)
		handlers, err := c.serve(spec, financeOperationsBasePath, "core.dashboard.operations", tenantID,
			func(_ *http.Request, req runtime.Request) lensdocument.BuildOptions {
				return lensdocument.BuildOptions{Endpoints: operationsEndpoints(drill), Drawer: &lensdocument.DrawerHeader{
					Eyebrow: text.OperationsDrawer,
					Title:   operationsDrawerTitle(drill),
					Caption: operationsDrawerCaption(spec, req),
					Size:    lensdocument.DrawerSizeWide,
				}}
			})
		if err != nil {
			serrorhttp.WriteTextContext(r.Context(), w, err, http.StatusServiceUnavailable, nil)
			return
		}
		pick(handlers)(w, r)
	}
}

func (c *DashboardController) serve(spec lens.DashboardSpec, basePath, namespace string, tenantID uuid.UUID, options lensserve.DocumentOptionsResolver) (*lensserve.Handlers, error) {
	return lensserve.New(lensserve.Config{
		Spec:            spec,
		Engine:          c.runtime,
		Snapshots:       c.snapshots,
		BasePath:        basePath,
		InlineDepth:     1,
		DocumentOptions: options,
		Request: func(*http.Request) runtime.Request {
			return runtime.Request{
				DataSources:          map[string]datasource.DataSource{"primary": c.ds},
				DataSourceIdentities: map[string]string{"primary": "core-postgres:v1"},
				DataScope:            tenantID.String(),
				Namespace:            namespace,
			}
		},
	})
}
