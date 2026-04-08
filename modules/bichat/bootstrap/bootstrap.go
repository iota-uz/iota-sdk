package bootstrap

import (
	"context"
	"math"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
	langfusego "github.com/henomis/langfuse-go"
	langfuseprovider "github.com/iota-uz/iota-sdk/pkg/bichat/observability/langfuse"
	sdkbootstrap "github.com/iota-uz/iota-sdk/pkg/bootstrap"
	"github.com/iota-uz/iota-sdk/pkg/serrors"

	"github.com/iota-uz/iota-sdk/modules/bichat"
	bichatagents "github.com/iota-uz/iota-sdk/modules/bichat/agents"
	bichatinfra "github.com/iota-uz/iota-sdk/modules/bichat/infrastructure"
	llmproviders "github.com/iota-uz/iota-sdk/modules/bichat/infrastructure/llmproviders"
	bichatpersistence "github.com/iota-uz/iota-sdk/modules/bichat/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/pkg/bichat/kb"
	"github.com/iota-uz/iota-sdk/pkg/bichat/schema"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/configuration"
)

type Option func(*options)

type options struct {
	registerTransports bool
	openAIKeyEnv       string
	langfusePublicEnv  string
	langfuseSecretEnv  string
	langfuseBaseURLEnv string
	langfuseEnv        string
}

func WithTransports() Option {
	return func(o *options) {
		o.registerTransports = true
	}
}

func New(opts ...Option) sdkbootstrap.Installer {
	cfg := options{
		openAIKeyEnv:       "OPENAI_API_KEY",
		langfusePublicEnv:  "LANGFUSE_PUBLIC_KEY",
		langfuseSecretEnv:  "LANGFUSE_SECRET_KEY",
		langfuseBaseURLEnv: "LANGFUSE_BASE_URL",
		langfuseEnv:        "development",
	}
	for _, opt := range opts {
		if opt != nil {
			opt(&cfg)
		}
	}

	return sdkbootstrap.InstallerFunc(func(ctx context.Context, rt *sdkbootstrap.Runtime) error {
		const op serrors.Op = "bichatbootstrap.New"

		appConfig, ok := rt.Config.(*configuration.Configuration)
		if !ok || appConfig == nil {
			return serrors.E(op, "runtime config must be *configuration.Configuration")
		}

		openAIKey := strings.TrimSpace(os.Getenv(cfg.openAIKeyEnv))
		if openAIKey == "" {
			rt.Logger.Info("OPENAI_API_KEY not set - BiChat module disabled")
			return nil
		}

		chatRepo := bichatpersistence.NewPostgresChatRepository()
		model, err := llmproviders.NewOpenAIModel()
		if err != nil {
			rt.Logger.Warnf("Failed to create OpenAI model for BiChat: %v", err)
			return nil
		}

		executor := bichatinfra.NewPostgresQueryExecutor(rt.Pool)
		learningStore := bichatpersistence.NewLearningRepository(rt.Pool)
		validatedQueryStore := bichatpersistence.NewValidatedQueryRepository(rt.Pool)

		agentOpts := []bichatagents.BIAgentOption{
			bichatagents.WithLearningStore(learningStore),
			bichatagents.WithValidatedQueryStore(validatedQueryStore),
		}
		if modelName := strings.TrimSpace(model.Info().Name); modelName != "" {
			agentOpts = append(agentOpts, bichatagents.WithModel(modelName))
		}

		moduleOpts := []bichat.ConfigOption{
			bichat.WithQueryExecutor(executor),
			bichat.WithLearningStore(learningStore),
			bichat.WithValidatedQueryStore(validatedQueryStore),
			bichat.WithAttachmentStorage(
				appConfig.UploadsPath+"/bichat",
				appConfig.Origin+"/"+appConfig.UploadsPath+"/bichat",
			),
		}

		knowledgeDir := strings.TrimSpace(appConfig.BiChatKnowledgeDir)
		kbIndexPath := strings.TrimSpace(appConfig.BiChatKBIndexPath)
		if kbIndexPath == "" && knowledgeDir != "" {
			kbIndexPath = filepath.Join(appConfig.UploadsPath, "bichat", "knowledge.bleve")
		}
		if kbIndexPath != "" {
			if err := os.MkdirAll(filepath.Dir(kbIndexPath), 0o750); err != nil {
				rt.Logger.Warnf("Failed to create KB index directory: %v", err)
			} else {
				_, kbSearcher, kbErr := kb.NewBleveIndex(kbIndexPath)
				if kbErr != nil {
					rt.Logger.Warnf("Failed to initialize BiChat KB index: %v", kbErr)
				} else {
					agentOpts = append(agentOpts, bichatagents.WithKBSearcher(kbSearcher))
					moduleOpts = append(moduleOpts, bichat.WithKBSearcher(kbSearcher))
				}
			}
		}

		metadataDir := strings.TrimSpace(appConfig.BiChatSchemaMetadataDir)
		if metadataDir == "" && knowledgeDir != "" {
			metadataDir = filepath.Join(knowledgeDir, "tables")
		}
		if metadataDir != "" {
			metadataProvider, providerErr := schema.NewFileMetadataProvider(metadataDir)
			if providerErr != nil {
				rt.Logger.Warnf("Failed to initialize schema metadata provider (%s): %v", metadataDir, providerErr)
			} else {
				moduleOpts = append(moduleOpts, bichat.WithSchemaMetadata(metadataProvider))
			}
		}

		parentAgent, err := bichatagents.NewDefaultBIAgent(executor, agentOpts...)
		if err != nil {
			rt.Logger.Warnf("Failed to create BiChat agent: %v", err)
			return nil
		}

		if publicKey := strings.TrimSpace(os.Getenv(cfg.langfusePublicEnv)); publicKey != "" {
			if baseURL := strings.TrimSpace(os.Getenv(cfg.langfuseBaseURLEnv)); baseURL != "" && os.Getenv("LANGFUSE_HOST") == "" {
				if err := os.Setenv("LANGFUSE_HOST", baseURL); err != nil {
					rt.Logger.Warnf("Failed to set LANGFUSE_HOST for Langfuse SDK: %v", err)
				}
			}

			lfClient := langfusego.New(ctx)
			lfProvider, lfErr := langfuseprovider.NewLangfuseProvider(lfClient, langfuseprovider.Config{
				Enabled:     true,
				PublicKey:   publicKey,
				SecretKey:   os.Getenv(cfg.langfuseSecretEnv),
				Host:        os.Getenv(cfg.langfuseBaseURLEnv),
				Environment: cfg.langfuseEnv,
				SampleRate:  1.0,
			})
			if lfErr != nil {
				rt.Logger.Warnf("Failed to create LangFuse provider: %v", lfErr)
			} else {
				moduleOpts = append(moduleOpts, bichat.WithObservability(lfProvider))
				rt.Logger.Info("LangFuse observability enabled")
			}
		}

		moduleConfig := bichat.NewModuleConfig(
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
				userID := uint64(currentUser.ID())
				if userID > math.MaxInt64 {
					panic("user id overflows int64")
				}
				return int64(userID)
			},
			chatRepo,
			model,
			bichat.DefaultContextPolicy(),
			parentAgent,
			moduleOpts...,
		)

		module := bichat.NewModuleWithConfig(moduleConfig)
		if err := module.RegisterWiring(rt.App); err != nil {
			rt.Logger.Warnf("Failed to register BiChat module: %v", err)
			return nil
		}

		rt.App.RegisterNavItems(bichat.NavItems...)
		if cfg.registerTransports {
			if err := module.RegisterTransports(rt.App); err != nil {
				return serrors.E(op, err, "register bichat transports")
			}
		}

		rt.Logger.Info("BiChat module registered successfully")
		rt.Provide(module)
		return nil
	})
}
