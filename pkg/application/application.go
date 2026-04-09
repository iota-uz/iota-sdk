// Package application provides this package.
package application

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/url"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"unicode"

	"github.com/BurntSushi/toml"
	"github.com/benbjohnson/hashfs"
	"github.com/gorilla/mux"
	"github.com/iota-uz/go-i18n/v2/i18n"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sirupsen/logrus"
	"golang.org/x/text/language"

	"github.com/iota-uz/applets"
	coreuser "github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/permission"
	"github.com/iota-uz/iota-sdk/pkg/configuration"
	"github.com/iota-uz/iota-sdk/pkg/di"
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
			Keywords:    append([]string(nil), item.Keywords...),
			Icon:        item.Icon,
			Permissions: item.Permissions,
			IsBeta:      item.IsBeta,
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

func (s *seeder) Seed(ctx context.Context, deps *SeedDeps) error {
	conf := configuration.Use()
	for _, seedFunc := range s.seedFuncs {
		conf.Logger().Infof("Seeding %s", reflect.TypeOf(seedFunc).Name())
		if err := seedFunc(ctx, deps); err != nil {
			return err
		}
	}
	return nil
}

func (s *seeder) Register(seedFuncs ...SeedFunc) {
	s.seedFuncs = append(s.seedFuncs, seedFuncs...)
}

// Seed wraps a function into a SeedFunc using dependency injection. The function
// must accept context.Context as the first argument and return exactly one error.
// Additional parameters are resolved from the SeedDeps provider registry.
//
// Seed panics if fn has an invalid signature. Call it at package init time or
// during registration so signature errors surface at startup, not at seed time.
func Seed(fn interface{}) SeedFunc {
	if err := validateSeedSignature(fn); err != nil {
		panic(err)
	}
	return func(ctx context.Context, deps *SeedDeps) error {
		if deps == nil {
			return fmt.Errorf("seed deps are required")
		}
		return deps.Invoke(ctx, fn)
	}
}

func (d *SeedDeps) Invoke(ctx context.Context, fn interface{}) error {
	if d == nil {
		return fmt.Errorf("seed deps are required")
	}
	results, err := di.InvokeWithProviders(ctx, fn, d.providersForInvocation()...)
	if err != nil {
		return err
	}
	if len(results) != 1 {
		return fmt.Errorf("seed function must return exactly one value")
	}
	if results[0].IsNil() {
		return nil
	}
	resultErr, ok := results[0].Interface().(error)
	if !ok {
		return fmt.Errorf("seed function must return an error")
	}
	return resultErr
}

func (d *SeedDeps) providersForInvocation() []di.Provider {
	if d == nil {
		return nil
	}

	providers := make([]di.Provider, 0, len(d.providers)+3)
	if d.Pool != nil {
		providers = append(providers, di.ValueProvider(d.Pool))
	}
	if d.EventBus != nil {
		providers = append(providers, di.ValueProvider(d.EventBus))
	}
	if d.Logger != nil {
		providers = append(providers, di.ValueProvider(d.Logger))
	}
	providers = append(providers, d.providers...)
	return providers
}

func validateSeedSignature(fn interface{}) error {
	if fn == nil {
		return fmt.Errorf("seed function is required")
	}
	fnType := reflect.TypeOf(fn)
	if fnType.Kind() != reflect.Func {
		return fmt.Errorf("seed function must be a function, got %s", fnType.Kind())
	}
	contextType := reflect.TypeOf((*context.Context)(nil)).Elem()
	if fnType.NumIn() == 0 || fnType.In(0) != contextType {
		return fmt.Errorf("seed function must accept context.Context as the first argument")
	}
	errorType := reflect.TypeOf((*error)(nil)).Elem()
	if fnType.NumOut() != 1 || !fnType.Out(0).Implements(errorType) {
		return fmt.Errorf("seed function must return exactly one error")
	}
	return nil
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

// DefaultSupportedLanguages returns the canonical UI languages supported by the SDK.
func DefaultSupportedLanguages() []string {
	return []string{
		string(coreuser.UILanguageEN),
		string(coreuser.UILanguageRU),
		string(coreuser.UILanguageUZ),
		string(coreuser.UILanguageZH),
	}
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

func New(opts *ApplicationOptions) (Application, error) {
	if opts == nil {
		return nil, fmt.Errorf("application options are required")
	}

	var engine spotlight.IndexEngine
	serviceOpts := make([]spotlight.ServiceOption, 0, 1)
	if opts.Logger != nil {
		serviceOpts = append(serviceOpts, spotlight.WithLogger(opts.Logger))
	}
	cfg := configuration.Use()
	if cfg.MeiliURL == "" {
		engine = spotlight.NewNoopEngine()
	} else {
		engine = spotlight.NewMeilisearchEngine(cfg.MeiliURL, cfg.MeiliAPIKey)
	}
	spotlightService := spotlight.NewService(
		engine,
		spotlight.NewHeuristicAgent(),
		spotlight.DefaultServiceConfig(),
		serviceOpts...,
	)
	quickLinks := spotlight.NewQuickLinks(opts.Bundle, opts.SupportedLanguages)
	spotlightService.RegisterProvider(quickLinks)
	// Inject QuickLinks into the service for in-memory fuzzy search in the fast stage.
	spotlight.WithQuickLinks(quickLinks)(spotlightService)

	app := &application{
		pool:               opts.Pool,
		eventPublisher:     opts.EventBus,
		websocket:          opts.Huber,
		controllers:        make(map[string]Controller),
		quickLinks:         quickLinks,
		spotlight:          spotlightService,
		bundle:             opts.Bundle,
		migrations:         NewMigrationManager(opts.Pool),
		supportedLanguages: opts.SupportedLanguages,
		appletRegistry:     applets.NewRegistry(),
	}
	return app, nil
}

// application with a dynamically extendable service registry
type application struct {
	pool               *pgxpool.Pool
	eventPublisher     eventbus.EventBus
	websocket          Huber
	controllers        map[string]Controller
	middleware         []mux.MiddlewareFunc
	hashFsAssets       []*hashfs.FS
	assets             []*embed.FS
	graphSchemas       []GraphSchema
	bundle             *i18n.Bundle
	spotlight          spotlight.Service
	quickLinks         *spotlight.QuickLinks
	migrations         MigrationManager
	navItems           []types.NavigationItem
	supportedLanguages []string
	appletRegistry     applets.Registry
}

func (app *application) Spotlight() spotlight.Service {
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
	app.registerNavQuickLinks(items...)
}

// AppendNavChildren finds a registered NavigationItem by name (recursively)
// and appends the given children to it. If the parent item has an Href, it is
// preserved as the first child so that the original link remains accessible
// from the dropdown.
func (app *application) AppendNavChildren(parentName string, children ...types.NavigationItem) {
	appendChildren(&app.navItems, parentName, children)
	app.registerNavQuickLinks(children...)
}

func (app *application) registerNavQuickLinks(items ...types.NavigationItem) {
	if app.quickLinks == nil {
		return
	}
	app.quickLinks.Add(navItemsToQuickLinks(items...)...)
}

func navItemsToQuickLinks(items ...types.NavigationItem) []*spotlight.QuickLink {
	out := make([]*spotlight.QuickLink, 0, len(items))
	for _, item := range items {
		if strings.TrimSpace(item.Href) != "" {
			builder := spotlight.NewQuickLinkBuilder(item.Name, item.Href).
				WithKeywords(navItemKeywords(item)...)
			if len(item.Permissions) == 0 {
				builder.Public()
			} else {
				builder.WithPermissions(navPermissionNames(item.Permissions)...)
			}
			out = append(out, builder.Build())
		}
		if len(item.Children) > 0 {
			out = append(out, navItemsToQuickLinks(item.Children...)...)
		}
	}
	return out
}

func navPermissionNames(perms []permission.Permission) []string {
	names := make([]string, 0, len(perms))
	for _, perm := range perms {
		if perm == nil {
			continue
		}
		names = append(names, perm.Name())
	}
	return names
}

func navItemKeywords(item types.NavigationItem) []string {
	keywords := make([]string, 0, len(item.Keywords)+8)
	keywords = append(keywords, item.Keywords...)
	keywords = append(keywords, splitKeywordTokens(item.Name)...)
	keywords = append(keywords, hrefKeywords(item.Href)...)
	return keywords
}

func hrefKeywords(href string) []string {
	trimmed := strings.TrimSpace(href)
	if trimmed == "" {
		return nil
	}
	u, err := url.Parse(trimmed)
	if err != nil {
		return splitKeywordTokens(trimmed)
	}
	keywords := splitKeywordTokens(u.Path)
	for key, values := range u.Query() {
		keywords = append(keywords, splitKeywordTokens(key)...)
		for _, value := range values {
			keywords = append(keywords, splitKeywordTokens(value)...)
		}
	}
	return keywords
}

func splitKeywordTokens(value string) []string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	parts := strings.FieldsFunc(value, func(r rune) bool {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return false
		}
		return true
	})
	keywords := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed == "" || len(trimmed) <= 1 {
			continue
		}
		keywords = append(keywords, strings.ToLower(trimmed))
	}
	return keywords
}

func appendChildren(items *[]types.NavigationItem, parentName string, children []types.NavigationItem) bool {
	for i := range *items {
		if (*items)[i].Name == parentName {
			(*items)[i].Children = append((*items)[i].Children, children...)
			if (*items)[i].Href != "" {
				(*items)[i].Href = ""
			}
			return true
		}
		if len((*items)[i].Children) > 0 {
			if appendChildren(&(*items)[i].Children, parentName, children) {
				return true
			}
		}
	}
	return false
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

// CreateAppletControllers creates controllers for all registered applets without
// starting long-lived runtime processes.
func (app *application) CreateAppletControllers(
	host applets.HostServices,
	sessionConfig applets.SessionConfig,
	logger *logrus.Logger,
	metrics applets.MetricsRecorder,
	opts ...applets.BuilderOption,
) ([]Controller, error) {
	return app.buildAppletControllers(host, sessionConfig, logger, metrics, opts...)
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

// Bundle returns the translation bundle registered with the application.
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
