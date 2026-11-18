package runtime

import (
	"context"
	"embed"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/endpoint"
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
	"github.com/iota-uz/iota-sdk/pkg/extruntime/native"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/originguard"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/pkgdocs"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/reference"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/spi"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/ui"
	"github.com/iota-uz/iota-sdk/pkg/helpdocs"
	"github.com/iota-uz/iota-sdk/pkg/middleware"
)

// NewComponent enables the extension runtime in an iota-sdk application. It
// builds on the core module (notification services, users) and provides
// *Runtime, *manage.Service, *catalog.Catalog, spi.Catalog and the
// native.Dispatcher Go code invokes native contracts through to the
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
//	natives, err := native.Compose(append(modules.Natives(), app.Natives)...)
//	comp, err := runtime.NewComponent(runtime.Config{
//	    Natives: natives, PoolSize: 4, CacheDir: "/var/cache/app/extruntime",
//	    // the workers come from node_modules/@iota-uz/sdk of the same release
//	    Dist: runtime.DistConfig{AppRoot: "/srv/app"},
//	})
func NewComponent(cfg Config) (composition.Component, error) {
	c := &component{cfg: cfg}
	if cfg.UI != nil {
		c.holder = &holder{}
		c.cfg.bind = c.holder
		l := &lazy{h: c.holder}
		origins, err := originguard.New(cfg.TrustedOrigins...)
		if err != nil {
			return nil, err
		}
		uic, err := ui.NewComponent(ui.Config{
			Source: l, Artifacts: l, Invoker: l, Catalog: l,
			Host: cfg.UI.Host, NativeRoutes: ui.NativeRoutes(cfg.Natives.Routes()), Origins: origins, Logger: cfg.UI.Logger,
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
	composition.Provide[*Runtime](builder, func(container *composition.Container) (*Runtime, error) {
		db, err := composition.Resolve[*pgxpool.Pool](container)
		if err != nil {
			return nil, err
		}
		dispatch, err := composition.Resolve[*services.NotificationDispatchService](container)
		if err != nil {
			return nil, err
		}
		notices, err := composition.Resolve[*notifications.Catalog](container)
		if err != nil {
			return nil, err
		}
		renderers, err := composition.Resolve[*notifications.ContentRenderers](container)
		if err != nil {
			return nil, err
		}
		contacts, err := composition.Resolve[*notifications.ContactResolvers](container)
		if err != nil {
			return nil, err
		}
		policies, err := composition.Resolve[*notifications.DeliveryPolicies](container)
		if err != nil {
			return nil, err
		}
		users, err := composition.Resolve[user.Repository](container)
		if err != nil {
			return nil, err
		}
		// The Go defaults of native contracts resolve their services from the
		// container when first invoked.
		return New(c.cfg, Deps{DB: db, Dispatch: dispatch, Notices: notices, Renderers: renderers, Contacts: contacts, Policies: policies, Users: users, NativeDeps: container})
	})
	composition.ProvideFunc(builder, func(rt *Runtime) *manage.Service { return rt.Manage })
	// Every Go entry of a native contract dispatches through the host.
	composition.ProvideFuncAs[native.Dispatcher](builder, func(rt *Runtime) *HostDispatcher { return rt.CommandDispatcher() })
	composition.ProvideFuncAs[spi.Catalog](builder, func(rt *Runtime) *catalog.Catalog { return rt.Catalog })
	// The live reference and the SDK guides join the Help Center whenever the
	// runtime is enabled; what a reader sees is decided per request. The
	// documents link the OpenAPI document only when this build serves it, which
	// is when the API capability composes the controllers below.
	apiServed := builder.Context().HasCapability(composition.CapabilityAPI)
	composition.ContributeHelpSources(builder, func(container *composition.Container) ([]helpdocs.Source, error) {
		rt, err := composition.Resolve[*Runtime](container)
		if err != nil {
			return nil, err
		}
		src, err := reference.NewSource(reference.SourceConfig{
			Builder: rt.Reference, Subject: SubjectFromContext, ManageRoutes: ManageRoutes(), APIServed: apiServed,
		})
		if err != nil {
			return nil, err
		}
		// The documentation the packages ship for their users: the end-user
		// area of the Help Center, next to the developer reference above.
		apps, err := pkgdocs.NewSourceOf(rt.Docs, SubjectFromContext)
		if err != nil {
			return nil, err
		}
		return []helpdocs.Source{src, apps}, nil
	})
	if apiServed {
		composition.ContributeControllersFunc(builder, func(rt *Runtime) []application.Controller {
			api := NewAPI(rt.Manage, requestSubject, rt.log).WithOriginGuard(rt.origins).WithReference(rt.Reference)
			openapi := NewOpenAPI(rt.Reference, requestSubject, func(msg string, err error) { rt.log.Error(msg, "error", err) })
			return []application.Controller{NewController(api), openapi}
		})
		composition.ContributeControllersFunc(builder, func(rt *Runtime) ([]application.Controller, error) {
			h, err := rt.EndpointHandler()
			if err != nil {
				return nil, err
			}
			return []application.Controller{endpoint.NewController(h)}, nil
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
	h := c.api.Handler()
	for _, mw := range []mux.MiddlewareFunc{middleware.ProvideUser(), middleware.Authorize()} {
		h = mw(h)
	}
	r.PathPrefix(APIPrefix).Handler(h)
}

var _ application.Controller = (*Controller)(nil)
