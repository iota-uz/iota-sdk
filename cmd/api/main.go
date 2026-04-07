package main

import (
	"context"
	"log"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
	"time"

	"github.com/google/uuid"
	langfusego "github.com/henomis/langfuse-go"
	"github.com/iota-uz/applets"
	internalassets "github.com/iota-uz/iota-sdk/internal/assets"
	"github.com/iota-uz/iota-sdk/internal/server"
	"github.com/iota-uz/iota-sdk/modules"
	"github.com/iota-uz/iota-sdk/modules/bichat"
	bichatagents "github.com/iota-uz/iota-sdk/modules/bichat/agents"
	bichatinfra "github.com/iota-uz/iota-sdk/modules/bichat/infrastructure"
	llmproviders "github.com/iota-uz/iota-sdk/modules/bichat/infrastructure/llmproviders"
	bichatpersistence "github.com/iota-uz/iota-sdk/modules/bichat/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/modules/core/presentation/controllers"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/bichat/kb"
	langfuseprovider "github.com/iota-uz/iota-sdk/pkg/bichat/observability/langfuse"
	"github.com/iota-uz/iota-sdk/pkg/bichat/schema"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/configuration"
	"github.com/iota-uz/iota-sdk/pkg/constants"
	"github.com/iota-uz/iota-sdk/pkg/eventbus"
	"github.com/iota-uz/iota-sdk/pkg/logging"
	"github.com/iota-uz/iota-sdk/pkg/types"

	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/lib/pq"
	"golang.org/x/text/language"
)

type noopMetrics struct{}

func (n noopMetrics) RecordDuration(name string, duration time.Duration, labels map[string]string) {}
func (n noopMetrics) IncrementCounter(name string, labels map[string]string)                       {}

type sdkAppletUserAdapter struct{ u user.User }

func (a *sdkAppletUserAdapter) ID() uint { return a.u.ID() }

func (a *sdkAppletUserAdapter) DisplayName() string {
	return strings.TrimSpace(a.u.FirstName() + " " + a.u.LastName())
}

func (a *sdkAppletUserAdapter) HasPermission(name string) bool {
	nameNorm := strings.ToLower(strings.TrimSpace(name))
	if nameNorm == "" {
		return false
	}
	for _, permissionName := range composables.EffectivePermissionNames(a.u) {
		if strings.ToLower(permissionName) == nameNorm {
			return true
		}
	}
	return false
}

func (a *sdkAppletUserAdapter) PermissionNames() []string {
	return composables.EffectivePermissionNames(a.u)
}

type sdkHostServices struct{ pool *pgxpool.Pool }

func (h *sdkHostServices) ExtractUser(ctx context.Context) (applets.AppletUser, error) {
	u, err := composables.UseUser(ctx)
	if err != nil || u == nil {
		return nil, err
	}
	return &sdkAppletUserAdapter{u: u}, nil
}

func (h *sdkHostServices) ExtractTenantID(ctx context.Context) (uuid.UUID, error) {
	return composables.UseTenantID(ctx)
}

func (h *sdkHostServices) ExtractPool(ctx context.Context) (*pgxpool.Pool, error) {
	return h.pool, nil
}

func (h *sdkHostServices) ExtractPageLocale(ctx context.Context) language.Tag {
	pc := ctx.Value(constants.PageContext)
	if pc == nil {
		return language.English
	}
	p, ok := pc.(types.PageContext)
	if !ok {
		return language.English
	}
	return p.GetLocale()
}

func main() {
	defer func() {
		if r := recover(); r != nil {
			configuration.Use().Unload()
			log.Println(r)
			debug.PrintStack()
			os.Exit(1)
		}
	}()

	conf := configuration.Use()
	logger := conf.Logger()

	var tracingCleanup func()
	if conf.OpenTelemetry.IsConfigured() {
		tracingCleanup = logging.SetupTracing(
			context.Background(),
			conf.OpenTelemetry.ServiceName+"-api",
			conf.OpenTelemetry.TempoURL,
		)
		defer tracingCleanup()
		logger.Info("OpenTelemetry tracing enabled, exporting to Tempo at " + conf.OpenTelemetry.TempoURL)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, conf.Database.Opts)
	if err != nil {
		panic(err)
	}
	bundle := application.LoadBundle()
	app, err := application.New(&application.ApplicationOptions{
		Pool:               pool,
		Bundle:             bundle,
		EventBus:           eventbus.NewEventPublisher(logger),
		Logger:             logger,
		SupportedLanguages: application.DefaultSupportedLanguages(),
		Huber: application.NewHub(&application.HuberOptions{
			Pool:           pool,
			Logger:         logger,
			Bundle:         bundle,
			UserRepository: persistence.NewUserRepository(persistence.NewUploadRepository()),
			CheckOrigin: func(r *http.Request) bool {
				return true
			},
		}),
	})
	if err != nil {
		log.Fatalf("failed to initialize application: %v", err)
	}
	defer func() {
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer shutdownCancel()
		if stopErr := app.StopRuntime(shutdownCtx); stopErr != nil {
			logger.WithError(stopErr).Warn("failed to stop runtime during shutdown")
		}
	}()
	if err := application.Wire(app, modules.BuiltInModules...); err != nil {
		log.Fatalf("failed to wire modules: %v", err)
	}
	app.RegisterNavItems(modules.NavLinks...)
	app.RegisterHashFsAssets(internalassets.HashFS)

	var bichatModule application.Module
	if openaiKey := os.Getenv("OPENAI_API_KEY"); openaiKey != "" {
		chatRepo := bichatpersistence.NewPostgresChatRepository()

		model, err := llmproviders.NewOpenAIModel()
		if err != nil {
			logger.Warnf("Failed to create OpenAI model for BiChat: %v", err)
		} else {
			executor := bichatinfra.NewPostgresQueryExecutor(pool)
			learningStore := bichatpersistence.NewLearningRepository(pool)
			validatedQueryStore := bichatpersistence.NewValidatedQueryRepository(pool)

			agentOpts := []bichatagents.BIAgentOption{
				bichatagents.WithLearningStore(learningStore),
				bichatagents.WithValidatedQueryStore(validatedQueryStore),
			}
			if modelName := strings.TrimSpace(model.Info().Name); modelName != "" {
				agentOpts = append(agentOpts, bichatagents.WithModel(modelName))
			}

			configOpts := []bichat.ConfigOption{
				bichat.WithQueryExecutor(executor),
				bichat.WithLearningStore(learningStore),
				bichat.WithValidatedQueryStore(validatedQueryStore),
				bichat.WithAttachmentStorage(
					conf.UploadsPath+"/bichat",
					conf.Origin+"/"+conf.UploadsPath+"/bichat",
				),
			}

			knowledgeDir := strings.TrimSpace(conf.BiChatKnowledgeDir)
			kbIndexPath := strings.TrimSpace(conf.BiChatKBIndexPath)
			if kbIndexPath == "" && knowledgeDir != "" {
				kbIndexPath = filepath.Join(conf.UploadsPath, "bichat", "knowledge.bleve")
			}
			if kbIndexPath != "" {
				if err := os.MkdirAll(filepath.Dir(kbIndexPath), 0o750); err != nil {
					logger.Warnf("Failed to create KB index directory: %v", err)
				} else {
					_, kbSearcher, kbErr := kb.NewBleveIndex(kbIndexPath)
					if kbErr != nil {
						logger.Warnf("Failed to initialize BiChat KB index: %v", kbErr)
					} else {
						agentOpts = append(agentOpts, bichatagents.WithKBSearcher(kbSearcher))
						configOpts = append(configOpts, bichat.WithKBSearcher(kbSearcher))
					}
				}
			}

			metadataDir := strings.TrimSpace(conf.BiChatSchemaMetadataDir)
			if metadataDir == "" && knowledgeDir != "" {
				metadataDir = filepath.Join(knowledgeDir, "tables")
			}
			if metadataDir != "" {
				metadataProvider, providerErr := schema.NewFileMetadataProvider(metadataDir)
				if providerErr != nil {
					logger.Warnf("Failed to initialize schema metadata provider (%s): %v", metadataDir, providerErr)
				} else {
					configOpts = append(configOpts, bichat.WithSchemaMetadata(metadataProvider))
				}
			}

			parentAgent, err := bichatagents.NewDefaultBIAgent(executor, agentOpts...)
			if err != nil {
				logger.Warnf("Failed to create BiChat agent: %v", err)
			} else {
				if lfPublicKey := os.Getenv("LANGFUSE_PUBLIC_KEY"); lfPublicKey != "" {
					if baseURL := os.Getenv("LANGFUSE_BASE_URL"); baseURL != "" && os.Getenv("LANGFUSE_HOST") == "" {
						if err := os.Setenv("LANGFUSE_HOST", baseURL); err != nil {
							logger.Warnf("Failed to set LANGFUSE_HOST for Langfuse SDK: %v", err)
						}
					}

					lfClient := langfusego.New(context.Background())
					lfProvider, lfErr := langfuseprovider.NewLangfuseProvider(lfClient, langfuseprovider.Config{
						Enabled:     true,
						PublicKey:   lfPublicKey,
						SecretKey:   os.Getenv("LANGFUSE_SECRET_KEY"),
						Host:        os.Getenv("LANGFUSE_BASE_URL"),
						Environment: "development",
						SampleRate:  1.0,
					})
					if lfErr != nil {
						logger.Warnf("Failed to create LangFuse provider: %v", lfErr)
					} else {
						configOpts = append(configOpts, bichat.WithObservability(lfProvider))
						logger.Info("LangFuse observability enabled")
					}
				}

				cfg := bichat.NewModuleConfig(
					func(ctx context.Context) uuid.UUID {
						tenantID, err := composables.UseTenantID(ctx)
						if err != nil {
							panic(err)
						}
						return tenantID
					},
					func(ctx context.Context) int64 {
						currentUser, err := composables.UseUser(ctx)
						if err != nil {
							panic(err)
						}
						uid := uint64(currentUser.ID())
						if uid > math.MaxInt64 {
							panic("user id overflows int64")
						}
						return int64(uid)
					},
					chatRepo,
					model,
					bichat.DefaultContextPolicy(),
					parentAgent,
					configOpts...,
				)

				bichatModule = bichat.NewModuleWithConfig(cfg)
				if err := bichatModule.RegisterWiring(app); err != nil {
					logger.Warnf("Failed to register BiChat module: %v", err)
				} else {
					app.RegisterNavItems(bichat.NavItems...)
					logger.Info("BiChat module registered successfully")
				}
			}
		}
	} else {
		logger.Info("OPENAI_API_KEY not set - BiChat module disabled")
	}

	hostServices := &sdkHostServices{pool: pool}
	appletControllers, err := app.CreateAppletControllers(
		hostServices,
		applets.DefaultSessionConfig,
		logger,
		noopMetrics{},
	)
	if err != nil {
		log.Fatalf("failed to create applet controllers: %v", err)
	}
	if err := app.RegisterAppletRuntime(
		hostServices,
		applets.DefaultSessionConfig,
		logger,
		noopMetrics{},
	); err != nil {
		log.Fatalf("failed to register applet runtime: %v", err)
	}
	if err := application.RegisterTransports(app, modules.BuiltInModules...); err != nil {
		log.Fatalf("failed to register module transports: %v", err)
	}
	if bichatModule != nil {
		if err := bichatModule.RegisterTransports(app); err != nil {
			log.Fatalf("failed to register bichat transports: %v", err)
		}
	}

	app.RegisterControllers(
		controllers.NewStaticFilesController(app.HashFsAssets()),
		controllers.NewGraphQLController(app),
	)
	app.RegisterControllers(appletControllers...)
	if err := app.StartRuntime(context.Background(), application.CompositionProfileAPIOnly); err != nil {
		log.Fatalf("failed to start runtime: %v", err)
	}

	options := &server.DefaultOptions{
		Logger:        logger,
		Configuration: conf,
		Application:   app,
		Pool:          pool,
	}
	serverInstance, err := server.Default(options)
	if err != nil {
		log.Fatalf("failed to create server: %v", err)
	}
	log.Printf("Listening on: %s\n", conf.Origin)
	if err := serverInstance.Start(conf.SocketAddress); err != nil {
		log.Fatalf("failed to start server: %v", err)
	}
}
