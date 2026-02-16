package application

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	goruntime "runtime"
	"sort"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/benbjohnson/hashfs"
	"github.com/gorilla/mux"
	appletsconfig "github.com/iota-uz/applets/config"
	"github.com/iota-uz/go-i18n/v2/i18n"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sirupsen/logrus"
	"golang.org/x/text/language"

	"github.com/iota-uz/applets"
	appletenginecontrollers "github.com/iota-uz/iota-sdk/pkg/appletengine/controllers"
	appletenginehandlers "github.com/iota-uz/iota-sdk/pkg/appletengine/handlers"
	appletenginejobs "github.com/iota-uz/iota-sdk/pkg/appletengine/jobs"
	appletenginerpc "github.com/iota-uz/iota-sdk/pkg/appletengine/rpc"
	appletengineruntime "github.com/iota-uz/iota-sdk/pkg/appletengine/runtime"
	appletenginewsbridge "github.com/iota-uz/iota-sdk/pkg/appletengine/wsbridge"
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

func shouldSkipSpotlightPreflight() bool {
	value := strings.TrimSpace(os.Getenv("IOTA_SKIP_SPOTLIGHT_PREFLIGHT"))
	return value == "1" || strings.EqualFold(value, "true")
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
	initCtx, initCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer initCancel()

	skipSpotlightPreflight := shouldSkipSpotlightPreflight()
	var engine spotlight.IndexEngine
	serviceOpts := make([]spotlight.ServiceOption, 0, 1)
	if opts.Logger != nil {
		serviceOpts = append(serviceOpts, spotlight.WithLogger(opts.Logger))
	}
	if opts.Pool == nil || skipSpotlightPreflight {
		engine = spotlight.NewNoopEngine()
	} else {
		if err := spotlight.PreflightCheck(initCtx, opts.Pool); err != nil {
			return nil, fmt.Errorf("spotlight preflight check: %w", err)
		}
		pgEngine := spotlight.NewPostgresPGTextSearchEngine(opts.Pool)
		engine = pgEngine
		serviceOpts = append(serviceOpts, spotlight.WithOutboxProcessor(
			spotlight.NewPostgresOutboxProcessor(opts.Pool, pgEngine),
		))
	}
	spotlightService := spotlight.NewService(
		engine,
		spotlight.NewHeuristicAgent(),
		spotlight.DefaultServiceConfig(),
		serviceOpts...,
	)
	if err := spotlightService.Start(initCtx); err != nil {
		return nil, fmt.Errorf("start spotlight service: %w", err)
	}
	quickLinks := spotlight.NewQuickLinks()
	spotlightService.RegisterProvider(quickLinks)

	return &application{
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
	}, nil
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
	rpcRegistry := appletenginerpc.NewRegistry()
	var wsBridge *appletenginewsbridge.Bridge
	ssrApplets := make([]Applet, 0)

	_, projectConfig, loadConfigErr := appletsconfig.LoadFromCWD()
	if loadConfigErr != nil {
		if errors.Is(loadConfigErr, appletsconfig.ErrConfigNotFound) {
			if logger != nil {
				logger.WithError(loadConfigErr).Warn("applet engine config not found; using applet defaults")
			}
			projectConfig = nil
		} else {
			return nil, fmt.Errorf("load applet config from .applets/config.toml: %w", loadConfigErr)
		}
	}

	engineByApplet := make(map[string]appletsconfig.AppletEngineConfig)
	for _, a := range allApplets {
		if projectConfig == nil {
			continue
		}
		appletCfg, ok := projectConfig.Applets[a.Name()]
		if !ok || appletCfg == nil || appletCfg.Engine == nil {
			continue
		}
		engineByApplet[a.Name()] = projectConfig.EffectiveEngineConfig(a.Name())
	}

	controllers := make([]Controller, 0, len(allApplets)+1)
	for _, a := range allApplets {
		a = applyProjectAppletOverrides(a, projectConfig)
		cfg := a.Config()
		engineCfg, hasEngineConfig := engineByApplet[a.Name()]
		bunRuntimeEnabled := hasEngineConfig && appletengineruntime.EnabledForEngineConfig(engineCfg)
		bunDelegateMode := bunRuntimeEnabled && a.Name() == "bichat"
		if cfg.RPC != nil {
			for methodName, method := range cfg.RPC.Methods {
				publicMethod := method
				if bunDelegateMode {
					goDelegateMethodName, err := toBunDelegateMethodName(a.Name(), methodName)
					if err != nil {
						return nil, err
					}
					if err := rpcRegistry.RegisterServerOnly(a.Name(), goDelegateMethodName, method, cfg.Middleware); err != nil {
						return nil, err
					}
					publicMethod = makeBunPublicProxyMethod(methodName, goDelegateMethodName, method)
				}
				if err := rpcRegistry.RegisterPublic(a.Name(), methodName, publicMethod, cfg.Middleware); err != nil {
					return nil, err
				}
			}
		}
		if appletFrontendType(projectConfig, a.Name()) == appletsconfig.FrontendTypeSSR {
			ssrApplets = append(ssrApplets, a)
			continue
		}

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

	fileStoreByApplet := make(map[string]appletenginehandlers.FilesStore)
	if len(engineByApplet) > 0 {
		wsBridge = appletenginewsbridge.New(logger)
		wsStub := appletenginehandlers.NewWSStub(wsBridge)
		for appletName, engineCfg := range engineByApplet {
			kvStub := appletenginehandlers.NewKVStub()
			if engineCfg.Backends.KV == appletsconfig.KVBackendRedis {
				redisKVStore, err := appletenginehandlers.NewRedisKVStore(engineCfg.Redis.URL)
				if err != nil {
					return nil, fmt.Errorf("configure redis kv store for %s: %w", appletName, err)
				}
				kvStub = appletenginehandlers.NewKVStubWithStore(redisKVStore)
			}
			if err := kvStub.Register(rpcRegistry, appletName); err != nil {
				return nil, err
			}

			dbStub := appletenginehandlers.NewDBStub()
			if engineCfg.Backends.DB == appletsconfig.DBBackendPostgres {
				if err := validateAppletSchemaArtifact(context.Background(), app.DB(), appletName); err != nil {
					return nil, err
				}
				postgresDBStore, err := appletenginehandlers.NewPostgresDBStore(app.DB())
				if err != nil {
					return nil, fmt.Errorf("configure postgres db store for %s: %w", appletName, err)
				}
				dbStub = appletenginehandlers.NewDBStubWithStore(postgresDBStore)
			}
			if err := dbStub.Register(rpcRegistry, appletName); err != nil {
				return nil, err
			}

			jobsStub := appletenginehandlers.NewJobsStub()
			if engineCfg.Backends.Jobs == appletsconfig.JobsBackendPostgres {
				postgresJobsStore, err := appletenginehandlers.NewPostgresJobsStore(app.DB())
				if err != nil {
					return nil, fmt.Errorf("configure postgres jobs store for %s: %w", appletName, err)
				}
				jobsStub = appletenginehandlers.NewJobsStubWithStore(postgresJobsStore)
			}
			if err := jobsStub.Register(rpcRegistry, appletName); err != nil {
				return nil, err
			}

			filesStore := appletenginehandlers.NewLocalFilesStore(strings.TrimSpace(engineCfg.Files.Dir))
			switch engineCfg.Backends.Files {
			case appletsconfig.FilesBackendPostgres:
				postgresFilesStore, err := appletenginehandlers.NewPostgresFilesStore(
					app.DB(),
					strings.TrimSpace(engineCfg.Files.Dir),
				)
				if err != nil {
					return nil, fmt.Errorf("configure postgres files store for %s: %w", appletName, err)
				}
				filesStore = postgresFilesStore
			case appletsconfig.FilesBackendS3:
				accessKey := strings.TrimSpace(os.Getenv(strings.TrimSpace(engineCfg.S3.AccessKeyEnv)))
				secretKey := strings.TrimSpace(os.Getenv(strings.TrimSpace(engineCfg.S3.SecretKeyEnv)))
				s3FilesStore, err := appletenginehandlers.NewS3FilesStore(app.DB(), appletenginehandlers.S3FilesConfig{
					Bucket:          strings.TrimSpace(engineCfg.S3.Bucket),
					Region:          strings.TrimSpace(engineCfg.S3.Region),
					Endpoint:        strings.TrimSpace(engineCfg.S3.Endpoint),
					AccessKeyID:     accessKey,
					SecretAccessKey: secretKey,
					ForcePathStyle:  engineCfg.S3.ForcePathStyle,
				})
				if err != nil {
					return nil, fmt.Errorf("configure s3 files store for %s: %w", appletName, err)
				}
				filesStore = s3FilesStore
			}
			fileStoreByApplet[appletName] = filesStore
			filesStub := appletenginehandlers.NewFilesStubWithStore(filesStore)
			if err := filesStub.Register(rpcRegistry, appletName); err != nil {
				return nil, err
			}

			secretsStore := appletenginehandlers.NewEnvSecretsStore()
			if engineCfg.Backends.Secrets == appletsconfig.SecretsBackendPostgres {
				masterKeyPayload, readErr := os.ReadFile(strings.TrimSpace(engineCfg.Secrets.MasterKeyFile))
				if readErr != nil {
					return nil, fmt.Errorf("read %s secrets master key file: %w", appletName, readErr)
				}
				postgresSecretsStore, err := appletenginehandlers.NewPostgresSecretsStore(
					app.DB(),
					strings.TrimSpace(string(masterKeyPayload)),
				)
				if err != nil {
					return nil, fmt.Errorf("configure postgres secrets store for %s: %w", appletName, err)
				}
				secretsStore = postgresSecretsStore
			}
			if err := validateRequiredAppletSecrets(context.Background(), appletName, engineCfg.Secrets.Required, secretsStore); err != nil {
				return nil, err
			}
			secretsStub := appletenginehandlers.NewSecretsStubWithStore(secretsStore)
			if err := secretsStub.Register(rpcRegistry, appletName); err != nil {
				return nil, err
			}

			if err := wsStub.Register(rpcRegistry, appletName); err != nil {
				return nil, err
			}
		}
	}

	if rpcRegistry.CountPublic() > 0 {
		dispatcher := appletenginerpc.NewDispatcher(rpcRegistry, host, logger)
		var runtimeManager *appletengineruntime.Manager

		// Bun runtime process plumbing for applets with runtime=bun.
		runtimeEnabledByApplet := make(map[string]appletsconfig.AppletEngineConfig)
		for appletName, engineCfg := range engineByApplet {
			if appletengineruntime.EnabledForEngineConfig(engineCfg) {
				runtimeEnabledByApplet[appletName] = engineCfg
			}
		}
		if len(runtimeEnabledByApplet) > 0 {
			runtimeManager = appletengineruntime.NewManager("", dispatcher, logger)
			dispatcher.SetBunPublicCaller(runtimeManager)
			effectiveBunBin := ""
			for _, engineCfg := range runtimeEnabledByApplet {
				if effectiveBunBin == "" {
					effectiveBunBin = engineCfg.BunBin
				}
				if strings.TrimSpace(engineCfg.BunBin) != "" && strings.TrimSpace(effectiveBunBin) != strings.TrimSpace(engineCfg.BunBin) {
					return nil, fmt.Errorf("runtime bun_bin mismatch across enabled applets")
				}
			}
			runtimeManager.SetBunBin(effectiveBunBin)

			for appletName := range runtimeEnabledByApplet {
				if err := rpcRegistry.SetPublicTargetForApplet(appletName, appletenginerpc.MethodTargetBun); err != nil {
					return nil, fmt.Errorf("set bun rpc target for %s: %w", appletName, err)
				}
				entrypoint := resolveAppletRuntimeEntrypoint(appletName)
				runtimeManager.RegisterApplet(appletName, entrypoint)
				if store, ok := fileStoreByApplet[appletName]; ok && store != nil {
					runtimeManager.RegisterFileStore(appletName, store)
				}
			}
			dispatcher.SetBeforeDispatch(func(ctx context.Context, appletName string) error {
				if _, ok := runtimeEnabledByApplet[appletName]; !ok {
					return nil
				}
				_, err := runtimeManager.EnsureStarted(ctx, appletName, "")
				return err
			})
			hasPostgresJobs := false
			for _, engineCfg := range runtimeEnabledByApplet {
				if engineCfg.Backends.Jobs == appletsconfig.JobsBackendPostgres {
					hasPostgresJobs = true
					break
				}
			}
			if hasPostgresJobs && app.DB() != nil {
				runner, err := appletenginejobs.NewRunner(app.DB(), runtimeManager, logger, 2*time.Second)
				if err != nil {
					return nil, fmt.Errorf("create applet jobs runner: %w", err)
				}
				jobCtx, jobCancel := context.WithCancel(context.Background())
				runtimeManager.SetJobCancel(jobCancel)
				go runner.Start(jobCtx)
			}
			if wsBridge != nil {
				wsBridge.SetRuntimeManager(runtimeManager)
			}
			app.appletRuntime = runtimeManager
		}

		if len(ssrApplets) > 0 {
			if runtimeManager == nil {
				return nil, fmt.Errorf("ssr applets require bun runtime manager")
			}
			for _, applet := range ssrApplets {
				entrypoint := resolveAppletRuntimeEntrypoint(applet.Name())
				runtimeManager.RegisterApplet(applet.Name(), entrypoint)
				controllers = append(controllers, appletenginecontrollers.NewSSRController(applet, runtimeManager, host, logger, entrypoint))
			}
		}

		controllers = append(controllers, appletenginecontrollers.NewRPCController(dispatcher))
		if wsBridge != nil {
			controllers = append(controllers, appletenginecontrollers.NewWSController(wsBridge, logger))
		}
	}

	return controllers, nil
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
