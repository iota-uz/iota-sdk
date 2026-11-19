package testkit

import (
	"embed"
	"fmt"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/composition"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/appconfig"
	"github.com/iota-uz/iota-sdk/pkg/testenv"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ScenarioFactory func(*pgxpool.Pool) (*testenv.Registry, string, error)

// NewScenarioComponent mounts an explicitly configured, credential-protected
// registry alongside the existing testkit, only when test endpoints are enabled.
func NewScenarioComponent(factory ScenarioFactory) composition.Component {
	return &scenarioComponent{factory: factory}
}

type scenarioComponent struct{ factory ScenarioFactory }

func (c *scenarioComponent) Descriptor() composition.Descriptor {
	return composition.Descriptor{Name: "testkit.scenarios", Requires: []string{"core"}}
}
func (c *scenarioComponent) LocaleFS() []*embed.FS { return nil }
func (c *scenarioComponent) Build(builder *composition.Builder) error {
	if c.factory == nil {
		return fmt.Errorf("scenario factory required")
	}
	if !builder.Context().HasCapability(composition.CapabilityAPI) {
		return nil
	}
	composition.ContributeControllers(builder, func(container *composition.Container) ([]application.Controller, error) {
		cfg, err := composition.Resolve[*appconfig.Config](container)
		if err != nil {
			return nil, err
		}
		if !cfg.EnableTestEndpoints || cfg.IsProduction() {
			return nil, fmt.Errorf("scenario controls require an explicitly enabled non-production test environment")
		}
		pool, err := composition.Resolve[*pgxpool.Pool](container)
		if err != nil {
			return nil, err
		}
		registry, token, err := c.factory(pool)
		if err != nil {
			return nil, err
		}
		handler, err := testenv.NewHandler(registry, token)
		if err != nil {
			return nil, err
		}
		return []application.Controller{&scenarioController{handler: handler}}, nil
	})
	return nil
}

type scenarioController struct{ handler http.Handler }

func (c *scenarioController) Descriptor() application.ControllerDescriptor {
	routes := testenv.Routes()
	specs := make([]application.RouteSpec, 0, len(routes))
	for _, route := range routes {
		specs = append(specs, application.Route(route.Method, route.Pattern, application.Public()))
	}
	return application.Descriptor("testkit.scenarios", 0, specs...)
}

// Register mounts every control route; the handler itself enforces the
// runtime control credential and scope ownership.
func (c *scenarioController) Register(router *mux.Router) {
	for _, route := range testenv.Routes() {
		router.Handle(route.Pattern, c.handler).Methods(route.Method)
	}
}
