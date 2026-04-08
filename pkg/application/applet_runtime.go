package application

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync/atomic"
	"time"

	"github.com/iota-uz/applets"
	appletsconfig "github.com/iota-uz/applets/config"
	appletenginecontrollers "github.com/iota-uz/iota-sdk/pkg/appletengine/controllers"
	appletenginehandlers "github.com/iota-uz/iota-sdk/pkg/appletengine/handlers"
	appletenginejobs "github.com/iota-uz/iota-sdk/pkg/appletengine/jobs"
	appletenginerpc "github.com/iota-uz/iota-sdk/pkg/appletengine/rpc"
	appletengineruntime "github.com/iota-uz/iota-sdk/pkg/appletengine/runtime"
	appletenginewsbridge "github.com/iota-uz/iota-sdk/pkg/appletengine/wsbridge"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sirupsen/logrus"
)

func (app *application) buildAppletControllersAndRuntime(
	host applets.HostServices,
	sessionConfig applets.SessionConfig,
	logger *logrus.Logger,
	metrics applets.MetricsRecorder,
	opts ...applets.BuilderOption,
) ([]Controller, []RuntimeRegistration, error) {
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
			return nil, nil, fmt.Errorf("load applet config from .applets/config.toml: %w", loadConfigErr)
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
						return nil, nil, err
					}
					if err := rpcRegistry.RegisterServerOnly(a.Name(), goDelegateMethodName, method, cfg.Middleware); err != nil {
						return nil, nil, err
					}
					publicMethod = makeBunPublicProxyMethod(methodName, goDelegateMethodName, method)
				}
				if err := rpcRegistry.RegisterPublic(a.Name(), methodName, publicMethod, cfg.Middleware); err != nil {
					return nil, nil, err
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
			return nil, nil, err
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
					return nil, nil, fmt.Errorf("configure redis kv store for %s: %w", appletName, err)
				}
				kvStub = appletenginehandlers.NewKVStubWithStore(redisKVStore)
			}
			if err := kvStub.Register(rpcRegistry, appletName); err != nil {
				return nil, nil, err
			}

			dbStub := appletenginehandlers.NewDBStub()
			if engineCfg.Backends.DB == appletsconfig.DBBackendPostgres {
				if err := validateAppletSchemaArtifact(context.Background(), app.DB(), appletName); err != nil {
					return nil, nil, err
				}
				postgresDBStore, err := appletenginehandlers.NewPostgresDBStore(app.DB())
				if err != nil {
					return nil, nil, fmt.Errorf("configure postgres db store for %s: %w", appletName, err)
				}
				dbStub = appletenginehandlers.NewDBStubWithStore(postgresDBStore)
			}
			if err := dbStub.Register(rpcRegistry, appletName); err != nil {
				return nil, nil, err
			}

			jobsStub := appletenginehandlers.NewJobsStub()
			if engineCfg.Backends.Jobs == appletsconfig.JobsBackendPostgres {
				postgresJobsStore, err := appletenginehandlers.NewPostgresJobsStore(app.DB())
				if err != nil {
					return nil, nil, fmt.Errorf("configure postgres jobs store for %s: %w", appletName, err)
				}
				jobsStub = appletenginehandlers.NewJobsStubWithStore(postgresJobsStore)
			}
			if err := jobsStub.Register(rpcRegistry, appletName); err != nil {
				return nil, nil, err
			}

			filesStore := appletenginehandlers.NewLocalFilesStore(strings.TrimSpace(engineCfg.Files.Dir))
			switch engineCfg.Backends.Files {
			case appletsconfig.FilesBackendPostgres:
				postgresFilesStore, err := appletenginehandlers.NewPostgresFilesStore(
					app.DB(),
					strings.TrimSpace(engineCfg.Files.Dir),
				)
				if err != nil {
					return nil, nil, fmt.Errorf("configure postgres files store for %s: %w", appletName, err)
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
					return nil, nil, fmt.Errorf("configure s3 files store for %s: %w", appletName, err)
				}
				filesStore = s3FilesStore
			}
			fileStoreByApplet[appletName] = filesStore
			filesStub := appletenginehandlers.NewFilesStubWithStore(filesStore)
			if err := filesStub.Register(rpcRegistry, appletName); err != nil {
				return nil, nil, err
			}

			secretsStore := appletenginehandlers.NewEnvSecretsStore()
			if engineCfg.Backends.Secrets == appletsconfig.SecretsBackendPostgres {
				masterKeyPayload, readErr := os.ReadFile(strings.TrimSpace(engineCfg.Secrets.MasterKeyFile))
				if readErr != nil {
					return nil, nil, fmt.Errorf("read %s secrets master key file: %w", appletName, readErr)
				}
				postgresSecretsStore, err := appletenginehandlers.NewPostgresSecretsStore(
					app.DB(),
					strings.TrimSpace(string(masterKeyPayload)),
				)
				if err != nil {
					return nil, nil, fmt.Errorf("configure postgres secrets store for %s: %w", appletName, err)
				}
				secretsStore = postgresSecretsStore
			}
			if err := validateRequiredAppletSecrets(context.Background(), appletName, engineCfg.Secrets.Required, secretsStore); err != nil {
				return nil, nil, err
			}
			secretsStub := appletenginehandlers.NewSecretsStubWithStore(secretsStore)
			if err := secretsStub.Register(rpcRegistry, appletName); err != nil {
				return nil, nil, err
			}

			if err := wsStub.Register(rpcRegistry, appletName); err != nil {
				return nil, nil, err
			}
		}
	}

	var registrations []RuntimeRegistration
	if rpcRegistry.CountPublic() > 0 {
		dispatcher := appletenginerpc.NewDispatcher(rpcRegistry, host, logger)
		var runtimeManager *appletengineruntime.Manager

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
					return nil, nil, fmt.Errorf("runtime bun_bin mismatch across enabled applets")
				}
			}
			runtimeManager.SetBunBin(effectiveBunBin)

			hasPostgresJobs := false
			for appletName, engineCfg := range runtimeEnabledByApplet {
				if err := rpcRegistry.SetPublicTargetForApplet(appletName, appletenginerpc.MethodTargetBun); err != nil {
					return nil, nil, fmt.Errorf("set bun rpc target for %s: %w", appletName, err)
				}
				entrypoint := resolveAppletRuntimeEntrypoint(appletName)
				runtimeManager.RegisterApplet(appletName, entrypoint)
				if store, ok := fileStoreByApplet[appletName]; ok && store != nil {
					runtimeManager.RegisterFileStore(appletName, store)
				}
				if engineCfg.Backends.Jobs == appletsconfig.JobsBackendPostgres {
					hasPostgresJobs = true
				}
			}
			dispatcher.SetBeforeDispatch(func(ctx context.Context, appletName string) error {
				if _, ok := runtimeEnabledByApplet[appletName]; !ok {
					return nil
				}
				_, err := runtimeManager.EnsureStarted(ctx, appletName, "")
				return err
			})
			if wsBridge != nil {
				wsBridge.SetRuntimeManager(runtimeManager)
			}
			app.appletRuntime = runtimeManager
			registrations = append(registrations, RuntimeRegistration{
				Component: newAppletRuntimeComponent(runtimeManager, app.DB(), logger, hasPostgresJobs),
				Tags: []RuntimeTag{
					RuntimeTagWorker,
				},
			})
		}

		if len(ssrApplets) > 0 {
			if runtimeManager == nil {
				return nil, nil, fmt.Errorf("ssr applets require bun runtime manager")
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

	return controllers, registrations, nil
}

type appletRuntimeComponent struct {
	manager         *appletengineruntime.Manager
	pool            *pgxpool.Pool
	logger          *logrus.Logger
	hasPostgresJobs bool
	startedJobs     atomic.Bool
}

func newAppletRuntimeComponent(
	manager *appletengineruntime.Manager,
	pool *pgxpool.Pool,
	logger *logrus.Logger,
	hasPostgresJobs bool,
) RuntimeComponent {
	return &appletRuntimeComponent{
		manager:         manager,
		pool:            pool,
		logger:          logger,
		hasPostgresJobs: hasPostgresJobs,
	}
}

func (c *appletRuntimeComponent) Name() string {
	return "applet-runtime"
}

func (c *appletRuntimeComponent) Start(ctx context.Context) error {
	if c.manager == nil || !c.hasPostgresJobs || c.startedJobs.Load() {
		return nil
	}
	if c.pool == nil {
		return nil
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	runner, err := appletenginejobs.NewRunner(c.pool, c.manager, c.logger, 2*time.Second)
	if err != nil {
		return fmt.Errorf("create applet jobs runner: %w", err)
	}
	if !c.startedJobs.CompareAndSwap(false, true) {
		return nil
	}
	jobCtx, jobCancel := context.WithCancel(context.Background())
	c.manager.SetJobCancel(jobCancel)
	go runner.Start(jobCtx)
	return nil
}

func (c *appletRuntimeComponent) Stop(ctx context.Context) error {
	if c.manager == nil {
		return nil
	}
	return c.manager.Shutdown(ctx)
}
