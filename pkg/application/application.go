package application

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"path/filepath"
	"reflect"

	"github.com/BurntSushi/toml"
	"github.com/benbjohnson/hashfs"
	"github.com/gorilla/mux"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nicksnyder/go-i18n/v2/i18n"
	"golang.org/x/text/language"

	"github.com/iota-uz/iota-sdk/pkg/configuration"
	"github.com/iota-uz/iota-sdk/pkg/eventbus"
	"github.com/iota-uz/iota-sdk/pkg/rbac"
	"github.com/iota-uz/iota-sdk/pkg/spotlight"
	"github.com/iota-uz/iota-sdk/pkg/types"
)

func translate(localizer *i18n.Localizer, items []types.NavigationItem) []types.NavigationItem {
	translated := make([]types.NavigationItem, 0, len(items))
	for _, item := range items {
		translated = append(translated, types.NavigationItem{
			Name: localizer.MustLocalize(&i18n.LocalizeConfig{
				MessageID: item.Name,
			}),
			Href:        item.Href,
			Children:    translate(localizer, item.Children),
			Icon:        item.Icon,
			Permissions: item.Permissions,
		})
	}
	return translated
}

func listFiles(fsys fs.FS, dir string) ([]string, error) {
	var fileList []string

	err := fs.WalkDir(fsys, dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			fileList = append(fileList, path)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("error reading directory %q: %w", dir, err)
	}

	return fileList, nil
}

// ---- Seeder implementation ----

func NewSeeder() Seeder {
	return &seeder{}
}

type seeder struct {
	seedFuncs []SeedFunc
}

func (s *seeder) Seed(ctx context.Context, app Application) error {
	conf := configuration.Use()
	for _, seedFunc := range s.seedFuncs {
		conf.Logger().Infof("Seeding %s", reflect.TypeOf(seedFunc).Name())
		if err := seedFunc(ctx, app); err != nil {
			return err
		}
	}
	return nil
}

func (s *seeder) Register(seedFuncs ...SeedFunc) {
	s.seedFuncs = append(s.seedFuncs, seedFuncs...)
}

// ---- Application implementation ----

func New(pool *pgxpool.Pool, eventPublisher eventbus.EventBus) Application {
	bundle := i18n.NewBundle(language.Russian)
	bundle.RegisterUnmarshalFunc("json", json.Unmarshal)
	bundle.RegisterUnmarshalFunc("toml", toml.Unmarshal)

	return &application{
		pool:           pool,
		eventPublisher: eventPublisher,
		rbac:           rbac.NewRbac(),
		controllers:    make(map[string]Controller),
		services:       make(map[reflect.Type]interface{}),
		spotlight:      spotlight.New(),
		bundle:         bundle,
		migrations:     NewMigrationManager(pool),
	}
}

// application with a dynamically extendable service registry
type application struct {
	pool           *pgxpool.Pool
	eventPublisher eventbus.EventBus
	rbac           rbac.RBAC
	services       map[reflect.Type]interface{}
	controllers    map[string]Controller
	middleware     []mux.MiddlewareFunc
	hashFsAssets   []*hashfs.FS
	assets         []*embed.FS
	graphSchemas   []GraphSchema
	bundle         *i18n.Bundle
	spotlight      spotlight.Spotlight
	migrations     MigrationManager
	navItems       []types.NavigationItem
}

func (app *application) Spotlight() spotlight.Spotlight {
	return app.spotlight
}

func (app *application) NavItems(localizer *i18n.Localizer) []types.NavigationItem {
	return translate(localizer, app.navItems)
}

func (app *application) RegisterNavItems(items ...types.NavigationItem) {
	app.navItems = append(app.navItems, items...)
}

func (app *application) RBAC() rbac.RBAC {
	return app.rbac
}

func (app *application) Middleware() []mux.MiddlewareFunc {
	return app.middleware
}

func (app *application) DB() *pgxpool.Pool {
	return app.pool
}

func (app *application) EventPublisher() eventbus.EventBus {
	return app.eventPublisher
}

func (app *application) Controllers() []Controller {
	controllers := make([]Controller, 0, len(app.controllers))
	for _, c := range app.controllers {
		controllers = append(controllers, c)
	}
	return controllers
}

func (app *application) Assets() []*embed.FS {
	return app.assets
}

func (app *application) HashFsAssets() []*hashfs.FS {
	return app.hashFsAssets
}

func (app *application) Migrations() MigrationManager {
	return app.migrations
}

func (app *application) GraphSchemas() []GraphSchema {
	return app.graphSchemas
}

func (app *application) RegisterControllers(controllers ...Controller) {
	for _, c := range controllers {
		app.controllers[c.Key()] = c
	}
}

func (app *application) RegisterMiddleware(middleware ...mux.MiddlewareFunc) {
	app.middleware = append(app.middleware, middleware...)
}

func (app *application) RegisterHashFsAssets(fs ...*hashfs.FS) {
	app.hashFsAssets = append(app.hashFsAssets, fs...)
}

func (app *application) RegisterAssets(fs ...*embed.FS) {
	app.assets = append(app.assets, fs...)
}

func (app *application) RegisterGraphSchema(schema GraphSchema) {
	app.graphSchemas = append(app.graphSchemas, schema)
}

func (app *application) RegisterLocaleFiles(fs ...*embed.FS) {
	for _, localeFs := range fs {
		files, err := listFiles(localeFs, ".")
		if err != nil {
			panic(err)
		}
		for _, file := range files {
			localeFile, err := localeFs.ReadFile(file)
			if err != nil {
				panic(err)
			}
			app.bundle.MustParseMessageFileBytes(localeFile, filepath.Base(file))
		}
	}
}

// RegisterServices registers a new service in the application by its type
func (app *application) RegisterServices(services ...interface{}) {
	for _, service := range services {
		serviceType := reflect.TypeOf(service).Elem()
		app.services[serviceType] = service
	}
}

// Service retrieves a service by its type
func (app *application) Service(service interface{}) interface{} {
	serviceType := reflect.TypeOf(service)
	svc, exists := app.services[serviceType]
	if !exists {
		panic(fmt.Sprintf("service %s not found", serviceType.Name()))
	}
	return svc
}

func (app *application) Services() map[reflect.Type]interface{} {
	return app.services
}

func (app *application) Bundle() *i18n.Bundle {
	return app.bundle
}
