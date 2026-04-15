package bichat

import (
	"context"
	"math"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
	langfusego "github.com/henomis/langfuse-go"
	bichatagents "github.com/iota-uz/iota-sdk/modules/bichat/agents"
	llmproviders "github.com/iota-uz/iota-sdk/modules/bichat/infrastructure/llmproviders"
	bichatpersistence "github.com/iota-uz/iota-sdk/modules/bichat/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/pkg/bichat/kb"
	"github.com/iota-uz/iota-sdk/pkg/bichat/observability"
	langfuseprovider "github.com/iota-uz/iota-sdk/pkg/bichat/observability/langfuse"
	"github.com/iota-uz/iota-sdk/pkg/bichat/schema"
	bichatsql "github.com/iota-uz/iota-sdk/pkg/bichat/sql"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/composition"
	"github.com/iota-uz/iota-sdk/pkg/configuration"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	openAIAPIKeyEnv      = "OPENAI_API_KEY"
	langfusePublicKeyEnv = "LANGFUSE_PUBLIC_KEY"
	langfuseSecretKeyEnv = "LANGFUSE_SECRET_KEY"
	langfuseBaseURLEnv   = "LANGFUSE_BASE_URL"
	langfuseEnvironment  = "development"
)

// loadModule builds the BiChat runtime graph (module config, service
// container, event bridge). Callers must check OPENAI_API_KEY before
// invoking this — see component.Build. A nil moduleConfig indicates a
// soft failure inside buildModuleConfig (e.g. OpenAI model creation or
// parent agent bootstrap failed); the error is nil in that case.
//
// extraAgentOpts are threaded into NewDefaultBIAgent BEFORE the parent
// agent is built, so downstream KB searchers / model overrides / custom
// agent registries actually influence the running agent. extraConfigOpts
// are appended to the module-level ConfigOption slice after the agent is
// already built — use them for attachment storage, observability, etc.
// The split exists because config options cannot retroactively configure
// an agent that has already been constructed.
func loadModule(
	ctx composition.BuildContext,
	extraAgentOpts []bichatagents.BIAgentOption,
	extraConfigOpts []ConfigOption,
) (*ModuleConfig, *ServiceContainer, *observability.EventBridge, error) {
	const op serrors.Op = "bichat.loadModule"

	pool := ctx.DB()
	if pool == nil {
		return nil, nil, nil, serrors.E(op, "database pool is required")
	}

	appConfig := ctx.Config()
	if appConfig == nil {
		appConfig = configuration.Use()
	}

	moduleConfig, eventBridge, err := buildModuleConfig(pool, appConfig, extraAgentOpts, extraConfigOpts)
	if err != nil {
		return nil, nil, nil, serrors.E(op, err)
	}
	if moduleConfig == nil {
		return nil, nil, nil, nil
	}

	servicesContainer, err := moduleConfig.BuildServices()
	if err != nil {
		return nil, nil, nil, serrors.E(op, err, "build services")
	}
	return moduleConfig, servicesContainer, eventBridge, nil
}

func buildModuleConfig(
	pool *pgxpool.Pool,
	appConfig *configuration.Configuration,
	extraAgentOpts []bichatagents.BIAgentOption,
	extraConfigOpts []ConfigOption,
) (*ModuleConfig, *observability.EventBridge, error) {
	const op serrors.Op = "bichat.buildModuleConfig"

	if appConfig == nil {
		return nil, nil, serrors.E(op, "configuration is required")
	}
	if pool == nil {
		return nil, nil, serrors.E(op, "database pool is required")
	}

	model, err := llmproviders.NewOpenAIModel()
	if err != nil {
		appConfig.Logger().Warnf("Failed to create OpenAI model for BiChat: %v", err)
		return nil, nil, nil
	}

	chatRepo := bichatpersistence.NewPostgresChatRepository()
	// SafeQueryExecutor ships with AllowAllPolicy by default: the bichat
	// SDK module applies no schema-level gating and trusts the Postgres
	// role behind `pool` (plus has_table_privilege filtering in the schema
	// tools) to bound visible objects. Downstream consumers that need
	// domain-aware permission checks wire them via WithQueryPolicy when
	// they build their own executor instance.
	executor := bichatsql.NewSafeQueryExecutor(pool,
		bichatsql.WithTenantResolver(composables.UseTenantID),
	)
	learningStore := bichatpersistence.NewLearningRepository(pool)
	validatedQueryStore := bichatpersistence.NewValidatedQueryRepository(pool)

	agentOpts := []bichatagents.BIAgentOption{
		bichatagents.WithLearningStore(learningStore),
		bichatagents.WithValidatedQueryStore(validatedQueryStore),
	}
	if modelName := strings.TrimSpace(model.Info().Name); modelName != "" {
		agentOpts = append(agentOpts, bichatagents.WithModel(modelName))
	}

	moduleOpts := []ConfigOption{
		WithQueryExecutor(executor),
		WithLearningStore(learningStore),
		WithValidatedQueryStore(validatedQueryStore),
		WithAttachmentStorage(
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
			appConfig.Logger().Warnf("Failed to create KB index directory: %v", err)
		} else {
			_, kbSearcher, kbErr := kb.NewBleveIndex(kbIndexPath)
			if kbErr != nil {
				appConfig.Logger().Warnf("Failed to initialize BiChat KB index: %v", kbErr)
			} else {
				agentOpts = append(agentOpts, bichatagents.WithKBSearcher(kbSearcher))
				moduleOpts = append(moduleOpts, WithKBSearcher(kbSearcher))
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
			appConfig.Logger().Warnf("Failed to initialize schema metadata provider (%s): %v", metadataDir, providerErr)
		} else {
			moduleOpts = append(moduleOpts, WithSchemaMetadata(metadataProvider))
		}
	}

	// Append caller-supplied agent options BEFORE constructing the parent
	// agent — options like WithKBSearcher, WithModel, WithAgentRegistry
	// need to be visible to NewDefaultBIAgent. Previously the extra
	// options were only merged into moduleOpts AFTER the agent was built,
	// which meant downstream agent-level overrides silently did nothing.
	agentOpts = append(agentOpts, extraAgentOpts...)
	parentAgent, err := bichatagents.NewDefaultBIAgent(executor, agentOpts...)
	if err != nil {
		appConfig.Logger().Warnf("Failed to create BiChat agent: %v", err)
		return nil, nil, nil
	}

	var providers []observability.Provider
	if publicKey := strings.TrimSpace(os.Getenv(langfusePublicKeyEnv)); publicKey != "" {
		lfClient := langfusego.New(context.Background())
		lfProvider, lfErr := langfuseprovider.NewLangfuseProvider(lfClient, langfuseprovider.Config{
			Enabled:     true,
			PublicKey:   publicKey,
			SecretKey:   os.Getenv(langfuseSecretKeyEnv),
			Host:        os.Getenv(langfuseBaseURLEnv),
			Environment: langfuseEnvironment,
			SampleRate:  1.0,
		})
		if lfErr != nil {
			appConfig.Logger().Warnf("Failed to create LangFuse provider: %v", lfErr)
		} else {
			providers = append(providers, lfProvider)
			moduleOpts = append(moduleOpts, WithObservability(lfProvider))
			appConfig.Logger().Info("LangFuse observability enabled")
		}
	}

	// Append caller-supplied ConfigOptions last so they win over defaults.
	// These are module-level knobs (attachment storage, observability,
	// prompt extensions) that apply on top of the already-constructed
	// parent agent.
	moduleOpts = append(moduleOpts, extraConfigOpts...)
	moduleConfig := NewModuleConfig(
		func(ctx context.Context) uuid.UUID {
			tenantID, err := composables.UseTenantID(ctx)
			if err != nil {
				appConfig.Logger().WithError(err).Warn("BiChat tenant resolver could not read tenant from context")
				return uuid.Nil
			}
			return tenantID
		},
		func(ctx context.Context) int64 {
			currentUser, err := composables.UseUser(ctx)
			if err != nil {
				appConfig.Logger().WithError(err).Warn("BiChat user resolver could not read user from context")
				return 0
			}
			userID := uint64(currentUser.ID())
			if userID > math.MaxInt64 {
				appConfig.Logger().Warn("BiChat user resolver detected user id overflow")
				return 0
			}
			return int64(userID)
		},
		chatRepo,
		model,
		DefaultContextPolicy(),
		parentAgent,
		moduleOpts...,
	)

	var eventBridge *observability.EventBridge
	if len(providers) > 0 {
		eventBridge = observability.NewEventBridge(moduleConfig.EventBus, providers)
	}

	return moduleConfig, eventBridge, nil
}
