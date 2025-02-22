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

	"github.com/benbjohnson/hashfs"
	"github.com/go-gorp/gorp/v3"
	"github.com/gorilla/mux"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/nicksnyder/go-i18n/v2/i18n"
	migrate "github.com/rubenv/sql-migrate"
	"golang.org/x/text/language"

	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/permission"
	"github.com/iota-uz/iota-sdk/pkg/eventbus"
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
	for _, seedFunc := range s.seedFuncs {
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
	return &application{
		pool:           pool,
		eventPublisher: eventPublisher,
		rbac:           permission.NewRbac(),
		controllers:    make(map[string]Controller),
		services:       make(map[reflect.Type]interface{}),
		spotlight:      spotlight.New(),
		bundle:         bundle,
	}
}

// application with a dynamically extendable service registry
type application struct {
	pool           *pgxpool.Pool
	eventPublisher eventbus.EventBus
	rbac           permission.RBAC
	services       map[reflect.Type]interface{}
	controllers    map[string]Controller
	middleware     []mux.MiddlewareFunc
	hashFsAssets   []*hashfs.FS
	assets         []*embed.FS
	migrationDirs  []*embed.FS
	graphSchemas   []GraphSchema
	bundle         *i18n.Bundle
	spotlight      spotlight.Spotlight
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

func (app *application) RBAC() permission.RBAC {
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

func (app *application) MigrationDirs() []*embed.FS {
	return app.migrationDirs
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

func (app *application) RegisterMigrationDirs(fs ...*embed.FS) {
	app.migrationDirs = append(app.migrationDirs, fs...)
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

func (app *application) Bundle() *i18n.Bundle {
	return app.bundle
}

func CollectMigrations(app *application) ([]*migrate.Migration, error) {
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

func newTxError(migration *migrate.PlannedMigration, err error) *migrate.TxError {
	return &migrate.TxError{
		Migration: migration.Migration,
		Err:       err,
	}
}

func (app *application) applyMigrations(ctx context.Context, dir migrate.MigrationDirection, migrations []*migrate.PlannedMigration, dbMap *gorp.DbMap) (int, error) {
	applied := 0
	for _, migration := range migrations {
		e, err := dbMap.Begin()
		if err != nil {
			return applied, newTxError(migration, err)
		}
		executor := e.WithContext(ctx)

		for _, stmt := range migration.Queries {
			// remove the semicolon from stmt, fix ORA-00922 issue in database oracle
			stmt = strings.TrimSuffix(stmt, "\n")
			stmt = strings.TrimSuffix(stmt, " ")
			stmt = strings.TrimSuffix(stmt, ";")
			if _, err := executor.Exec(stmt); err != nil {
				if trans, ok := executor.(*gorp.Transaction); ok {
					_ = trans.Rollback()
				}

				return applied, newTxError(migration, err)
			}
		}

		switch dir {
		case migrate.Up:
			err = executor.Insert(&migrate.MigrationRecord{
				Id:        migration.Id,
				AppliedAt: time.Now(),
			})
			if err != nil {
				if trans, ok := executor.(*gorp.Transaction); ok {
					_ = trans.Rollback()
				}

				return applied, newTxError(migration, err)
			}
		case migrate.Down:
			_, err := executor.Delete(&migrate.MigrationRecord{
				Id: migration.Id,
			})
			if err != nil {
				if trans, ok := executor.(*gorp.Transaction); ok {
					_ = trans.Rollback()
				}

				return applied, newTxError(migration, err)
			}
		default:
			panic("Not possible")
		}

		if trans, ok := executor.(*gorp.Transaction); ok {
			if err := trans.Commit(); err != nil {
				return applied, newTxError(migration, err)
			}
		}

		applied++
	}

	return applied, nil
}

// Internal in-memory migration source that respects the order of migrations.
type memoryMigrationSourceInternal struct {
	Migrations []*migrate.Migration
}

// FindMigrations returns the list of unsorted migrations. Giving up determenistic order in favor of
// the order in which the migrations were added.
func (m *memoryMigrationSourceInternal) FindMigrations() ([]*migrate.Migration, error) {
	migrations := make([]*migrate.Migration, len(m.Migrations))
	copy(migrations, m.Migrations)
	return migrations, nil
}

func (app *application) RunMigrations() error {
	db := stdlib.OpenDB(*app.pool.Config().ConnConfig)
	migrations, err := CollectMigrations(app)
	if err != nil {
		return err
	}
	if len(migrations) == 0 {
		log.Printf("No migrations found")
		return nil
	}
	migrationSource := &memoryMigrationSourceInternal{
		Migrations: migrations,
	}
	ms := migrate.MigrationSet{}
	plannedMigrations, dbMap, err := ms.PlanMigration(db, "postgres", migrationSource, migrate.Up, 0)
	if err != nil {
		return err
	}

	applied, err := app.applyMigrations(context.Background(), migrate.Up, plannedMigrations, dbMap)
	if err != nil {
		return err
	}
	log.Printf("Applied %d migrations", applied)
	return nil
}

func (app *application) RollbackMigrations() error {
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
	plannedMigrations, dbMap, err := ms.PlanMigration(db, "postgres", migrationSource, migrate.Down, 0)
	if err != nil {
		return err
	}

	applied, err := app.applyMigrations(context.Background(), migrate.Down, plannedMigrations, dbMap)
	if err != nil {
		return err
	}
	log.Printf("Rolled back %d migrations", applied)
	return nil
}
