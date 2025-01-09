package application

import (
	"bytes"
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"path/filepath"
	"reflect"
	"strings"
	"time"

	"github.com/go-gorp/gorp/v3"
	"github.com/jackc/pgx/v5/stdlib"

	"github.com/benbjohnson/hashfs"
	"github.com/gorilla/mux"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/permission"
	"github.com/iota-uz/iota-sdk/pkg/event"
	"github.com/iota-uz/iota-sdk/pkg/spotlight"
	"github.com/iota-uz/iota-sdk/pkg/types"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nicksnyder/go-i18n/v2/i18n"
	migrate "github.com/rubenv/sql-migrate"
	"golang.org/x/text/language"
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

func New(pool *pgxpool.Pool, eventPublisher event.Publisher) Application {
	bundle := i18n.NewBundle(language.Russian)
	bundle.RegisterUnmarshalFunc("json", json.Unmarshal)
	return &ApplicationImpl{
		pool:           pool,
		eventPublisher: eventPublisher,
		rbac:           permission.NewRbac(),
		controllers:    make(map[string]Controller),
		services:       make(map[reflect.Type]interface{}),
		spotlight:      spotlight.New(),
		bundle:         bundle,
	}
}

// ApplicationImpl with a dynamically extendable service registry
type ApplicationImpl struct {
	pool           *pgxpool.Pool
	eventPublisher event.Publisher
	rbac           *permission.Rbac
	services       map[reflect.Type]interface{}
	controllers    map[string]Controller
	middleware     []mux.MiddlewareFunc
	hashFsAssets   []*hashfs.FS
	assets         []*embed.FS
	migrationDirs  []*embed.FS
	graphSchemas   []GraphSchema
	seedFuncs      []SeedFunc
	bundle         *i18n.Bundle
	spotlight      spotlight.Spotlight
	navItems       []types.NavigationItem
}

func (app *ApplicationImpl) Spotlight() spotlight.Spotlight {
	return app.spotlight
}

func (app *ApplicationImpl) NavItems(localizer *i18n.Localizer) []types.NavigationItem {
	return translate(localizer, app.navItems)
}

func (app *ApplicationImpl) RegisterNavItems(items ...types.NavigationItem) {
	app.navItems = append(app.navItems, items...)
}

func (app *ApplicationImpl) Seed(ctx context.Context) error {
	for _, seedFunc := range app.seedFuncs {
		if err := seedFunc(ctx, app); err != nil {
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

func (app *ApplicationImpl) DB() *pgxpool.Pool {
	return app.pool
}

func (app *ApplicationImpl) EventPublisher() event.Publisher {
	return app.eventPublisher
}

func (app *ApplicationImpl) Controllers() []Controller {
	controllers := make([]Controller, 0, len(app.controllers))
	for _, c := range app.controllers {
		controllers = append(controllers, c)
	}
	return controllers
}

func (app *ApplicationImpl) Assets() []*embed.FS {
	return app.assets
}

func (app *ApplicationImpl) HashFsAssets() []*hashfs.FS {
	return app.hashFsAssets
}

func (app *ApplicationImpl) MigrationDirs() []*embed.FS {
	return app.migrationDirs
}

func (app *ApplicationImpl) GraphSchemas() []GraphSchema {
	return app.graphSchemas
}

func (app *ApplicationImpl) RegisterControllers(controllers ...Controller) {
	for _, c := range controllers {
		app.controllers[c.Key()] = c
	}
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

func (app *ApplicationImpl) RegisterGraphSchema(schema GraphSchema) {
	app.graphSchemas = append(app.graphSchemas, schema)
}

func (app *ApplicationImpl) RegisterLocaleFiles(fs ...*embed.FS) {
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

func (app *ApplicationImpl) RegisterMigrationDirs(fs ...*embed.FS) {
	app.migrationDirs = append(app.migrationDirs, fs...)
}

func (app *ApplicationImpl) RegisterSeedFuncs(seedFuncs ...SeedFunc) {
	app.seedFuncs = append(app.seedFuncs, seedFuncs...)
}

// RegisterServices registers a new service in the application by its type
func (app *ApplicationImpl) RegisterServices(services ...interface{}) {
	for _, service := range services {
		serviceType := reflect.TypeOf(service).Elem()
		app.services[serviceType] = service
	}
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

func (app *ApplicationImpl) Bundle() *i18n.Bundle {
	return app.bundle
}

func CollectMigrations(app *ApplicationImpl) ([]*migrate.Migration, error) {
	var migrations []*migrate.Migration
	for _, migrationFs := range app.migrationDirs {
		files, err := listFiles(migrationFs, ".")
		if err != nil {
			return nil, err
		}
		for _, file := range files {
			content, err := migrationFs.ReadFile(file)
			if err != nil {
				return nil, err
			}
			migration, err := migrate.ParseMigration(filepath.Join(file), bytes.NewReader(content))
			if err != nil {
				return nil, err
			}
			migrations = append(migrations, migration)
		}
	}
	return migrations, nil
}

func (app *ApplicationImpl) RunMigrations() error {
	db := stdlib.OpenDB(*app.pool.Config().ConnConfig)
	migrations, err := CollectMigrations(app)
	if err != nil {
		return err
	}
	if len(migrations) == 0 {
		log.Printf("No migrations found")
		return nil
	}
	migrationSource := &migrate.MemoryMigrationSource{
		Migrations: migrations,
	}
	ms := migrate.MigrationSet{}
	plannedMigrations, dbMap, err := ms.PlanMigration(db, "postgres", migrationSource, migrate.Up, 0)
	if err != nil {
		return err
	}
	applied := 0
	tx, err := dbMap.Begin()
	if err != nil {
		return err
	}
	defer func(tx *gorp.Transaction) {
		if err := tx.Rollback(); err != nil {
			log.Printf("Failed to rollback transaction: %v", err)
		}
	}(tx)
	for _, m := range plannedMigrations {
		for _, stmt := range m.Queries {
			stmt = strings.TrimSuffix(stmt, "\n")
			stmt = strings.TrimSuffix(stmt, " ")
			stmt = strings.TrimSuffix(stmt, ";")
			if _, err := tx.Exec(stmt); err != nil {
				return &migrate.TxError{
					Migration: m.Migration,
					Err:       err,
				}
			}
		}

		if err := tx.Insert(&migrate.MigrationRecord{
			Id:        m.Id,
			AppliedAt: time.Now(),
		}); err != nil {
			return &migrate.TxError{
				Migration: m.Migration,
				Err:       err,
			}
		}
		applied++
	}
	if err := tx.Commit(); err != nil {
		return err
	}

	log.Printf("Applied %d migrations", applied)
	return nil
}

func (app *ApplicationImpl) RollbackMigrations() error {
	db := stdlib.OpenDB(*app.pool.Config().ConnConfig)
	migrations, err := CollectMigrations(app)
	if err != nil {
		return err
	}
	if len(migrations) == 0 {
		log.Printf("No migrations found")
		return nil
	}
	migrationSource := &migrate.MemoryMigrationSource{
		Migrations: migrations,
	}
	n, err := migrate.Exec(db, "postgres", migrationSource, migrate.Down)
	if err != nil {
		return err
	}
	log.Printf("Rolled back %d migrations", n)
	return nil
}
