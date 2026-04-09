// Package application defines the SDK composition surface for runtime access.
package application

import (
	"context"
	"embed"

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

type RuntimeSource interface {
	Controllers() []Controller
	Middleware() []mux.MiddlewareFunc
	Assets() []*embed.FS
	HashFSAssets() []*hashfs.FS
	LocaleFiles() []*embed.FS
	GraphSchemas() []GraphSchema
	Applets() []Applet
	NavItems() []types.NavigationItem
	QuickLinks() []*spotlight.QuickLink
	SpotlightProviders() []spotlight.SearchProvider
}

type RuntimeBinder interface {
	AttachRuntimeSource(source RuntimeSource) error
	DetachRuntimeSource()
}

// Application exposes the runtime services consumed by modules, controllers, and servers.
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
	GraphSchemas() []GraphSchema
	Bundle() *i18n.Bundle
	GetSupportedLanguages() []string
	AppletRegistry() AppletRegistry
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

// Applet represents a React/Next.js application that integrates with the SDK
// This is now an alias for applets.Applet to unify the applet system.
// All applets should implement applets.Applet directly, which includes Config().
type Applet = applets.Applet

// AppletRegistry is now an alias for applets.Registry to unify the registry system.
// The application uses pkg/applets.Registry directly for all applet operations.
type AppletRegistry = applets.Registry
