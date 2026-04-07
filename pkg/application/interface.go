// Package application defines the SDK composition surface for wiring, transports, and runtime startup.
package application

import (
	"context"
	"embed"
	"reflect"

	"github.com/iota-uz/applets"
	"github.com/iota-uz/iota-sdk/pkg/di"
	"github.com/iota-uz/iota-sdk/pkg/eventbus"
	"github.com/iota-uz/iota-sdk/pkg/spotlight"
	"github.com/iota-uz/iota-sdk/pkg/types"

	"github.com/99designs/gqlgen/graphql"
	"github.com/99designs/gqlgen/graphql/executor"
	"github.com/benbjohnson/hashfs"
	"github.com/gorilla/mux"
	"github.com/iota-uz/go-i18n/v2/i18n"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sirupsen/logrus"
)

type GraphSchema struct {
	Value      graphql.ExecutableSchema
	BasePath   string
	ExecutorCb func(*executor.Executor)
}

// Application with a dynamically extendable service registry
type Application interface {
	DB() *pgxpool.Pool
	EventPublisher() eventbus.EventBus
	Controllers() []Controller
	Middleware() []mux.MiddlewareFunc
	Assets() []*embed.FS
	HashFsAssets() []*hashfs.FS
	Websocket() Huber
	Spotlight() spotlight.Service
	QuickLinks() *spotlight.QuickLinks
	Migrations() MigrationManager
	NavItems(localizer *i18n.Localizer) []types.NavigationItem
	RegisterNavItems(items ...types.NavigationItem)
	AppendNavChildren(parentName string, children ...types.NavigationItem)
	RegisterControllers(controllers ...Controller)
	RegisterHashFsAssets(fs ...*hashfs.FS)
	RegisterAssets(fs ...*embed.FS)
	RegisterLocaleFiles(fs ...*embed.FS)
	RegisterGraphSchema(schema GraphSchema)
	GraphSchemas() []GraphSchema
	RegisterServices(services ...interface{})
	RegisterRuntime(registrations ...RuntimeRegistration)
	RuntimeComponents() []RuntimeRegistration
	StartRuntime(ctx context.Context, profile CompositionProfile) error
	StopRuntime(ctx context.Context) error
	RegisterMiddleware(middleware ...mux.MiddlewareFunc)
	Service(service interface{}) interface{}
	Services() map[reflect.Type]interface{}
	Bundle() *i18n.Bundle
	GetSupportedLanguages() []string
	RegisterApplet(applet Applet) error
	AppletRegistry() AppletRegistry
	CreateAppletControllers(
		host applets.HostServices,
		sessionConfig applets.SessionConfig,
		logger *logrus.Logger,
		metrics applets.MetricsRecorder,
		opts ...applets.BuilderOption,
	) ([]Controller, error)
	RegisterAppletRuntime(
		host applets.HostServices,
		sessionConfig applets.SessionConfig,
		logger *logrus.Logger,
		metrics applets.MetricsRecorder,
		opts ...applets.BuilderOption,
	) error
}

type Seeder interface {
	Seed(ctx context.Context, deps *SeedDeps) error
	Register(funcs ...SeedFunc)
}

type SeedFunc func(ctx context.Context, deps *SeedDeps) error

type SeedDeps struct {
	Pool      *pgxpool.Pool
	EventBus  eventbus.EventBus
	Logger    logrus.FieldLogger
	providers []di.Provider
}

func (d *SeedDeps) RegisterValues(values ...interface{}) {
	if d == nil {
		panic("seed deps are required")
	}
	for _, value := range values {
		d.providers = append(d.providers, di.ValueProvider(value))
	}
}

func (d *SeedDeps) RegisterProviders(providers ...di.Provider) {
	if d == nil {
		panic("seed deps are required")
	}
	d.providers = append(d.providers, providers...)
}

type Controller interface {
	Register(r *mux.Router)
	Key() string
}

type Module interface {
	Name() string
	RegisterWiring(app Application) error
	RegisterTransports(app Application) error
}

// Applet represents a React/Next.js application that integrates with the SDK
// This is now an alias for applets.Applet to unify the applet system.
// All applets should implement applets.Applet directly, which includes Config().
type Applet = applets.Applet

// AppletRegistry is now an alias for applets.Registry to unify the registry system.
// The application uses pkg/applets.Registry directly for all applet operations.
type AppletRegistry = applets.Registry
