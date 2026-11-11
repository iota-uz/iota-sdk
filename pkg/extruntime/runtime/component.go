package runtime

import (
	"context"
	"embed"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/notifications"
	"github.com/iota-uz/iota-sdk/modules/core/services"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/composition"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/catalog"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/configurator"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/manage"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/spi"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/ui"
	"github.com/iota-uz/iota-sdk/pkg/middleware"
)

// NewComponent enables the extension runtime in an iota-sdk application. It
// builds on the core module (notification services, users) and provides
// *Runtime, *manage.Service, *catalog.Catalog and spi.Catalog to the
// container; with the API capability it mounts the management API under
// APIPrefix, and with the API or worker capability it starts the runtime's
// background parts (catalog listener, workflow engine, event dispatcher,
// scheduler, notification adapter) and closes the Node pool on shutdown.
//
// With Config.UI set, the UI slice is composed first: its middleware and
// controller are registered before the management API, over the same
// storage (its source, artifact store, invoker and catalog are the
// runtime's).
//
//	comp, err := runtime.NewComponent(runtime.Config{
//	    NodeBinary: "node", WorkerScript: "dist/host/worker.mjs",
//	    PoolSize: 4, CacheDir: "/var/cache/app/extruntime",
//	    Builder: buildWorker, Kits: kitGenerator,
//	})
func NewComponent(cfg Config) (composition.Component, error) {
	c := &component{cfg: cfg}
	if cfg.UI != nil {
		c.holder = &holder{}
		c.cfg.bind = c.holder
		l := &lazy{h: c.holder}
		uic, err := ui.NewComponent(ui.Config{
			Source: l, Artifacts: l, Invoker: l, Catalog: l,
			Host: cfg.UI.Host, NativeRoutes: cfg.UI.NativeRoutes, Logger: cfg.UI.Logger,
		}, l)
		if err != nil {
			return nil, err
		}
		c.ui = uic
	}
	return c, nil
}

type component struct {
	cfg    Config
	ui     composition.Component
	holder *holder
}

func (c *component) Descriptor() composition.Descriptor {
	return composition.Descriptor{Name: "extruntime"}
}

func (c *component) LocaleFS() []*embed.FS { return nil }

func (c *component) Build(builder *composition.Builder) error {
	if c.ui != nil {
		if err := c.ui.Build(builder); err != nil {
			return err
		}
	}
	composition.ProvideFunc(builder, func(
		db *pgxpool.Pool, dispatch *services.NotificationDispatchService, notices *notifications.Catalog,
		renderers *notifications.ContentRenderers, contacts *notifications.ContactResolvers, users user.Repository,
	) (*Runtime, error) {
		return New(c.cfg, Deps{DB: db, Dispatch: dispatch, Notices: notices, Renderers: renderers, Contacts: contacts, Users: users})
	})
	composition.ProvideFunc(builder, func(rt *Runtime) *manage.Service { return rt.Manage })
	composition.ProvideFuncAs[spi.Catalog](builder, func(rt *Runtime) *catalog.Catalog { return rt.Catalog })
	if builder.Context().HasCapability(composition.CapabilityAPI) {
		composition.ContributeControllersFunc(builder, func(rt *Runtime) []application.Controller {
			return []application.Controller{NewController(NewAPI(rt.Manage, requestSubject, rt.log))}
		})
		if c.cfg.Configurator != nil {
			composition.ContributeControllersFunc(builder, func(rt *Runtime) ([]application.Controller, error) {
				cc := *c.cfg.Configurator
				cc.Service = rt.Manage
				if cc.Logger == nil {
					cc.Logger = rt.log
				}
				if len(cc.Host.Entry) == 0 && c.cfg.UI != nil {
					cc.Host = c.cfg.UI.Host
				}
				ctrl, err := configurator.NewController(cc)
				if err != nil {
					return nil, err
				}
				return []application.Controller{ctrl}, nil
			})
		}
	}
	if builder.Context().HasCapability(composition.CapabilityAPI) || builder.Context().HasCapability(composition.CapabilityWorker) {
		composition.ContributeHooks(builder, func(container *composition.Container) ([]composition.Hook, error) {
			rt, err := composition.Resolve[*Runtime](container)
			if err != nil {
				return nil, err
			}
			return []composition.Hook{{Name: "extruntime", Start: func(ctx context.Context) (composition.StopFn, error) {
				if err := rt.Start(ctx); err != nil {
					return nil, err
				}
				return rt.Close, nil
			}}}, nil
		})
	}
	return nil
}

func requestSubject(r *http.Request) (spi.Subject, error) { return SubjectFromContext(r.Context()) }

// Controller mounts the management API behind the session middleware. All
// authorization is the service's: the route only requires a signed-in user.
type Controller struct{ api *API }

// NewController wraps the API as an application controller.
func NewController(api *API) *Controller { return &Controller{api: api} }

func (c *Controller) Descriptor() application.ControllerDescriptor {
	return application.Descriptor("extruntime.manage", 0,
		application.Route("", APIPrefix, application.Authenticated(), func(r *application.RouteSpec) { r.Prefix = true }))
}

func (c *Controller) Register(r *mux.Router) {
	var h http.Handler = c.api.Handler()
	for _, mw := range []mux.MiddlewareFunc{middleware.ProvideUser(), middleware.Authorize()} {
		h = mw(h)
	}
	r.PathPrefix(APIPrefix).Handler(h)
}

var _ application.Controller = (*Controller)(nil)
