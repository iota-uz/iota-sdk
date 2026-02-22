package bichat

import (
	"io/fs"
	"strings"

	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/permission"
	coreservices "github.com/iota-uz/iota-sdk/modules/core/services"
	"github.com/iota-uz/iota-sdk/pkg/analytics"
	"github.com/iota-uz/iota-sdk/pkg/bichat/agents"
	bichatcontext "github.com/iota-uz/iota-sdk/pkg/bichat/context"
	"github.com/iota-uz/iota-sdk/pkg/bichat/hooks"
	"github.com/iota-uz/iota-sdk/pkg/bichat/kb"
	"github.com/iota-uz/iota-sdk/pkg/bichat/learning"
	"github.com/iota-uz/iota-sdk/pkg/bichat/observability"
	"github.com/iota-uz/iota-sdk/pkg/bichat/prompts"
	"github.com/iota-uz/iota-sdk/pkg/bichat/schema"
	bichatsql "github.com/iota-uz/iota-sdk/pkg/bichat/sql"
	"github.com/sirupsen/logrus"
)

// WithModelRegistry sets the model registry for multi-model support
func WithModelRegistry(registry ModelRegistry) ConfigOption {
	return func(c *ModuleConfig) {
		c.ModelRegistry = registry
	}
}

// WithQueryExecutor sets the SQL query executor service
func WithQueryExecutor(executor bichatsql.QueryExecutor) ConfigOption {
	return func(c *ModuleConfig) {
		c.QueryExecutor = executor
	}
}

// WithKBSearcher sets the knowledge base searcher
func WithKBSearcher(searcher kb.KBSearcher) ConfigOption {
	return func(c *ModuleConfig) {
		c.KBSearcher = searcher
	}
}

// WithTenantService sets the tenant service for tenant name lookups
func WithTenantService(svc *coreservices.TenantService) ConfigOption {
	return func(c *ModuleConfig) {
		c.TenantService = svc
	}
}

// WithSchemaMetadata sets the schema metadata provider for table documentation.
// When configured, table metadata (descriptions, use cases, metrics) will be
// injected into agent context as KindReference blocks.
//
// Example:
//
//	provider, _ := schema.NewFileMetadataProvider("/var/lib/bichat/metadata")
//	cfg := bichat.NewModuleConfig(..., bichat.WithSchemaMetadata(provider))
func WithSchemaMetadata(provider schema.MetadataProvider) ConfigOption {
	return func(c *ModuleConfig) {
		c.SchemaMetadataProvider = provider
	}
}

// WithLearningStore sets the learning store for dynamic agent learnings.
// When configured, the agent can save and retrieve learnings from SQL errors,
// type mismatches, and user corrections to avoid repeating mistakes.
//
// Example:
//
//	learningStore := persistence.NewLearningRepository(dbPool)
//	cfg := bichat.NewModuleConfig(..., bichat.WithLearningStore(learningStore))
func WithLearningStore(store learning.LearningStore) ConfigOption {
	return func(c *ModuleConfig) {
		c.LearningStore = store
	}
}

// WithValidatedQueryStore sets the validated query store for query pattern library.
// When configured, the agent can search and save validated SQL query patterns
// to reuse proven solutions for similar questions.
//
// Example:
//
//	validatedQueryStore := persistence.NewValidatedQueryRepository(dbPool)
//	cfg := bichat.NewModuleConfig(..., bichat.WithValidatedQueryStore(validatedQueryStore))
func WithValidatedQueryStore(store learning.ValidatedQueryStore) ConfigOption {
	return func(c *ModuleConfig) {
		c.ValidatedQueryStore = store
	}
}

// WithProjectPromptExtension sets a project-scoped prompt extension that is appended
// to the vendor system prompt in the parent agent execution flow.
func WithProjectPromptExtension(text string) ConfigOption {
	return func(c *ModuleConfig) {
		c.ProjectPromptExtension = text
	}
}

// WithProjectPromptExtensionProvider sets a provider for loading project-scoped prompt extension text.
// Provider output takes precedence over WithProjectPromptExtension when non-empty.
func WithProjectPromptExtensionProvider(provider prompts.ProjectPromptExtensionProvider) ConfigOption {
	return func(c *ModuleConfig) {
		c.ProjectPromptExtensionProvider = provider
	}
}

// WithSkillsDir sets the root directory containing SKILL.md files.
func WithSkillsDir(dir string) ConfigOption {
	return func(c *ModuleConfig) {
		c.SkillsDir = strings.TrimSpace(dir)
	}
}

// WithSkillsSelectionLimit sets the maximum number of skill metadata entries
// included in the per-turn skills catalog reference.
func WithSkillsSelectionLimit(limit int) ConfigOption {
	return func(c *ModuleConfig) {
		c.SkillsSelectionLimit = limit
	}
}

// WithSkillsMaxChars sets the maximum character budget for rendered skills catalog
// reference and load_skill tool output.
func WithSkillsMaxChars(maxChars int) ConfigOption {
	return func(c *ModuleConfig) {
		c.SkillsMaxChars = maxChars
	}
}

// WithLogger sets the logger for observability
func WithLogger(logger *logrus.Logger) ConfigOption {
	return func(c *ModuleConfig) {
		c.Logger = logger
	}
}

// WithEventBus sets the event bus for observability
func WithEventBus(bus hooks.EventBus) ConfigOption {
	return func(c *ModuleConfig) {
		c.EventBus = bus
	}
}

// WithCheckpointer sets the checkpointer for HITL state persistence
func WithCheckpointer(checkpointer Checkpointer) ConfigOption {
	return func(c *ModuleConfig) {
		c.Checkpointer = checkpointer
	}
}

// WithSubAgents sets additional sub-agents for delegation
func WithSubAgents(subAgents ...Agent) ConfigOption {
	return func(c *ModuleConfig) {
		c.SubAgents = append(c.SubAgents, subAgents...)
	}
}

// WithSubAgentDefinitionsSource overrides markdown definitions source used to
// build built-in multi-agent sub-agents.
func WithSubAgentDefinitionsSource(source fs.FS, basePath string) ConfigOption {
	return func(c *ModuleConfig) {
		c.SubAgentDefinitionsFS = source
		c.SubAgentDefinitionsBasePath = strings.TrimSpace(basePath)
	}
}

// WithFeatureProfile applies a preset capability profile.
func WithFeatureProfile(profile FeatureProfile) ConfigOption {
	return func(c *ModuleConfig) {
		c.Profile = profile
	}
}

// WithCapabilities sets explicit capabilities.
func WithCapabilities(capabilities Capabilities) ConfigOption {
	return func(c *ModuleConfig) {
		c.Capabilities = capabilities
		c.capabilitiesConfigured = true
	}
}

// WithCodeInterpreterMemoryLimit sets code interpreter container memory for providers that support it.
// Allowed values: "1g", "4g", "16g", "64g".
func WithCodeInterpreterMemoryLimit(limit string) ConfigOption {
	return func(c *ModuleConfig) {
		c.CodeInterpreterMemoryLimit = strings.ToLower(strings.TrimSpace(limit))
	}
}

// WithAttachmentStorageMode sets attachment storage mode.
func WithAttachmentStorageMode(mode AttachmentStorageMode) ConfigOption {
	return func(c *ModuleConfig) {
		c.AttachmentStorageMode = mode
	}
}

// WithTokenEstimator sets the token estimator for cost tracking and budget management.
// If not provided, a no-op estimator will be used.
//
// Example:
//
//	estimator := agents.NewTiktokenEstimator("cl100k_base")
//	cfg := bichat.NewModuleConfig(..., bichat.WithTokenEstimator(estimator))
func WithTokenEstimator(estimator agents.TokenEstimator) ConfigOption {
	return func(c *ModuleConfig) {
		c.TokenEstimator = estimator
	}
}

// WithObservability adds an observability provider for tracing, metrics, and cost tracking.
// Providers are wrapped in AsyncHandler to prevent blocking the main execution path.
// Multiple providers can be registered by calling this option multiple times.
func WithObservability(provider observability.Provider) ConfigOption {
	return func(c *ModuleConfig) {
		c.ObservabilityProviders = append(c.ObservabilityProviders, provider)
	}
}

// WithRenderer sets a custom renderer for context block formatting.
// If not provided, defaults to AnthropicRenderer.
func WithRenderer(renderer bichatcontext.Renderer) ConfigOption {
	return func(c *ModuleConfig) {
		c.Renderer = renderer
	}
}

// WithAttachmentStorage configures file storage for attachments.
// Both basePath and baseURL are required for attachment support.
//
// Example:
//
//	bichat.WithAttachmentStorage("/var/lib/bichat/attachments", "https://example.com/bichat/attachments")
func WithAttachmentStorage(basePath, baseURL string) ConfigOption {
	return func(c *ModuleConfig) {
		c.AttachmentStorageBasePath = basePath
		c.AttachmentStorageBaseURL = baseURL
	}
}

// WithTitleQueueRedis enables Redis-backed durable title generation queue with default settings.
func WithTitleQueueRedis(redisURL string) ConfigOption {
	return func(c *ModuleConfig) {
		c.TitleQueue = DefaultTitleQueueConfig(redisURL)
	}
}

// WithTitleQueue sets a custom title queue configuration.
// Use DefaultTitleQueueConfig(url) as a starting point and override fields as needed.
func WithTitleQueue(tq *TitleQueueConfig) ConfigOption {
	return func(c *ModuleConfig) {
		c.TitleQueue = tq
	}
}

// WithAnalyticsViews sets the ViewManager for analytics view sync and access control.
// The ViewManager defines view SQL + permissions in Go and syncs them to the database
// on every app start via CREATE OR REPLACE VIEW.
//
// Example:
//
//	vm := analytics.NewViewManager()
//	vm.Register(analytics.DefaultTenantViews()...)
//	vm.Register(analytics.CustomView("custom_report", sql, analytics.RequireAny(perm)))
//	cfg := bichat.NewModuleConfig(..., bichat.WithAnalyticsViews(vm))
func WithAnalyticsViews(vm *analytics.ViewManager) ConfigOption {
	return func(c *ModuleConfig) {
		c.ViewManager = vm
	}
}

// WithStreamRequireAccessPermission overrides the permission required to access StreamController.
func WithStreamRequireAccessPermission(p permission.Permission) ConfigOption {
	return func(c *ModuleConfig) {
		c.StreamRequireAccessPermission = p
	}
}

// WithStreamReadAllPermission overrides the permission required to read other users' sessions via StreamController.
func WithStreamReadAllPermission(p permission.Permission) ConfigOption {
	return func(c *ModuleConfig) {
		c.StreamReadAllPermission = p
	}
}
