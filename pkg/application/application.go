// Package application provides this package.
package application

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net/url"
	"path/filepath"
	"reflect"
	goruntime "runtime"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/BurntSushi/toml"
	"github.com/benbjohnson/hashfs"
	"github.com/gorilla/mux"
	appletsconfig "github.com/iota-uz/applets/config"
	"github.com/iota-uz/go-i18n/v2/i18n"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sirupsen/logrus"
	"golang.org/x/text/language"

	"github.com/iota-uz/applets"
	coreuser "github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/permission"
	appletenginehandlers "github.com/iota-uz/iota-sdk/pkg/appletengine/handlers"
	appletengineruntime "github.com/iota-uz/iota-sdk/pkg/appletengine/runtime"
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
		services:           make(map[reflect.Type]interface{}),
		quickLinks:         quickLinks,
		spotlight:          spotlightService,
		bundle:             opts.Bundle,
		migrations:         NewMigrationManager(opts.Pool),
		supportedLanguages: opts.SupportedLanguages,
		appletRegistry:     applets.NewRegistry(),
	}
	app.RegisterRuntime(RuntimeRegistration{
		Component: newSpotlightRuntimeComponent(cfg, spotlightService),
		Tags: []RuntimeTag{
			RuntimeTagAPI,
		},
	})
	return app, nil
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
	spotlight          spotlight.Service
	quickLinks         *spotlight.QuickLinks
	migrations         MigrationManager
	navItems           []types.NavigationItem
	supportedLanguages []string
	appletRegistry     applets.Registry
	appletRuntime      *appletengineruntime.Manager
	runtimeComponents  []RuntimeRegistration
	startedRuntime     []RuntimeComponent
	currentRuntimeTags []RuntimeTag
	startingRuntime    bool
	stoppingRuntime    bool
	pendingAppletRT    []RuntimeRegistration
	pendingAppletRTMu  sync.Mutex
	runtimeMu          sync.Mutex
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

func (app *application) RegisterRuntime(registrations ...RuntimeRegistration) {
	app.runtimeMu.Lock()
	defer app.runtimeMu.Unlock()

	for _, registration := range registrations {
		if registration.Component == nil {
			continue
		}
		app.runtimeComponents = append(app.runtimeComponents, cloneRuntimeRegistration(registration))
	}
}

func (app *application) RuntimeComponents() []RuntimeRegistration {
	app.runtimeMu.Lock()
	defer app.runtimeMu.Unlock()

	components := make([]RuntimeRegistration, len(app.runtimeComponents))
	for i, registration := range app.runtimeComponents {
		components[i] = cloneRuntimeRegistration(registration)
	}
	return components
}

func (app *application) StartRuntime(ctx context.Context, tags ...RuntimeTag) error {
	normalizedTags, err := normalizeRuntimeTags(tags)
	if err != nil {
		return err
	}
	if len(normalizedTags) == 0 {
		return nil
	}
	app.runtimeMu.Lock()
	if app.startingRuntime {
		activeTags := formatRuntimeTags(app.currentRuntimeTags)
		if activeTags == "[]" {
			activeTags = formatRuntimeTags(normalizedTags)
		}
		app.runtimeMu.Unlock()
		return fmt.Errorf("runtime is starting with tags %s", activeTags)
	}
	if app.stoppingRuntime {
		app.runtimeMu.Unlock()
		return fmt.Errorf("runtime is stopping")
	}
	if app.currentRuntimeTags != nil {
		if runtimeTagsEqual(app.currentRuntimeTags, normalizedTags) {
			app.runtimeMu.Unlock()
			return nil
		}
		app.runtimeMu.Unlock()
		return fmt.Errorf("runtime already started with tags %s", formatRuntimeTags(app.currentRuntimeTags))
	}

	activeTags := runtimeTagSet(normalizedTags)
	applicable := make([]RuntimeRegistration, 0, len(app.runtimeComponents))
	for _, registration := range app.runtimeComponents {
		registrationTags, err := normalizeRuntimeTags(registration.Tags)
		if err != nil {
			app.runtimeMu.Unlock()
			return fmt.Errorf("runtime component %q: %w", registration.Component.Name(), err)
		}
		cloned := cloneRuntimeRegistration(registration)
		cloned.Tags = registrationTags
		if !cloned.AppliesTo(activeTags) {
			continue
		}
		applicable = append(applicable, cloned)
	}
	app.startingRuntime = true
	app.currentRuntimeTags = append([]RuntimeTag(nil), normalizedTags...)
	app.runtimeMu.Unlock()

	started := make([]RuntimeComponent, 0, len(applicable))
	var startErr error
	for _, registration := range applicable {
		if err := registration.Component.Start(ctx); err != nil {
			rollbackCtx, rollbackCancel := context.WithTimeout(context.Background(), 10*time.Second)
			var rollbackErr error
			for i := len(started) - 1; i >= 0; i-- {
				if stopErr := started[i].Stop(rollbackCtx); stopErr != nil {
					rollbackErr = errors.Join(
						rollbackErr,
						fmt.Errorf("rollback runtime component %q: %w", started[i].Name(), stopErr),
					)
				}
			}
			rollbackCancel()
			startErr = errors.Join(
				fmt.Errorf("start runtime component %q: %w", registration.Component.Name(), err),
				rollbackErr,
			)
			break
		}
		started = append(started, registration.Component)
	}

	app.runtimeMu.Lock()
	defer app.runtimeMu.Unlock()
	app.startingRuntime = false
	if startErr != nil {
		app.startedRuntime = nil
		app.currentRuntimeTags = nil
		return startErr
	}
	app.startedRuntime = started
	return nil
}

func (app *application) StopRuntime(ctx context.Context) error {
	app.runtimeMu.Lock()
	if app.startingRuntime {
		app.runtimeMu.Unlock()
		return fmt.Errorf("runtime is starting")
	}
	if app.stoppingRuntime {
		app.runtimeMu.Unlock()
		return fmt.Errorf("runtime is stopping")
	}
	started := append([]RuntimeComponent(nil), app.startedRuntime...)
	if app.currentRuntimeTags == nil {
		app.runtimeMu.Unlock()
		return nil
	}
	app.stoppingRuntime = true
	app.runtimeMu.Unlock()
	var stopErr error
	for i := len(started) - 1; i >= 0; i-- {
		if err := started[i].Stop(ctx); err != nil {
			stopErr = errors.Join(stopErr, fmt.Errorf("stop runtime component %q: %w", started[i].Name(), err))
		}
	}
	app.runtimeMu.Lock()
	app.startedRuntime = nil
	app.currentRuntimeTags = nil
	app.stoppingRuntime = false
	app.runtimeMu.Unlock()
	return stopErr
}

func cloneRuntimeRegistration(registration RuntimeRegistration) RuntimeRegistration {
	cloned := registration
	if len(registration.Tags) == 0 {
		cloned.Tags = nil
		return cloned
	}
	cloned.Tags = append([]RuntimeTag(nil), registration.Tags...)
	return cloned
}

// CreateAppletControllers creates controllers for all registered applets without
// starting long-lived runtime processes. Runtime registrations are staged until
// RegisterAppletRuntime is called.
func (app *application) CreateAppletControllers(
	host applets.HostServices,
	sessionConfig applets.SessionConfig,
	logger *logrus.Logger,
	metrics applets.MetricsRecorder,
	opts ...applets.BuilderOption,
) ([]Controller, error) {
	controllers, registrations, err := app.buildAppletControllersAndRuntime(host, sessionConfig, logger, metrics, opts...)
	if err != nil {
		return nil, err
	}
	app.pendingAppletRTMu.Lock()
	app.pendingAppletRT = append(app.pendingAppletRT, registrations...)
	app.pendingAppletRTMu.Unlock()
	return controllers, nil
}

func (app *application) RegisterAppletRuntime(
	host applets.HostServices,
	sessionConfig applets.SessionConfig,
	logger *logrus.Logger,
	metrics applets.MetricsRecorder,
	opts ...applets.BuilderOption,
) error {
	app.pendingAppletRTMu.Lock()
	registrations := append([]RuntimeRegistration(nil), app.pendingAppletRT...)
	app.pendingAppletRT = nil
	app.pendingAppletRTMu.Unlock()
	if len(registrations) == 0 {
		var err error
		_, registrations, err = app.buildAppletControllersAndRuntime(host, sessionConfig, logger, metrics, opts...)
		if err != nil {
			return err
		}
	}
	app.RegisterRuntime(registrations...)
	return nil
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

func resolveAppletRuntimeEntrypoint(appletName string) string {
	_, currentFile, _, ok := goruntime.Caller(0)
	if !ok {
		return filepath.Join("modules", appletName, "runtime", "index.ts")
	}
	// pkg/application/application.go -> repo root -> modules/<applet>/runtime/index.ts
	root := filepath.Clean(filepath.Join(filepath.Dir(currentFile), "..", ".."))
	return filepath.Join(root, "modules", appletName, "runtime", "index.ts")
}

type appletOverride struct {
	base   Applet
	config applets.Config
}

func (a *appletOverride) Name() string     { return a.base.Name() }
func (a *appletOverride) BasePath() string { return a.base.BasePath() }
func (a *appletOverride) Config() applets.Config {
	return a.config
}

func applyProjectAppletOverrides(base Applet, projectConfig *appletsconfig.ProjectConfig) Applet {
	if base == nil || projectConfig == nil {
		return base
	}
	appletCfg, ok := projectConfig.Applets[base.Name()]
	if !ok || appletCfg == nil {
		return base
	}

	config := base.Config()
	changed := false
	if len(appletCfg.Hosts) > 0 {
		hosts := make([]string, 0, len(appletCfg.Hosts))
		for _, host := range appletCfg.Hosts {
			host = strings.TrimSpace(host)
			if host == "" {
				continue
			}
			hosts = append(hosts, host)
		}
		if len(hosts) > 0 {
			config.Hosts = hosts
			changed = true
		}
	}
	if !changed {
		return base
	}
	return &appletOverride{base: base, config: config}
}

func appletFrontendType(projectConfig *appletsconfig.ProjectConfig, appletName string) string {
	if projectConfig == nil || strings.TrimSpace(appletName) == "" {
		return appletsconfig.FrontendTypeStatic
	}
	cfg, ok := projectConfig.Applets[appletName]
	if !ok || cfg == nil || cfg.Frontend == nil {
		return appletsconfig.FrontendTypeStatic
	}
	frontendType := strings.TrimSpace(cfg.Frontend.Type)
	if frontendType == "" {
		return appletsconfig.FrontendTypeStatic
	}
	return frontendType
}

func toBunDelegateMethodName(appletName, methodName string) (string, error) {
	appletName = strings.TrimSpace(appletName)
	methodName = strings.TrimSpace(methodName)
	if appletName == "" {
		return "", fmt.Errorf("applet name is required for bun delegate method")
	}
	if methodName == "" {
		return "", fmt.Errorf("method name is required for bun delegate method")
	}
	prefix := appletName + "."
	if !strings.HasPrefix(methodName, prefix) {
		return "", fmt.Errorf("method %q must be namespaced with %q", methodName, prefix)
	}
	suffix := strings.TrimPrefix(methodName, prefix)
	if suffix == "" {
		return "", fmt.Errorf("method %q has empty suffix", methodName)
	}
	return appletName + ".__go." + suffix, nil
}

func makeBunPublicProxyMethod(publicMethodName, goDelegateMethodName string, base applets.RPCMethod) applets.RPCMethod {
	return applets.RPCMethod{
		RequirePermissions: append([]string(nil), base.RequirePermissions...),
		Handler: func(context.Context, json.RawMessage) (any, error) {
			return nil, fmt.Errorf(
				"method %s is routed via bun runtime; use %s on internal transport",
				publicMethodName,
				goDelegateMethodName,
			)
		},
	}
}

func validateRequiredAppletSecrets(ctx context.Context, appletName string, required []string, store appletenginehandlers.SecretsStore) error {
	if len(required) == 0 {
		return nil
	}
	if store == nil {
		return fmt.Errorf("validate required secrets for %s: secrets store is required", appletName)
	}
	seen := make(map[string]struct{}, len(required))
	missing := make([]string, 0)
	for _, name := range required {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		if _, exists := seen[name]; exists {
			continue
		}
		seen[name] = struct{}{}
		_, found, err := store.Get(ctx, appletName, name)
		if err != nil {
			return fmt.Errorf("validate required secret %q for %s: %w", name, appletName, err)
		}
		if !found {
			missing = append(missing, name)
		}
	}
	if len(missing) == 0 {
		return nil
	}
	sort.Strings(missing)
	return fmt.Errorf("required secrets missing for %s: %s", appletName, strings.Join(missing, ", "))
}
