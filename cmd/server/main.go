package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
	"time"

	"github.com/google/uuid"
	langfusego "github.com/henomis/langfuse-go"
	internalassets "github.com/iota-uz/iota-sdk/internal/assets"
	"github.com/iota-uz/iota-sdk/internal/server"
	"github.com/iota-uz/iota-sdk/modules"
	"github.com/iota-uz/iota-sdk/modules/bichat"
	bichatagents "github.com/iota-uz/iota-sdk/modules/bichat/agents"
	bichatinfra "github.com/iota-uz/iota-sdk/modules/bichat/infrastructure"
	llmproviders "github.com/iota-uz/iota-sdk/modules/bichat/infrastructure/llmproviders"
	bichatpersistence "github.com/iota-uz/iota-sdk/modules/bichat/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/modules/core/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/modules/core/presentation/controllers"
	"github.com/iota-uz/applets/pkg/applet"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/bichat/kb"
	langfuseprovider "github.com/iota-uz/iota-sdk/pkg/bichat/observability/langfuse"
	"github.com/iota-uz/iota-sdk/pkg/bichat/schema"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/configuration"
	"github.com/iota-uz/iota-sdk/pkg/eventbus"
	"github.com/iota-uz/iota-sdk/pkg/logging"

	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/lib/pq"
)

// noopMetrics is a no-op implementation of MetricsRecorder
type noopMetrics struct{}

func (n noopMetrics) RecordDuration(name string, duration time.Duration, labels map[string]string) {}
func (n noopMetrics) IncrementCounter(name string, labels map[string]string)                       {}

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

	// Set up OpenTelemetry if enabled
	var tracingCleanup func()
	if conf.OpenTelemetry.Enabled {
		tracingCleanup = logging.SetupTracing(
			context.Background(),
			conf.OpenTelemetry.ServiceName,
			conf.OpenTelemetry.TempoURL,
		)
		defer tracingCleanup()
		logger.Info("OpenTelemetry tracing enabled, exporting to Tempo at " + conf.OpenTelemetry.TempoURL)
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()
	pool, err := pgxpool.New(ctx, conf.Database.Opts)
	if err != nil {
		panic(err)
	}
	bundle := application.LoadBundle()
	app := application.New(&application.ApplicationOptions{
		Pool:     pool,
		Bundle:   bundle,
		EventBus: eventbus.NewEventPublisher(logger),
		Logger:   logger,
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
	if err := modules.Load(app, modules.BuiltInModules...); err != nil {
		log.Fatalf("failed to load modules: %v", err)
	}
	app.RegisterNavItems(modules.NavLinks...)
	app.RegisterHashFsAssets(internalassets.HashFS)

	// Register BiChat module with config (requires OpenAI API key)
	if openaiKey := os.Getenv("OPENAI_API_KEY"); openaiKey != "" {
		// Create BiChat dependencies
		chatRepo := bichatpersistence.NewPostgresChatRepository()

		model, err := llmproviders.NewOpenAIModel()
		if err != nil {
			logger.Warnf("Failed to create OpenAI model for BiChat: %v", err)
		} else {
			// Create PostgreSQL query executor for SQL tools
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
				if err := os.MkdirAll(filepath.Dir(kbIndexPath), 0755); err != nil {
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

			// Create BiChat agent with SQL query capabilities
			parentAgent, err := bichatagents.NewDefaultBIAgent(executor, agentOpts...)
			if err != nil {
				logger.Warnf("Failed to create BiChat agent: %v", err)
			} else {
				// Set up LangFuse observability if credentials are available
				if lfPublicKey := os.Getenv("LANGFUSE_PUBLIC_KEY"); lfPublicKey != "" {
					// Bridge LANGFUSE_BASE_URL → LANGFUSE_HOST for the langfuse-go SDK
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

				// Create BiChat config with wrapper functions for tenant/user ID
				cfg := bichat.NewModuleConfig(
					func(ctx context.Context) uuid.UUID {
						tenantID, err := composables.UseTenantID(ctx)
						if err != nil {
							panic(err) // Fail fast if tenant context missing
						}
						return tenantID
					},
					func(ctx context.Context) int64 {
						user, err := composables.UseUser(ctx)
						if err != nil {
							panic(err) // Fail fast if user context missing
						}
						return int64(user.ID())
					},
					chatRepo,
					model,
					bichat.DefaultContextPolicy(),
					parentAgent,
					configOpts...,
				)

				// Register BiChat module with config
				bichatModule := bichat.NewModuleWithConfig(cfg)
				if err := bichatModule.Register(app); err != nil {
					logger.Warnf("Failed to register BiChat module: %v", err)
				} else {
					// Register BiChat navigation items (only when module is loaded)
					app.RegisterNavItems(bichat.NavItems...)
					logger.Info("BiChat module registered successfully")
				}
			}
		}
	} else {
		logger.Info("OPENAI_API_KEY not set - BiChat module disabled")
	}

	// Register applet controllers for all registered applets
	appletControllers, err := app.CreateAppletControllers(
		applet.DefaultSessionConfig,
		logger,
		noopMetrics{},
	)
	if err != nil {
		log.Fatalf("failed to create applet controllers: %v", err)
	}

	app.RegisterControllers(
		controllers.NewStaticFilesController(app.HashFsAssets()),
		controllers.NewGraphQLController(app),
	)
	app.RegisterControllers(appletControllers...)
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
