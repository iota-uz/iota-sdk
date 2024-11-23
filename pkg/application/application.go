package application

import (
	"bytes"
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/benbjohnson/hashfs"
	"github.com/gorilla/mux"
	"github.com/iota-agency/iota-sdk/modules/core"
	"github.com/iota-agency/iota-sdk/pkg/domain/entities/permission"
	"github.com/iota-agency/iota-sdk/pkg/event"
	"github.com/iota-agency/iota-sdk/pkg/types"
	"github.com/nicksnyder/go-i18n/v2/i18n"
	migrate "github.com/rubenv/sql-migrate"
	"golang.org/x/text/language"
	"gorm.io/gorm"
	"log"
	"reflect"
)

// Application with a dynamically extendable service registry
type Application interface {
	DB() *gorm.DB
	EventPublisher() event.Publisher
	Modules() []Module
	Controllers() []Controller
	Middleware() []mux.MiddlewareFunc
	Assets() []*embed.FS
	HashFsAssets() []*hashfs.FS
	Templates() []*embed.FS
	LocaleFiles() []*embed.FS
	MigrationDirs() []*embed.FS
	Seed(ctx context.Context) error
	Permissions() []permission.Permission
	RegisterControllers(controllers ...Controller)
	NavigationItems(localizer *i18n.Localizer) []types.NavigationItem
	RegisterModule(module Module)
	RegisterPermissions(permissions ...permission.Permission)
	RegisterHashFsAssets(fs ...*hashfs.FS)
	RegisterAssets(fs ...*embed.FS)
	RegisterTemplates(fs ...*embed.FS)
	RegisterLocaleFiles(fs ...*embed.FS)
	RegisterMigrationDirs(fs ...*embed.FS)
	RegisterService(service interface{})
	RegisterMiddleware(middleware ...mux.MiddlewareFunc)
	Service(service interface{}) interface{}
	Bundle() (*i18n.Bundle, error)
	RunMigrations() error
	RollbackMigrations() error
}

func New(db *gorm.DB, eventPublisher event.Publisher) Application {
	return &ApplicationImpl{
		db:             db,
		eventPublisher: eventPublisher,
		rbac:           permission.NewRbac(),
		services:       make(map[reflect.Type]interface{}),
	}
}

// ApplicationImpl with a dynamically extendable service registry
type ApplicationImpl struct {
	db             *gorm.DB
	eventPublisher event.Publisher
	rbac           *permission.Rbac
	services       map[reflect.Type]interface{}
	modules        []Module
	controllers    []Controller
	middleware     []mux.MiddlewareFunc
	hashFsAssets   []*hashfs.FS
	assets         []*embed.FS
	templates      []*embed.FS
	localeFiles    []*embed.FS
	migrationDirs  []*embed.FS
}

func (app *ApplicationImpl) Seed(ctx context.Context) error {
	for _, module := range app.modules {
		if err := module.Seed(ctx, app); err != nil {
			return err
		}
	}
	return nil
}

func (app *ApplicationImpl) Permissions() []permission.Permission {
	return app.rbac.Permissions()
}

func (app *ApplicationImpl) Middleware() []mux.MiddlewareFunc {
	return app.middleware
}

func (app *ApplicationImpl) RegisterPermissions(permissions ...permission.Permission) {
	app.rbac.Register(permissions...)
}

func (app *ApplicationImpl) DB() *gorm.DB {
	return app.db
}

func (app *ApplicationImpl) EventPublisher() event.Publisher {
	return app.eventPublisher
}

func (app *ApplicationImpl) Modules() []Module {
	return app.modules
}

func (app *ApplicationImpl) Controllers() []Controller {
	return app.controllers
}

func (app *ApplicationImpl) Assets() []*embed.FS {
	return app.assets
}

func (app *ApplicationImpl) HashFsAssets() []*hashfs.FS {
	return app.hashFsAssets
}

func (app *ApplicationImpl) Templates() []*embed.FS {
	return app.templates
}

func (app *ApplicationImpl) LocaleFiles() []*embed.FS {
	return app.localeFiles
}

func (app *ApplicationImpl) MigrationDirs() []*embed.FS {
	return app.migrationDirs
}

func (app *ApplicationImpl) NavigationItems(l *i18n.Localizer) []types.NavigationItem {
	var result []types.NavigationItem
	for _, m := range app.modules {
		result = append(result, m.NavigationItems(l)...)
	}
	return result
}

func (app *ApplicationImpl) RegisterControllers(controllers ...Controller) {
	app.controllers = append(app.controllers, controllers...)
}

func (app *ApplicationImpl) RegisterModule(module Module) {
	app.modules = append(app.modules, module)
}

func (app *ApplicationImpl) RegisterMiddleware(middleware ...mux.MiddlewareFunc) {
	app.middleware = append(app.middleware, middleware...)
}

func (app *ApplicationImpl) RegisterHashFsAssets(fs ...*hashfs.FS) {
	app.hashFsAssets = append(app.hashFsAssets, fs...)
}

func (app *ApplicationImpl) RegisterAssets(fs ...*embed.FS) {
	app.assets = append(app.assets, fs...)
}

func (app *ApplicationImpl) RegisterTemplates(fs ...*embed.FS) {
	app.templates = append(app.templates, fs...)
}

func (app *ApplicationImpl) RegisterLocaleFiles(fs ...*embed.FS) {
	app.localeFiles = append(app.localeFiles, fs...)
}

func (app *ApplicationImpl) RegisterMigrationDirs(fs ...*embed.FS) {
	app.migrationDirs = append(app.migrationDirs, fs...)
}

// RegisterService registers a new service in the application by its type
func (app *ApplicationImpl) RegisterService(service interface{}) {
	serviceType := reflect.TypeOf(service).Elem()
	app.services[serviceType] = service
}

// Service retrieves a service by its type
func (app *ApplicationImpl) Service(service interface{}) interface{} {
	serviceType := reflect.TypeOf(service)
	svc, exists := app.services[serviceType]
	if !exists {
		panic(fmt.Sprintf("service %s not found", serviceType.Name()))
	}
	return svc
}
func (app *ApplicationImpl) Bundle() (*i18n.Bundle, error) {
	bundle := i18n.NewBundle(language.Russian)
	bundle.RegisterUnmarshalFunc("json", json.Unmarshal)
	localeDirs := append([]*embed.FS{&core.LocalesFS}, app.LocaleFiles()...)
	for _, localeFs := range localeDirs {
		files, err := localeFs.ReadDir("locales")
		if err != nil {
			return nil, err
		}
		for _, file := range files {
			if !file.IsDir() {
				localeFile, err := localeFs.ReadFile("locales/" + file.Name())
				if err != nil {
					return nil, err
				}
				bundle.MustParseMessageFileBytes(localeFile, file.Name())
			}
		}
	}
	return bundle, nil
}

func CollectMigrations(app *ApplicationImpl) ([]*migrate.Migration, error) {
	migrationDirs := append([]*embed.FS{&core.MigrationsFs}, app.MigrationDirs()...)

	var migrations []*migrate.Migration
	for _, fs := range migrationDirs {
		files, err := fs.ReadDir("migrations")
		if err != nil {
			return nil, err
		}
		for _, file := range files {
			if file.IsDir() {
				continue
			}
			content, err := fs.ReadFile("migrations/" + file.Name())
			if err != nil {
				return nil, err
			}
			migration, err := migrate.ParseMigration(file.Name(), bytes.NewReader(content))
			if err != nil {
				return nil, err
			}
			migrations = append(migrations, migration)
		}
	}
	if len(migrations) == 0 {
		return nil, errors.New("no migrations found")
	}
	return migrations, nil
}

func (app *ApplicationImpl) RunMigrations() error {
	db, err := app.db.DB()
	if err != nil {
		return err
	}
	migrations, err := CollectMigrations(app)
	if err != nil {
		return err
	}
	migrationSource := &migrate.MemoryMigrationSource{
		Migrations: migrations,
	}
	n, err := migrate.Exec(db, "postgres", migrationSource, migrate.Up)
	if err != nil {
		return err
	}
	if n == 0 {
		return errors.New("no migrations applied")
	} else {
		log.Printf("Applied %d migrations", n)
	}
	return nil
}

func (app *ApplicationImpl) RollbackMigrations() error {
	db, err := app.db.DB()
	if err != nil {
		return err
	}
	migrations, err := CollectMigrations(app)
	if err != nil {
		return err
	}
	migrationSource := &migrate.MemoryMigrationSource{
		Migrations: migrations,
	}
	n, err := migrate.Exec(db, "postgres", migrationSource, migrate.Down)
	if err != nil {
		return err
	}
	if n == 0 {
		return errors.New("no migrations found")
	}
	return nil
}
