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
	"github.com/iota-uz/iota-sdk/pkg/config"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/bichatconfig"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/httpconfig"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/uploadsconfig"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sirupsen/logrus"
)

const langfuseEnvironment = "development"

// resolveBichatConfigs extracts all typed configs needed by buildModuleConfig
// from the BuildContext. When a config.Source is attached, the registry path is
// used (koanf unmarshal + optional Validate). Otherwise the legacy
// *configuration.Configuration is used via each package's FromLegacy shim.
func resolveBichatConfigs(buildCtx composition.BuildContext) (
	bichatCfg *bichatconfig.Config,
	httpCfg *httpconfig.Config,
	uploadsCfg *uploadsconfig.Config,
	logger *logrus.Logger,
) {
	logger = buildCtx.Logger()

	if src := buildCtx.Source(); src != nil {
		reg := config.NewRegistry(src)

		if ptr, err := config.Register[bichatconfig.Config](reg, "bichat"); err == nil {
			bichatCfg = ptr
		}
		if ptr, err := config.Register[httpconfig.Config](reg, "http"); err == nil {
			httpCfg = ptr
		}
		if ptr, err := config.Register[uploadsconfig.Config](reg, "uploads"); err == nil {
			uploadsCfg = ptr
		}
	} else if cfg := buildCtx.Config(); cfg != nil {
		bv := bichatconfig.FromLegacy(cfg)
		bichatCfg = &bv
		hv := httpconfig.FromLegacy(cfg)
		httpCfg = &hv
		uv := uploadsconfig.FromLegacy(cfg)
		uploadsCfg = &uv
	}

	// Ensure zero-value defaults when resolution produced nothing.
	if bichatCfg == nil {
		v := bichatconfig.Config{}
		v.SetDefaults()
		bichatCfg = &v
	}
	if httpCfg == nil {
		httpCfg = &httpconfig.Config{}
	}
	if uploadsCfg == nil {
		v := uploadsconfig.Config{}
		v.SetDefaults()
		uploadsCfg = &v
	}

	return bichatCfg, httpCfg, uploadsCfg, logger
}

// loadModule builds the BiChat runtime graph (module config, service
// container, event bridge). Callers must check bichatCfg.OpenAI.IsConfigured()
// before invoking this — see component.Build. A nil moduleConfig indicates a
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

	moduleConfig, eventBridge, err := buildModuleConfig(ctx, pool, extraAgentOpts, extraConfigOpts)
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
	buildCtx composition.BuildContext,
	pool *pgxpool.Pool,
	extraAgentOpts []bichatagents.BIAgentOption,
	extraConfigOpts []ConfigOption,
) (*ModuleConfig, *observability.EventBridge, error) {
	const op serrors.Op = "bichat.buildModuleConfig"

	if pool == nil {
		return nil, nil, serrors.E(op, "database pool is required")
	}

	bichatCfg, httpCfg, uploadsCfg, logger := resolveBichatConfigs(buildCtx)

	model, err := llmproviders.NewOpenAIModelFromConfig(bichatCfg.OpenAI)
	if err != nil {
		if logger != nil {
			logger.Warnf("Failed to create OpenAI model for BiChat: %v", err)
		}
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

	uploadsPath := uploadsCfg.Path
	origin := httpCfg.Origin

	moduleOpts := []ConfigOption{
		WithQueryExecutor(executor),
		WithLearningStore(learningStore),
		WithValidatedQueryStore(validatedQueryStore),
		WithAttachmentStorage(
			uploadsPath+"/bichat",
			origin+"/"+uploadsPath+"/bichat",
		),
		withAppletSettings(
			httpCfg.IsDev(),
			bichatCfg.Applet.ViteURL,
			bichatCfg.Applet.Entry,
			bichatCfg.Applet.Client,
			bichatCfg.OpenAI.IsConfigured(),
		),
	}

	knowledgeDir := strings.TrimSpace(bichatCfg.Knowledge.Dir)
	kbIndexPath := strings.TrimSpace(bichatCfg.Knowledge.KBIndexPath)
	if kbIndexPath == "" && knowledgeDir != "" {
		kbIndexPath = filepath.Join(uploadsPath, "bichat", "knowledge.bleve")
	}
	if kbIndexPath != "" {
		if err := os.MkdirAll(filepath.Dir(kbIndexPath), 0o750); err != nil {
			if logger != nil {
				logger.Warnf("Failed to create KB index directory: %v", err)
			}
		} else {
			_, kbSearcher, kbErr := kb.NewBleveIndex(kbIndexPath)
			if kbErr != nil {
				if logger != nil {
					logger.Warnf("Failed to initialize BiChat KB index: %v", kbErr)
				}
			} else {
				agentOpts = append(agentOpts, bichatagents.WithKBSearcher(kbSearcher))
				moduleOpts = append(moduleOpts, WithKBSearcher(kbSearcher))
			}
		}
	}

	metadataDir := strings.TrimSpace(bichatCfg.Knowledge.SchemaMetadata)
	if metadataDir == "" && knowledgeDir != "" {
		metadataDir = filepath.Join(knowledgeDir, "tables")
	}
	if metadataDir != "" {
		metadataProvider, providerErr := schema.NewFileMetadataProvider(metadataDir)
		if providerErr != nil {
			if logger != nil {
				logger.Warnf("Failed to initialize schema metadata provider (%s): %v", metadataDir, providerErr)
			}
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
		if logger != nil {
			logger.Warnf("Failed to create BiChat agent: %v", err)
		}
		return nil, nil, nil
	}

	var providers []observability.Provider
	if bichatCfg.Langfuse.IsConfigured() {
		lfClient := langfusego.New(context.Background())
		lfProvider, lfErr := langfuseprovider.NewLangfuseProvider(lfClient, langfuseprovider.Config{
			Enabled:     true,
			PublicKey:   bichatCfg.Langfuse.PublicKey,
			SecretKey:   bichatCfg.Langfuse.SecretKey,
			Host:        bichatCfg.Langfuse.Host,
			Environment: langfuseEnvironment,
			SampleRate:  1.0,
		})
		if lfErr != nil {
			if logger != nil {
				logger.Warnf("Failed to create LangFuse provider: %v", lfErr)
			}
		} else {
			providers = append(providers, lfProvider)
			moduleOpts = append(moduleOpts, WithObservability(lfProvider))
			// Store the Langfuse base URL so debug traces can include a trace link.
			lfURL := bichatCfg.Langfuse.BaseURL
			if lfURL == "" {
				lfURL = bichatCfg.Langfuse.Host
			}
			if lfURL != "" {
				moduleOpts = append(moduleOpts, WithLangfuseBaseURL(lfURL))
			}
			if logger != nil {
				logger.Info("LangFuse observability enabled")
			}
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
				if logger != nil {
					logger.WithError(err).Warn("BiChat tenant resolver could not read tenant from context")
				}
				return uuid.Nil
			}
			return tenantID
		},
		func(ctx context.Context) int64 {
			currentUser, err := composables.UseUser(ctx)
			if err != nil {
				if logger != nil {
					logger.WithError(err).Warn("BiChat user resolver could not read user from context")
				}
				return 0
			}
			userID := uint64(currentUser.ID())
			if userID > math.MaxInt64 {
				if logger != nil {
					logger.Warn("BiChat user resolver detected user id overflow")
				}
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
