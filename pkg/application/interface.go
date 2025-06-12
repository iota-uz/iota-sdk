package application

import (
	"context"
	"embed"
	"reflect"

	"github.com/iota-uz/iota-sdk/pkg/eventbus"
	"github.com/iota-uz/iota-sdk/pkg/rbac"
	"github.com/iota-uz/iota-sdk/pkg/spotlight"
	"github.com/iota-uz/iota-sdk/pkg/types"

	"github.com/99designs/gqlgen/graphql"
	"github.com/99designs/gqlgen/graphql/executor"
	"github.com/benbjohnson/hashfs"
	"github.com/gorilla/mux"
	"github.com/iota-uz/go-i18n/v2/i18n"
	"github.com/jackc/pgx/v5/pgxpool"
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
	RBAC() rbac.RBAC
	Websocket() Huber
	Spotlight() spotlight.Spotlight
	QuickLinks() *spotlight.QuickLinks
	Migrations() MigrationManager
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
	Services() map[reflect.Type]interface{}
	Bundle() *i18n.Bundle
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
