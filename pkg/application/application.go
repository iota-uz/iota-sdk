package application

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"path/filepath"
	"reflect"
	"sort"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/benbjohnson/hashfs"
	"github.com/gorilla/mux"
	"github.com/iota-uz/go-i18n/v2/i18n"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sirupsen/logrus"
	"golang.org/x/text/language"

	"github.com/iota-uz/applets"
	"github.com/iota-uz/iota-sdk/pkg/configuration"
	"github.com/iota-uz/iota-sdk/pkg/eventbus"
	"github.com/iota-uz/iota-sdk/pkg/intl"
	"github.com/iota-uz/iota-sdk/pkg/spotlight"
	"github.com/iota-uz/iota-sdk/pkg/types"
)

func translate(localizer *i18n.Localizer, items []types.NavigationItem) []types.NavigationItem {
	translated := make([]types.NavigationItem, 0, len(items))
	for _, item := range items {
		translated = append(translated, types.NavigationItem{
			Name: intl.MustLocalize(localizer, &i18n.LocalizeConfig{
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

// ---- Applet Registry implementation ----
// Now uses pkg/applets.Registry directly for unified applet management

// ---- Application implementation ----

type ApplicationOptions struct {
	Pool               *pgxpool.Pool
	EventBus           eventbus.EventBus
	Logger             *logrus.Logger
	Bundle             *i18n.Bundle
	Huber              Huber
	SupportedLanguages []string
}

func LoadBundle() *i18n.Bundle {
	bundle := i18n.NewBundle(language.Russian)
	bundle.RegisterUnmarshalFunc("json", json.Unmarshal)
	bundle.RegisterUnmarshalFunc("toml", toml.Unmarshal)
	return bundle
}

// LoadBundleFromLocaleFiles creates a bundle and loads all message files from the given embed.FS slices.
// Use this to build a bundle from locale files only (e.g. for tests without DB or config).
func LoadBundleFromLocaleFiles(fs ...*embed.FS) *i18n.Bundle {
	bundle := LoadBundle()
	for _, localeFs := range fs {
		loadLocaleFSIntoBundle(bundle, localeFs)
	}
	return bundle
}

// loadLocaleFSIntoBundle reads all files from the given embed.FS and parses them into the bundle.
func loadLocaleFSIntoBundle(bundle *i18n.Bundle, localeFs *embed.FS) {
	files, err := listFiles(localeFs, ".")
	if err != nil {
		panic(err)
	}
	for _, file := range files {
		localeFile, err := localeFs.ReadFile(file)
		if err != nil {
			panic(err)
		}
		bundle.MustParseMessageFileBytes(localeFile, filepath.Base(file))
	}
}

func New(opts *ApplicationOptions) Application {
	sl := spotlight.New()
	quickLinks := &spotlight.QuickLinks{}
	sl.Register(quickLinks)

	return &application{
		pool:               opts.Pool,
		eventPublisher:     opts.EventBus,
		websocket:          opts.Huber,
		controllers:        make(map[string]Controller),
		services:           make(map[reflect.Type]interface{}),
		quickLinks:         quickLinks,
		spotlight:          sl,
		bundle:             opts.Bundle,
		migrations:         NewMigrationManager(opts.Pool),
		supportedLanguages: opts.SupportedLanguages,
		appletRegistry:     applets.NewRegistry(),
	}
}

// application with a dynamically extendable service registry
type application struct {
	pool               *pgxpool.Pool
	eventPublisher     eventbus.EventBus
	websocket          Huber
	services           map[reflect.Type]interface{}
	controllers        map[string]Controller
	middleware         []mux.MiddlewareFunc
	hashFsAssets       []*hashfs.FS
	assets             []*embed.FS
	graphSchemas       []GraphSchema
	bundle             *i18n.Bundle
	spotlight          spotlight.Spotlight
	quickLinks         *spotlight.QuickLinks
	migrations         MigrationManager
	navItems           []types.NavigationItem
	supportedLanguages []string
	appletRegistry     applets.Registry
}

func (app *application) Spotlight() spotlight.Spotlight {
	return app.spotlight
}

func (app *application) Websocket() Huber {
	return app.websocket
}

func (app *application) QuickLinks() *spotlight.QuickLinks {
	return app.quickLinks
}

func (app *application) NavItems(localizer *i18n.Localizer) []types.NavigationItem {
	return translate(localizer, app.navItems)
}

func (app *application) RegisterNavItems(items ...types.NavigationItem) {
	app.navItems = append(app.navItems, items...)
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
	// Register applet controllers first so their asset routes (e.g. /admin/ali/chat/assets)
	// are added before other controllers that might match /admin/... and return 404 for
	// dev proxy requests (@vite/client, /src/*, etc.).
	sort.Slice(controllers, func(i, j int) bool {
		ki, kj := controllers[i].Key(), controllers[j].Key()
		appletI := strings.HasPrefix(ki, "applet_")
		appletJ := strings.HasPrefix(kj, "applet_")
		if appletI != appletJ {
			return appletI
		}
		return ki < kj
	})
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
		if c == nil {
			continue
		}
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
		loadLocaleFSIntoBundle(app.bundle, localeFs)
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

func (app *application) GetSupportedLanguages() []string {
	return app.supportedLanguages
}

func (app *application) RegisterApplet(a Applet) error {
	return app.appletRegistry.Register(a)
}

func (app *application) AppletRegistry() AppletRegistry {
	return app.appletRegistry
}

// CreateAppletControllers creates controllers for all registered applets.
// This provides a single mounting point for all applets in the application.
//
// Parameters:
//   - host: Host services for extracting user, tenant, pool, locale from request context
//   - sessionConfig: Session configuration for context building
//   - logger: Logger for applet operations
//   - metrics: Metrics recorder (can be nil)
//   - opts: Optional builder options (e.g., WithTenantNameResolver, WithErrorEnricher)
//
// Returns a slice of controllers that can be registered via RegisterControllers().
//
// Example usage:
//
//	controllers, err := app.CreateAppletControllers(
//		hostServices,
//		applets.DefaultSessionConfig,
//		logger,
//		metrics,
//	)
//	if err != nil {
//		return err
//	}
//	app.RegisterControllers(controllers...)
func (app *application) CreateAppletControllers(
	host applets.HostServices,
	sessionConfig applets.SessionConfig,
	logger *logrus.Logger,
	metrics applets.MetricsRecorder,
	opts ...applets.BuilderOption,
) ([]Controller, error) {
	registry := app.AppletRegistry()
	allApplets := registry.All()

	controllers := make([]Controller, 0, len(allApplets))
	for _, a := range allApplets {
		controller, err := applets.NewAppletController(
			a,
			app.Bundle(),
			sessionConfig,
			logger,
			metrics,
			host,
			opts...,
		)
		if err != nil {
			return nil, err
		}
		controllers = append(controllers, controller)
	}

	return controllers, nil
}
