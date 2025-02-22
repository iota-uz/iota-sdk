package application

import (
	"context"
	"embed"

	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/permission"
	"github.com/iota-uz/iota-sdk/pkg/spotlight"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/99designs/gqlgen/graphql"
	"github.com/benbjohnson/hashfs"
	"github.com/gorilla/mux"
	"github.com/iota-uz/iota-sdk/pkg/eventbus"
	"github.com/iota-uz/iota-sdk/pkg/types"
	"github.com/nicksnyder/go-i18n/v2/i18n"
)

type GraphSchema struct {
	Value    graphql.ExecutableSchema
	BasePath string
}

// Application with a dynamically extendable service registry
type Application interface {
	DB() *pgxpool.Pool
	EventPublisher() eventbus.EventBus
	Controllers() []Controller
	Middleware() []mux.MiddlewareFunc
	Assets() []*embed.FS
	HashFsAssets() []*hashfs.FS
	MigrationDirs() []*embed.FS
	RBAC() permission.RBAC
	Spotlight() spotlight.Spotlight
	NavItems(localizer *i18n.Localizer) []types.NavigationItem
	RegisterNavItems(items ...types.NavigationItem)
	RegisterControllers(controllers ...Controller)
	RegisterHashFsAssets(fs ...*hashfs.FS)
	RegisterAssets(fs ...*embed.FS)
	RegisterLocaleFiles(fs ...*embed.FS)
	RegisterGraphSchema(schema GraphSchema)
	GraphSchemas() []GraphSchema
	RegisterServices(services ...interface{})
	RegisterMiddleware(middleware ...mux.MiddlewareFunc)
	Service(service interface{}) interface{}
	Bundle() *i18n.Bundle
	RunMigrations() error
	RollbackMigrations() error
}

type Seeder interface {
	Seed(ctx context.Context, app Application) error
	Register(funcs ...SeedFunc)
}

type SeedFunc func(ctx context.Context, app Application) error

type Controller interface {
	Register(r *mux.Router)
	Key() string
}

type Module interface {
	Name() string
	Register(app Application) error
}
