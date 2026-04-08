package applet

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	goruntime "runtime"
	"sort"
	"strings"

	"github.com/gorilla/mux"
	"github.com/iota-uz/applets"
	appletsconfig "github.com/iota-uz/applets/config"
	"github.com/iota-uz/go-i18n/v2/i18n"
	appletenginecontrollers "github.com/iota-uz/iota-sdk/pkg/appletengine/controllers"
	appletenginehandlers "github.com/iota-uz/iota-sdk/pkg/appletengine/handlers"
	appletenginerpc "github.com/iota-uz/iota-sdk/pkg/appletengine/rpc"
	appletengineruntime "github.com/iota-uz/iota-sdk/pkg/appletengine/runtime"
	appletenginewsbridge "github.com/iota-uz/iota-sdk/pkg/appletengine/wsbridge"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sirupsen/logrus"
)

type Controller interface {
	Register(*mux.Router)
	Key() string
}

type BuildInput struct {
	Applets       []applets.Applet
	Pool          *pgxpool.Pool
	Bundle        *i18n.Bundle
	Host          applets.HostServices
	SessionConfig applets.SessionConfig
	Logger        *logrus.Logger
	Metrics       applets.MetricsRecorder
	Options       []applets.BuilderOption
	ProjectRoot   string
	ProjectConfig *appletsconfig.ProjectConfig
}

type BuildResult struct {
	Controllers          []Controller
	RuntimeRegistrations []RuntimeRegistration
	RuntimeManager       *appletengineruntime.Manager
	RPCRegistry          *appletenginerpc.Registry
	WSBridge             *appletenginewsbridge.Bridge
	Errors               []error
}

type RuntimeRegistration struct {
	Manager         *appletengineruntime.Manager
	HasPostgresJobs bool
}

type BuildContext struct {
	AppletName    string
	Applet        applets.Applet
	ProjectRoot   string
	ProjectConfig *appletsconfig.ProjectConfig
	EngineConfig  appletsconfig.AppletEngineConfig
	Pool          *pgxpool.Pool
	Logger        *logrus.Logger
}

type Container struct {
	context BuildContext
}

func (c *Container) Context() BuildContext {
	if c == nil {
		return BuildContext{}
	}
	return c.context
}

type BunBinResolver interface {
	Resolve(map[string]appletsconfig.AppletEngineConfig) (string, error)
}

type DefaultBunBinResolver struct{}

func (DefaultBunBinResolver) Resolve(configs map[string]appletsconfig.AppletEngineConfig) (string, error) {
	effective := ""
	for _, name := range sortedAppletNames(configs) {
		bunBin := strings.TrimSpace(configs[name].BunBin)
		if bunBin == "" {
			continue
		}
		if effective == "" {
			effective = bunBin
			continue
		}
		if effective != bunBin {
			return "", fmt.Errorf("runtime bun_bin mismatch across enabled applets")
		}
	}
	return effective, nil
}

type AppletEngineBuilder struct {
	backends       *BackendRegistry
	bunBinResolver BunBinResolver
}

func NewAppletEngineBuilder() *AppletEngineBuilder {
	return &AppletEngineBuilder{
		backends:       DefaultBackendRegistry(),
		bunBinResolver: DefaultBunBinResolver{},
	}
}

func (b *AppletEngineBuilder) Backends() *BackendRegistry {
	if b == nil {
		return nil
	}
	return b.backends
}

func (b *AppletEngineBuilder) SetBunBinResolver(resolver BunBinResolver) {
	if b == nil || resolver == nil {
		return
	}
	b.bunBinResolver = resolver
}

func (b *AppletEngineBuilder) Build(input BuildInput) (BuildResult, error) {
	state := buildState{
		input:          input,
		rpcRegistry:    appletenginerpc.NewRegistry(),
		fileStoreByApp: make(map[string]appletenginehandlers.FilesStore),
	}
	if b == nil {
		return BuildResult{}, fmt.Errorf("composition applet: builder is nil")
	}
	if b.backends == nil {
		b.backends = DefaultBackendRegistry()
	}
	if b.bunBinResolver == nil {
		b.bunBinResolver = DefaultBunBinResolver{}
	}
	if err := b.loadProjectConfig(&state); err != nil {
		return BuildResult{}, err
	}
	if err := b.planAppletSpecs(&state); err != nil {
		return BuildResult{}, err
	}
	if err := b.registerAppletRPCMethods(&state); err != nil {
		return BuildResult{}, err
	}
	if err := b.validateAllBackends(&state); err != nil {
		return BuildResult{}, err
	}
	if err := b.buildBackends(&state); err != nil {
		return BuildResult{}, err
	}
	if err := b.registerStubs(&state); err != nil {
		return BuildResult{}, err
	}
	if err := b.buildAppletControllers(&state); err != nil {
		return BuildResult{}, err
	}
	if err := b.maybeBuildBunRuntime(&state); err != nil {
		return BuildResult{}, err
	}
	if err := b.assembleDispatcher(&state); err != nil {
		return BuildResult{}, err
	}
	if err := b.buildSSRControllers(&state); err != nil {
		return BuildResult{}, err
	}
	if err := b.assembleControllers(&state); err != nil {
		return BuildResult{}, err
	}
	if err := b.buildRuntimeRegistrations(&state); err != nil {
		return BuildResult{}, err
	}
	return BuildResult{
		Controllers:          state.controllers,
		RuntimeRegistrations: state.runtimeRegistrations,
		RuntimeManager:       state.runtimeManager,
		RPCRegistry:          state.rpcRegistry,
		WSBridge:             state.wsBridge,
		Errors:               state.errors,
	}, nil
}

type buildState struct {
	input BuildInput

	projectRoot   string
	projectConfig *appletsconfig.ProjectConfig
	specs         []appletSpec
	ssrSpecs      []appletSpec
	rpcRegistry   *appletenginerpc.Registry
	wsBridge      *appletenginewsbridge.Bridge

	fileStoreByApp      map[string]appletenginehandlers.FilesStore
	runtimeEnabled      map[string]appletsconfig.AppletEngineConfig
	effectiveRuntimeBin string
	dispatcher          *appletenginerpc.Dispatcher
	runtimeManager      *appletengineruntime.Manager
	hasPostgresJobs     bool

	controllers          []Controller
	runtimeRegistrations []RuntimeRegistration
	errors               []error
}

type appletSpec struct {
	applet          applets.Applet
	frontendType    string
	hasEngineConfig bool
	engineConfig    appletsconfig.AppletEngineConfig
	bunDelegateMode bool
	entrypoint      string
	backends        builtBackends
}

type builtBackends struct {
	kv      appletenginehandlers.KVStore
	db      appletenginehandlers.DBStore
	jobs    appletenginehandlers.JobsStore
	files   appletenginehandlers.FilesStore
	secrets appletenginehandlers.SecretsStore
}

func (b *AppletEngineBuilder) loadProjectConfig(state *buildState) error {
	if state.input.ProjectConfig != nil {
		state.projectRoot = strings.TrimSpace(state.input.ProjectRoot)
		state.projectConfig = state.input.ProjectConfig
		return nil
	}
	root, projectConfig, err := appletsconfig.LoadFromCWD()
	if err != nil {
		if errors.Is(err, appletsconfig.ErrConfigNotFound) {
			if state.input.Logger != nil {
				state.input.Logger.WithError(err).Warn("applet engine config not found; using applet defaults")
			}
			return nil
		}
		return err
	}
	state.projectRoot = root
	state.projectConfig = projectConfig
	return nil
}

func (b *AppletEngineBuilder) planAppletSpecs(state *buildState) error {
	specs := make([]appletSpec, 0, len(state.input.Applets))
	for _, applet := range state.input.Applets {
		if applet == nil {
			continue
		}
		effectiveApplet := applyProjectAppletOverrides(applet, state.projectConfig)
		spec := appletSpec{
			applet:       effectiveApplet,
			frontendType: appletFrontendType(state.projectConfig, applet.Name()),
			entrypoint:   resolveAppletRuntimeEntrypoint(state.projectRoot, applet.Name()),
		}
		if state.projectConfig != nil {
			if cfg, ok := state.projectConfig.Applets[applet.Name()]; ok && cfg != nil && cfg.Engine != nil {
				spec.hasEngineConfig = true
				spec.engineConfig = state.projectConfig.EffectiveEngineConfig(applet.Name())
				spec.bunDelegateMode = appletengineruntime.EnabledForEngineConfig(spec.engineConfig) && applet.Name() == "bichat"
			}
		}
		specs = append(specs, spec)
	}
	state.specs = specs
	return nil
}

func (b *AppletEngineBuilder) registerAppletRPCMethods(state *buildState) error {
	for _, spec := range state.specs {
		cfg := spec.applet.Config()
		if cfg.RPC != nil {
			for methodName, method := range cfg.RPC.Methods {
				publicMethod := method
				if spec.bunDelegateMode {
					goDelegateMethodName, err := toBunDelegateMethodName(spec.applet.Name(), methodName)
					if err != nil {
						return err
					}
					if err := state.rpcRegistry.RegisterServerOnly(spec.applet.Name(), goDelegateMethodName, method, cfg.Middleware); err != nil {
						return err
					}
					publicMethod = makeBunPublicProxyMethod(methodName, goDelegateMethodName, method)
				}
				if err := state.rpcRegistry.RegisterPublic(spec.applet.Name(), methodName, publicMethod, cfg.Middleware); err != nil {
					return err
				}
			}
		}
		if spec.frontendType == appletsconfig.FrontendTypeSSR {
			state.ssrSpecs = append(state.ssrSpecs, spec)
		}
	}
	return nil
}

func (b *AppletEngineBuilder) validateAllBackends(state *buildState) error {
	for _, spec := range state.specs {
		if !spec.hasEngineConfig {
			continue
		}
		ctx := BuildContext{
			AppletName:    spec.applet.Name(),
			Applet:        spec.applet,
			ProjectRoot:   state.projectRoot,
			ProjectConfig: state.projectConfig,
			EngineConfig:  spec.engineConfig,
			Pool:          state.input.Pool,
			Logger:        state.input.Logger,
		}
		if err := b.backends.KV.Validate(spec.engineConfig.Backends.KV, ctx); err != nil {
			return err
		}
		if err := b.backends.DB.Validate(spec.engineConfig.Backends.DB, ctx); err != nil {
			return err
		}
		if err := b.backends.Jobs.Validate(spec.engineConfig.Backends.Jobs, ctx); err != nil {
			return err
		}
		if err := b.backends.Files.Validate(spec.engineConfig.Backends.Files, ctx); err != nil {
			return err
		}
		if err := b.backends.Secrets.Validate(spec.engineConfig.Backends.Secrets, ctx); err != nil {
			return err
		}
	}
	return nil
}

func (b *AppletEngineBuilder) buildBackends(state *buildState) error {
	if countEngineSpecs(state.specs) == 0 {
		return nil
	}
	state.wsBridge = appletenginewsbridge.New(state.input.Logger)
	for i := range state.specs {
		spec := &state.specs[i]
		if !spec.hasEngineConfig {
			continue
		}
		container := &Container{context: BuildContext{
			AppletName:    spec.applet.Name(),
			Applet:        spec.applet,
			ProjectRoot:   state.projectRoot,
			ProjectConfig: state.projectConfig,
			EngineConfig:  spec.engineConfig,
			Pool:          state.input.Pool,
			Logger:        state.input.Logger,
		}}

		kvStore, err := b.backends.KV.Build(spec.engineConfig.Backends.KV, container)
		if err != nil {
			return fmt.Errorf("configure %s kv store for %s: %w", spec.engineConfig.Backends.KV, spec.applet.Name(), err)
		}
		spec.backends.kv = kvStore

		dbStore, err := b.backends.DB.Build(spec.engineConfig.Backends.DB, container)
		if err != nil {
			return fmt.Errorf("configure %s db store for %s: %w", spec.engineConfig.Backends.DB, spec.applet.Name(), err)
		}
		spec.backends.db = dbStore

		jobsStore, err := b.backends.Jobs.Build(spec.engineConfig.Backends.Jobs, container)
		if err != nil {
			return fmt.Errorf("configure %s jobs store for %s: %w", spec.engineConfig.Backends.Jobs, spec.applet.Name(), err)
		}
		spec.backends.jobs = jobsStore

		filesStore, err := b.backends.Files.Build(spec.engineConfig.Backends.Files, container)
		if err != nil {
			return fmt.Errorf("configure %s files store for %s: %w", spec.engineConfig.Backends.Files, spec.applet.Name(), err)
		}
		spec.backends.files = filesStore
		if filesStore != nil {
			state.fileStoreByApp[spec.applet.Name()] = filesStore
		}

		secretsStore, err := b.backends.Secrets.Build(spec.engineConfig.Backends.Secrets, container)
		if err != nil {
			return fmt.Errorf("configure %s secrets store for %s: %w", spec.engineConfig.Backends.Secrets, spec.applet.Name(), err)
		}
		if err := validateRequiredAppletSecrets(context.Background(), spec.applet.Name(), spec.engineConfig.Secrets.Required, secretsStore); err != nil {
			return err
		}
		spec.backends.secrets = secretsStore
	}
	return nil
}

func (b *AppletEngineBuilder) registerStubs(state *buildState) error {
	if state.wsBridge == nil {
		return nil
	}
	wsStub := appletenginehandlers.NewWSStub(state.wsBridge)
	for _, spec := range state.specs {
		if !spec.hasEngineConfig {
			continue
		}
		kvStub := appletenginehandlers.NewKVStub()
		if spec.backends.kv != nil {
			kvStub = appletenginehandlers.NewKVStubWithStore(spec.backends.kv)
		}
		if err := kvStub.Register(state.rpcRegistry, spec.applet.Name()); err != nil {
			return err
		}

		dbStub := appletenginehandlers.NewDBStub()
		if spec.backends.db != nil {
			dbStub = appletenginehandlers.NewDBStubWithStore(spec.backends.db)
		}
		if err := dbStub.Register(state.rpcRegistry, spec.applet.Name()); err != nil {
			return err
		}

		jobsStub := appletenginehandlers.NewJobsStub()
		if spec.backends.jobs != nil {
			jobsStub = appletenginehandlers.NewJobsStubWithStore(spec.backends.jobs)
		}
		if err := jobsStub.Register(state.rpcRegistry, spec.applet.Name()); err != nil {
			return err
		}

		filesStub := appletenginehandlers.NewFilesStub()
		if spec.backends.files != nil {
			filesStub = appletenginehandlers.NewFilesStubWithStore(spec.backends.files)
		}
		if err := filesStub.Register(state.rpcRegistry, spec.applet.Name()); err != nil {
			return err
		}

		secretsStub := appletenginehandlers.NewSecretsStub()
		if spec.backends.secrets != nil {
			secretsStub = appletenginehandlers.NewSecretsStubWithStore(spec.backends.secrets)
		}
		if err := secretsStub.Register(state.rpcRegistry, spec.applet.Name()); err != nil {
			return err
		}

		if err := wsStub.Register(state.rpcRegistry, spec.applet.Name()); err != nil {
			return err
		}
	}
	return nil
}

func (b *AppletEngineBuilder) buildAppletControllers(state *buildState) error {
	controllers := make([]Controller, 0, len(state.specs)+2)
	for _, spec := range state.specs {
		if spec.frontendType == appletsconfig.FrontendTypeSSR {
			continue
		}
		controller, err := applets.NewAppletController(
			spec.applet,
			state.input.Bundle,
			state.input.SessionConfig,
			state.input.Logger,
			state.input.Metrics,
			state.input.Host,
			state.input.Options...,
		)
		if err != nil {
			return err
		}
		controllers = append(controllers, controller)
	}
	state.controllers = controllers
	return nil
}

func (b *AppletEngineBuilder) maybeBuildBunRuntime(state *buildState) error {
	runtimeEnabled := make(map[string]appletsconfig.AppletEngineConfig)
	for _, spec := range state.specs {
		if spec.hasEngineConfig && appletengineruntime.EnabledForEngineConfig(spec.engineConfig) {
			runtimeEnabled[spec.applet.Name()] = spec.engineConfig
		}
	}
	state.runtimeEnabled = runtimeEnabled
	if len(runtimeEnabled) == 0 {
		return nil
	}
	effectiveBin, err := b.bunBinResolver.Resolve(runtimeEnabled)
	if err != nil {
		return err
	}
	state.effectiveRuntimeBin = effectiveBin
	return nil
}

func (b *AppletEngineBuilder) assembleDispatcher(state *buildState) error {
	if state.rpcRegistry.CountPublic() == 0 {
		return nil
	}
	dispatcher := appletenginerpc.NewDispatcher(state.rpcRegistry, state.input.Host, state.input.Logger)
	state.dispatcher = dispatcher

	if len(state.runtimeEnabled) == 0 {
		return nil
	}
	runtimeManager := appletengineruntime.NewManager("", dispatcher, state.input.Logger)
	runtimeManager.SetBunBin(state.effectiveRuntimeBin)
	dispatcher.SetBunPublicCaller(runtimeManager)
	for _, spec := range state.specs {
		engineCfg, ok := state.runtimeEnabled[spec.applet.Name()]
		if !ok {
			continue
		}
		if err := state.rpcRegistry.SetPublicTargetForApplet(spec.applet.Name(), appletenginerpc.MethodTargetBun); err != nil {
			return fmt.Errorf("set bun rpc target for %s: %w", spec.applet.Name(), err)
		}
		runtimeManager.RegisterApplet(spec.applet.Name(), spec.entrypoint)
		if store, ok := state.fileStoreByApp[spec.applet.Name()]; ok && store != nil {
			runtimeManager.RegisterFileStore(spec.applet.Name(), store)
		}
		if engineCfg.Backends.Jobs == appletsconfig.JobsBackendPostgres {
			state.hasPostgresJobs = true
		}
	}
	dispatcher.SetBeforeDispatch(func(ctx context.Context, appletName string) error {
		if _, ok := state.runtimeEnabled[appletName]; !ok {
			return nil
		}
		_, err := runtimeManager.EnsureStarted(ctx, appletName, "")
		return err
	})
	if state.wsBridge != nil {
		state.wsBridge.SetRuntimeManager(runtimeManager)
	}
	state.runtimeManager = runtimeManager
	return nil
}

func (b *AppletEngineBuilder) buildSSRControllers(state *buildState) error {
	if len(state.ssrSpecs) == 0 {
		return nil
	}
	if state.runtimeManager == nil {
		return fmt.Errorf("ssr applets require bun runtime manager")
	}
	for _, spec := range state.ssrSpecs {
		state.runtimeManager.RegisterApplet(spec.applet.Name(), spec.entrypoint)
		state.controllers = append(state.controllers, appletenginecontrollers.NewSSRController(
			spec.applet,
			state.runtimeManager,
			state.input.Host,
			state.input.Logger,
			spec.entrypoint,
		))
	}
	return nil
}

func (b *AppletEngineBuilder) assembleControllers(state *buildState) error {
	if state.rpcRegistry.CountPublic() == 0 {
		return nil
	}
	state.controllers = append(state.controllers, appletenginecontrollers.NewRPCController(state.dispatcher))
	if state.wsBridge != nil {
		state.controllers = append(state.controllers, appletenginecontrollers.NewWSController(state.wsBridge, state.input.Logger))
	}
	return nil
}

func (b *AppletEngineBuilder) buildRuntimeRegistrations(state *buildState) error {
	if state.runtimeManager == nil {
		return nil
	}
	state.runtimeRegistrations = append(state.runtimeRegistrations, RuntimeRegistration{
		Manager:         state.runtimeManager,
		HasPostgresJobs: state.hasPostgresJobs,
	})
	return nil
}

type appletOverride struct {
	base   applets.Applet
	config applets.Config
}

func (a *appletOverride) Name() string     { return a.base.Name() }
func (a *appletOverride) BasePath() string { return a.base.BasePath() }
func (a *appletOverride) Config() applets.Config {
	return a.config
}

func applyProjectAppletOverrides(base applets.Applet, projectConfig *appletsconfig.ProjectConfig) applets.Applet {
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

type appletSchemaArtifact struct {
	Version int `json:"version"`
	Tables  map[string]struct {
		Required []string `json:"required"`
	} `json:"tables"`
}

func validateAppletSchemaArtifact(appletName, projectRoot string, pool *pgxpool.Pool) error {
	if pool == nil || strings.TrimSpace(appletName) == "" {
		return nil
	}
	artifactPath := resolveAppletSchemaArtifactPath(projectRoot, appletName)
	payload, err := os.ReadFile(artifactPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read schema artifact for %s: %w", appletName, err)
	}

	var artifact appletSchemaArtifact
	if err := json.Unmarshal(payload, &artifact); err != nil {
		return fmt.Errorf("decode schema artifact for %s: %w", appletName, err)
	}
	if len(artifact.Tables) == 0 {
		return nil
	}

	rows, err := pool.Query(context.Background(), `
SELECT tenant_id, table_name, document_id, value
FROM applets.documents
WHERE applet_id = $1
`, appletName)
	if err != nil {
		return fmt.Errorf("query documents for schema validation (%s): %w", appletName, err)
	}
	defer rows.Close()

	violations := make([]string, 0)
	for rows.Next() {
		var tenantID, tableName, documentID string
		var rawValue []byte
		if err := rows.Scan(&tenantID, &tableName, &documentID, &rawValue); err != nil {
			return fmt.Errorf("scan schema validation row (%s): %w", appletName, err)
		}
		tableDef, ok := artifact.Tables[tableName]
		if !ok || len(tableDef.Required) == 0 {
			continue
		}
		var value map[string]any
		if err := json.Unmarshal(rawValue, &value); err != nil {
			violations = append(violations, fmt.Sprintf("tenant=%s table=%s id=%s invalid_json", tenantID, tableName, documentID))
			continue
		}
		for _, field := range tableDef.Required {
			field = strings.TrimSpace(field)
			if field == "" {
				continue
			}
			if _, exists := value[field]; !exists {
				violations = append(violations, fmt.Sprintf("tenant=%s table=%s id=%s missing=%s", tenantID, tableName, documentID, field))
			}
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate schema validation rows (%s): %w", appletName, err)
	}
	if len(violations) == 0 {
		return nil
	}
	if len(violations) > 25 {
		violations = violations[:25]
	}
	return fmt.Errorf("schema validation failed for %s: %s", appletName, strings.Join(violations, "; "))
}

func resolveAppletRuntimeEntrypoint(projectRoot, appletName string) string {
	if strings.TrimSpace(projectRoot) != "" {
		return filepath.Join(projectRoot, "modules", appletName, "runtime", "index.ts")
	}
	_, currentFile, _, ok := goruntime.Caller(0)
	if !ok {
		return filepath.Join("modules", appletName, "runtime", "index.ts")
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(currentFile), "..", "..", ".."))
	return filepath.Join(root, "modules", appletName, "runtime", "index.ts")
}

func resolveAppletSchemaArtifactPath(projectRoot, appletName string) string {
	if strings.TrimSpace(projectRoot) != "" {
		return filepath.Join(projectRoot, "modules", appletName, "runtime", "schema.artifact.json")
	}
	_, currentFile, _, ok := goruntime.Caller(0)
	if !ok {
		return filepath.Join("modules", appletName, "runtime", "schema.artifact.json")
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(currentFile), "..", "..", ".."))
	return filepath.Join(root, "modules", appletName, "runtime", "schema.artifact.json")
}

func countEngineSpecs(specs []appletSpec) int {
	count := 0
	for _, spec := range specs {
		if spec.hasEngineConfig {
			count++
		}
	}
	return count
}

func sortedAppletNames(configs map[string]appletsconfig.AppletEngineConfig) []string {
	names := make([]string, 0, len(configs))
	for name := range configs {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
