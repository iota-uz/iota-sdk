package application

import (
	"context"
	"embed"
	"github.com/benbjohnson/hashfs"
	"github.com/gorilla/mux"
	"github.com/iota-agency/iota-sdk/pkg/domain/entities/permission"
	"github.com/iota-agency/iota-sdk/pkg/event"
	"github.com/iota-agency/iota-sdk/pkg/types"
	"github.com/nicksnyder/go-i18n/v2/i18n"
	"gorm.io/gorm"
)

// Application with a dynamically extendable service registry
type Application interface {
	DB() *gorm.DB
	EventPublisher() event.Publisher
	Controllers() []Controller
	Middleware() []mux.MiddlewareFunc
	Assets() []*embed.FS
	HashFsAssets() []*hashfs.FS
	Templates() []*embed.FS
	LocaleFiles() []*embed.FS
	MigrationDirs() []*embed.FS
	Seed(ctx context.Context) error
	Permissions() []permission.Permission
	NavItems(localizer *i18n.Localizer) []types.NavigationItem
	RegisterNavItems(items ...types.NavigationItem)
	RegisterControllers(controllers ...Controller)
	RegisterPermissions(permissions ...permission.Permission)
	RegisterHashFsAssets(fs ...*hashfs.FS)
	RegisterSeedFuncs(seedFuncs ...SeedFunc)
	RegisterAssets(fs ...*embed.FS)
	RegisterTemplates(fs ...*embed.FS)
	RegisterLocaleFiles(fs ...*embed.FS)
	RegisterMigrationDirs(fs ...*embed.FS)
	RegisterServices(services ...interface{})
	RegisterMiddleware(middleware ...mux.MiddlewareFunc)
	Service(service interface{}) interface{}
	Bundle() *i18n.Bundle
	RunMigrations() error
	RollbackMigrations() error
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
