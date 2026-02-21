package bichat

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"strings"
	"time"

	"github.com/google/uuid"
	bichatagents "github.com/iota-uz/iota-sdk/modules/bichat/agents"
	"github.com/iota-uz/iota-sdk/modules/bichat/services"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/permission"
	coreservices "github.com/iota-uz/iota-sdk/modules/core/services"
	"github.com/iota-uz/iota-sdk/pkg/analytics"
	"github.com/iota-uz/iota-sdk/pkg/bichat/agents"
	bichatcontext "github.com/iota-uz/iota-sdk/pkg/bichat/context"
	"github.com/iota-uz/iota-sdk/pkg/bichat/context/formatters"
	"github.com/iota-uz/iota-sdk/pkg/bichat/context/renderers"
	"github.com/iota-uz/iota-sdk/pkg/bichat/domain"
	"github.com/iota-uz/iota-sdk/pkg/bichat/hooks"
	"github.com/iota-uz/iota-sdk/pkg/bichat/kb"
	"github.com/iota-uz/iota-sdk/pkg/bichat/learning"
	"github.com/iota-uz/iota-sdk/pkg/bichat/observability"
	"github.com/iota-uz/iota-sdk/pkg/bichat/prompts"
	"github.com/iota-uz/iota-sdk/pkg/bichat/schema"
	bichatservices "github.com/iota-uz/iota-sdk/pkg/bichat/services"
	bichatskills "github.com/iota-uz/iota-sdk/pkg/bichat/skills"
	bichatsql "github.com/iota-uz/iota-sdk/pkg/bichat/sql"
	"github.com/iota-uz/iota-sdk/pkg/bichat/storage"
	bichattools "github.com/iota-uz/iota-sdk/pkg/bichat/tools"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sirupsen/logrus"
)

// Type aliases for convenience
type (
	Agent         = agents.ExtendedAgent
	Model         = agents.Model
	ModelRegistry = *agents.ModelRegistry // Pointer type to avoid copying mutex
	Checkpointer  = agents.Checkpointer
)

type FeatureProfile string

const (
	FeatureProfileMinimal FeatureProfile = "minimal"
	FeatureProfileDataOps FeatureProfile = "data_ops"
	FeatureProfileFull    FeatureProfile = "full"
)

type Capabilities struct {
	Vision          bool
	WebSearch       bool
	CodeInterpreter bool
	MultiAgent      bool
}

func CapabilitiesForProfile(profile FeatureProfile) (Capabilities, bool) {
	switch profile {
	case FeatureProfileMinimal:
		return Capabilities{}, true
	case FeatureProfileDataOps:
		return Capabilities{
			CodeInterpreter: true,
		}, true
	case FeatureProfileFull:
		return Capabilities{
			Vision:          true,
			WebSearch:       true,
			CodeInterpreter: true,
			MultiAgent:      true,
		}, true
	default:
		return Capabilities{}, false
	}
}

type AttachmentStorageMode string

const (
	AttachmentStorageModeLocal AttachmentStorageMode = "local"
	AttachmentStorageModeNoOp  AttachmentStorageMode = "noop"
)

const (
	defaultTitleQueueStream         = "bichat:title:jobs"
	defaultTitleQueueGroup          = "bichat-title-workers"
	defaultTitleQueuePollInterval   = 300 * time.Millisecond
	defaultTitleQueueReadBlock      = 2 * time.Second
	defaultTitleQueueBatchSize      = 16
	defaultTitleQueueMaxRetries     = 3
	defaultTitleQueueRetryBaseDelay = 5 * time.Second
	defaultTitleQueueRetryMaxDelay  = 2 * time.Minute
	defaultTitleQueuePendingIdle    = 30 * time.Second
	defaultTitleQueueReconcileEvery = 1 * time.Minute
	defaultTitleQueueReconcileBatch = 200
	defaultTitleQueueDedupeTTL      = 30 * time.Minute
	defaultTitleQueueJobTimeout     = 20 * time.Second
	defaultSkillsSelectionLimit     = 3
	defaultSkillsMaxChars           = 8000
)

// ModuleConfig holds configuration for the bichat module.
// It uses functional options pattern for optional dependencies.
type ModuleConfig struct {
	// Required: Core dependencies
	TenantID func(ctx context.Context) uuid.UUID
	UserID   func(ctx context.Context) int64
	ChatRepo domain.ChatRepository

	// Required: LLM Model
	Model Model

	// Optional: LLM Model Registry (for multi-model support)
	ModelRegistry ModelRegistry

	// Required: Context management
	ContextPolicy bichatcontext.ContextPolicy

	// Optional: Parent agent (if nil, BuildParentAgent can construct default from QueryExecutor)
	ParentAgent Agent
	SubAgents   []Agent
	// Optional: markdown definitions source for built-in sub-agents.
	SubAgentDefinitionsFS       fs.FS
	SubAgentDefinitionsBasePath string

	// Optional: Project-scoped prompt extension appended to parent agent system prompt.
	ProjectPromptExtension         string
	ProjectPromptExtensionProvider prompts.ProjectPromptExtensionProvider
	SkillsDir                      string
	SkillsSelectionLimit           int
	SkillsMaxChars                 int

	// Optional: Agent Registry for multi-agent orchestration
	AgentRegistry *agents.AgentRegistry

	// Optional services (can be nil)
	QueryExecutor          bichatsql.QueryExecutor
	KBSearcher             kb.KBSearcher
	TenantService          *coreservices.TenantService
	SchemaMetadataProvider schema.MetadataProvider
	LearningStore          learning.LearningStore       // Optional: Dynamic learnings system
	ValidatedQueryStore    learning.ValidatedQueryStore // Optional: Validated query library

	// BiChat query executor pool (restricted permissions)
	// This pool should be configured with bichat_agent_role for SQL security
	QueryExecutorPool *pgxpool.Pool

	// Optional: Token Estimator for cost tracking and budget management
	// If not provided, a no-op estimator will be used
	TokenEstimator agents.TokenEstimator

	// Optional: Observability
	Logger                 *logrus.Logger
	EventBus               hooks.EventBus
	ObservabilityProviders []observability.Provider

	// Optional: Checkpointer for HITL (defaults to in-memory)
	Checkpointer Checkpointer

	// Capabilities controls frontend and orchestration behavior in a single place.
	Capabilities Capabilities
	Profile      FeatureProfile
	// CodeInterpreterMemoryLimit configures OpenAI code_interpreter container memory.
	// Allowed values: "1g", "4g", "16g", "64g".
	CodeInterpreterMemoryLimit string

	// Renderer for context block formatting (required, defaults to AnthropicRenderer)
	Renderer bichatcontext.Renderer

	// Attachment storage configuration
	AttachmentStorageBasePath string // e.g., "/var/lib/bichat/attachments"
	AttachmentStorageBaseURL  string // e.g., "https://example.com/bichat/attachments"
	AttachmentStorageMode     AttachmentStorageMode

	// Optional: Redis-backed title generation queue settings
	TitleQueueRedisURL       string
	TitleQueueStream         string
	TitleQueueGroup          string
	TitleQueueConsumer       string
	TitleQueuePollInterval   time.Duration
	TitleQueueReadBlock      time.Duration
	TitleQueueBatchSize      int
	TitleQueueMaxRetries     int
	TitleQueueRetryBaseDelay time.Duration
	TitleQueueRetryMaxDelay  time.Duration
	TitleQueuePendingIdle    time.Duration
	TitleQueueReconcileEvery time.Duration
	TitleQueueReconcileBatch int
	TitleQueueDedupeTTL      time.Duration
	TitleQueueJobTimeout     time.Duration

	// Optional: ViewManager manages analytics view definitions and syncs them to DB.
	// When configured, views are synced on startup and used for permission-based access control.
	ViewManager *analytics.ViewManager

	// Optional: Permission overrides for StreamController.
	// When not set, the module uses BiChat defaults.
	StreamRequireAccessPermission permission.Permission
	StreamReadAllPermission       permission.Permission

	// Internal: Created services (initialized during BuildServices)
	chatService       bichatservices.ChatService
	agentService      bichatservices.AgentService
	attachmentService bichatservices.AttachmentService
	artifactService   bichatservices.ArtifactService

	// Internal: Resolved once during BuildServices.
	resolvedProjectPromptExtension string
	projectPromptExtensionResolved bool
	capabilitiesConfigured         bool
	subAgentsInitialized           bool
	titleGenerationService         services.TitleGenerationService
	titleJobQueue                  *services.RedisTitleJobQueue
	skillsCatalog                  *bichatskills.Catalog
	skillsSelector                 bichatskills.Selector
}

// ConfigOption is a functional option for ModuleConfig
type ConfigOption func(*ModuleConfig)

var ErrTitleJobWorkerDisabled = errors.New("title job worker is disabled")

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

// WithSkillsSelectionLimit sets the maximum number of skills to inject per turn.
func WithSkillsSelectionLimit(limit int) ConfigOption {
	return func(c *ModuleConfig) {
		c.SkillsSelectionLimit = limit
	}
}

// WithSkillsMaxChars sets the maximum character budget for rendered skills context.
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

// WithTitleQueueRedis enables Redis-backed durable title generation queue.
func WithTitleQueueRedis(redisURL string) ConfigOption {
	return func(c *ModuleConfig) {
		c.TitleQueueRedisURL = strings.TrimSpace(redisURL)
	}
}

// WithTitleQueueStream overrides the Redis stream name for title jobs.
func WithTitleQueueStream(stream string) ConfigOption {
	return func(c *ModuleConfig) {
		c.TitleQueueStream = strings.TrimSpace(stream)
	}
}

// WithTitleQueueConsumerGroup overrides the Redis consumer group for title jobs.
func WithTitleQueueConsumerGroup(group string) ConfigOption {
	return func(c *ModuleConfig) {
		c.TitleQueueGroup = strings.TrimSpace(group)
	}
}

// WithTitleQueueConsumer overrides the consumer name used by title job worker.
func WithTitleQueueConsumer(consumer string) ConfigOption {
	return func(c *ModuleConfig) {
		c.TitleQueueConsumer = strings.TrimSpace(consumer)
	}
}

// WithTitleQueuePolling configures queue polling cadence and read block timeout.
func WithTitleQueuePolling(pollInterval, readBlock time.Duration) ConfigOption {
	return func(c *ModuleConfig) {
		c.TitleQueuePollInterval = pollInterval
		c.TitleQueueReadBlock = readBlock
	}
}

// WithTitleQueueRetry configures retry attempts and backoff windows.
func WithTitleQueueRetry(maxRetries int, baseDelay, maxDelay time.Duration) ConfigOption {
	return func(c *ModuleConfig) {
		c.TitleQueueMaxRetries = maxRetries
		c.TitleQueueRetryBaseDelay = baseDelay
		c.TitleQueueRetryMaxDelay = maxDelay
	}
}

// WithTitleQueueReconciliation configures periodic DB reconciliation for missing titles.
func WithTitleQueueReconciliation(interval time.Duration, batchSize int) ConfigOption {
	return func(c *ModuleConfig) {
		c.TitleQueueReconcileEvery = interval
		c.TitleQueueReconcileBatch = batchSize
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

// NewModuleConfig creates a new module configuration.
// Use ConfigOption functions to set optional dependencies.
func NewModuleConfig(
	tenantID func(ctx context.Context) uuid.UUID,
	userID func(ctx context.Context) int64,
	chatRepo domain.ChatRepository,
	model Model,
	policy bichatcontext.ContextPolicy,
	parentAgent Agent,
	opts ...ConfigOption,
) *ModuleConfig {
	cfg := &ModuleConfig{
		TenantID:                    tenantID,
		UserID:                      userID,
		ChatRepo:                    chatRepo,
		Model:                       model,
		ContextPolicy:               policy,
		ParentAgent:                 parentAgent,
		SubAgents:                   []Agent{},
		SubAgentDefinitionsFS:       bichatagents.DefaultSubAgentDefinitionsFS(),
		SubAgentDefinitionsBasePath: bichatagents.DefaultSubAgentDefinitionsBasePath,
		Logger:                      logrus.New(),
		Profile:                     FeatureProfileMinimal,
		AttachmentStorageMode:       AttachmentStorageModeLocal,
		TitleQueueStream:            defaultTitleQueueStream,
		TitleQueueGroup:             defaultTitleQueueGroup,
		TitleQueuePollInterval:      defaultTitleQueuePollInterval,
		TitleQueueReadBlock:         defaultTitleQueueReadBlock,
		TitleQueueBatchSize:         defaultTitleQueueBatchSize,
		TitleQueueMaxRetries:        defaultTitleQueueMaxRetries,
		TitleQueueRetryBaseDelay:    defaultTitleQueueRetryBaseDelay,
		TitleQueueRetryMaxDelay:     defaultTitleQueueRetryMaxDelay,
		TitleQueuePendingIdle:       defaultTitleQueuePendingIdle,
		TitleQueueReconcileEvery:    defaultTitleQueueReconcileEvery,
		TitleQueueReconcileBatch:    defaultTitleQueueReconcileBatch,
		TitleQueueDedupeTTL:         defaultTitleQueueDedupeTTL,
		TitleQueueJobTimeout:        defaultTitleQueueJobTimeout,
		SkillsSelectionLimit:        defaultSkillsSelectionLimit,
		SkillsMaxChars:              defaultSkillsMaxChars,
	}

	// Apply options
	for _, opt := range opts {
		opt(cfg)
	}

	// Set defaults
	if cfg.EventBus == nil {
		cfg.EventBus = hooks.NewEventBus()
	}

	if cfg.Checkpointer == nil {
		cfg.Checkpointer = agents.NewInMemoryCheckpointer()
	}

	// Default to proper token estimator (not NoOp)
	if cfg.TokenEstimator == nil {
		cfg.TokenEstimator = agents.NewTiktokenEstimator("cl100k_base")
	} else if _, isNoOp := cfg.TokenEstimator.(*agents.NoOpTokenEstimator); isNoOp {
		cfg.Logger.Warn("NoOpTokenEstimator configured - context window management disabled")
	}

	// Default to AnthropicRenderer
	if cfg.Renderer == nil {
		cfg.Renderer = renderers.NewAnthropicRenderer()
	}

	// Wire summarizer to context policy if compaction is enabled
	if cfg.ContextPolicy.Compaction != nil && cfg.ContextPolicy.Compaction.SummarizeHistory {
		if cfg.ContextPolicy.Summarizer == nil {
			// Create LLM-based summarizer using the configured model and estimator
			cfg.ContextPolicy.Summarizer = bichatcontext.NewLLMHistorySummarizer(
				cfg.Model,
				cfg.TokenEstimator,
			)
		}
	}

	capabilitiesFromProfile, ok := CapabilitiesForProfile(cfg.Profile)
	if !ok {
		cfg.Logger.WithField("profile", cfg.Profile).Warn("Unknown feature profile, defaulting to minimal")
		cfg.Profile = FeatureProfileMinimal
		capabilitiesFromProfile = Capabilities{}
	}
	if !cfg.capabilitiesConfigured {
		cfg.Capabilities = capabilitiesFromProfile
	}

	// Setup multi-agent system if enabled.
	if cfg.Capabilities.MultiAgent {
		if err := cfg.setupMultiAgentSystem(); err != nil {
			cfg.Logger.WithError(err).Warn("Failed to setup multi-agent system, continuing without delegation")
		}
	}

	return cfg
}

// setupMultiAgentSystem initializes shared multi-agent registry state.
// Concrete sub-agent definitions are loaded during BuildServices.
func (c *ModuleConfig) setupMultiAgentSystem() error {
	if c.AgentRegistry == nil {
		c.AgentRegistry = agents.NewAgentRegistry()
	}
	return nil
}

func (c *ModuleConfig) setupConfiguredSubAgents(fileStorage storage.FileStorage) error {
	const op serrors.Op = "ModuleConfig.setupConfiguredSubAgents"

	if !c.Capabilities.MultiAgent {
		return nil
	}
	if c.subAgentsInitialized {
		return nil
	}
	if c.AgentRegistry == nil {
		c.AgentRegistry = agents.NewAgentRegistry()
	}

	definitions, err := bichatagents.LoadSubAgentDefinitions(
		c.SubAgentDefinitionsFS,
		c.SubAgentDefinitionsBasePath,
	)
	if err != nil {
		return serrors.E(op, err, "failed to load sub-agent definitions")
	}

	buildOpts := make([]bichatagents.SubAgentBuildOption, 0, 1)
	if c.Model != nil {
		if modelName := strings.TrimSpace(c.Model.Info().Name); modelName != "" {
			buildOpts = append(buildOpts, bichatagents.WithSubAgentModel(modelName))
		}
	}

	deps := bichatagents.SubAgentDependencies{
		QueryExecutor:  c.QueryExecutor,
		ChatRepository: c.ChatRepo,
		FileStorage:    fileStorage,
	}

	for _, def := range definitions {
		subAgent, err := bichatagents.BuildSubAgent(def, deps, buildOpts...)
		if err != nil {
			return serrors.E(op, err, fmt.Sprintf("failed to build sub-agent %q", def.Name))
		}
		if err := c.AgentRegistry.Register(subAgent); err != nil {
			return serrors.E(op, err, fmt.Sprintf("failed to register sub-agent %q", def.Name))
		}
	}

	for _, subAgent := range c.SubAgents {
		if subAgent == nil {
			return serrors.E(op, serrors.KindValidation, "custom sub-agent cannot be nil")
		}
		if err := c.AgentRegistry.Register(subAgent); err != nil {
			return serrors.E(op, err, fmt.Sprintf("failed to register custom sub-agent %q", subAgent.Name()))
		}
	}

	c.subAgentsInitialized = true
	c.Logger.WithField("count", len(c.AgentRegistry.All())).Info("Multi-agent system initialized from markdown definitions")
	return nil
}

// Validate checks that all required configuration is present
func (c *ModuleConfig) Validate() error {
	if c.TenantID == nil {
		return errors.New("TenantID function is required")
	}
	if c.UserID == nil {
		return errors.New("UserID function is required")
	}
	if c.ChatRepo == nil {
		return errors.New("ChatRepository is required")
	}
	if c.Model == nil {
		return errors.New("Model is required")
	}
	if c.ParentAgent == nil && c.QueryExecutor == nil {
		return errors.New("ParentAgent is required when QueryExecutor is not configured")
	}
	if c.ContextPolicy.ContextWindow == 0 {
		return errors.New("ContextPolicy.ContextWindow must be set")
	}

	// Validate Renderer
	if c.Renderer == nil {
		return errors.New("renderer is required")
	}

	// Validate TokenEstimator
	if c.TokenEstimator == nil {
		return errors.New("TokenEstimator is required")
	}

	// Validate OverflowStrategy
	validStrategies := map[bichatcontext.OverflowStrategy]bool{
		bichatcontext.OverflowError:    true,
		bichatcontext.OverflowTruncate: true,
		bichatcontext.OverflowCompact:  true,
	}
	if !validStrategies[c.ContextPolicy.OverflowStrategy] {
		return fmt.Errorf("invalid OverflowStrategy: %s (must be error/truncate/compact)", c.ContextPolicy.OverflowStrategy)
	}

	// Validate OverflowCompact configuration
	if c.ContextPolicy.OverflowStrategy == bichatcontext.OverflowCompact {
		if c.ContextPolicy.Compaction == nil {
			return errors.New("OverflowStrategy=compact requires Compaction config")
		}

		// Warn if using NoOp estimator with compaction
		if _, isNoOp := c.TokenEstimator.(*agents.NoOpTokenEstimator); isNoOp {
			return errors.New("OverflowStrategy=compact requires accurate TokenEstimator (not NoOp)")
		}
	}

	switch c.AttachmentStorageMode {
	case AttachmentStorageModeLocal:
		if c.AttachmentStorageBasePath == "" {
			return errors.New("AttachmentStorageBasePath required for local attachment storage")
		}
		if c.AttachmentStorageBaseURL == "" {
			return errors.New("AttachmentStorageBaseURL required for local attachment storage")
		}
	case AttachmentStorageModeNoOp:
	default:
		return errors.New("AttachmentStorageMode must be one of: local, noop")
	}

	if strings.TrimSpace(c.TitleQueueRedisURL) != "" {
		if strings.TrimSpace(c.TitleQueueStream) == "" {
			return errors.New("TitleQueueStream is required when TitleQueueRedisURL is set")
		}
		if strings.TrimSpace(c.TitleQueueGroup) == "" {
			return errors.New("TitleQueueGroup is required when TitleQueueRedisURL is set")
		}
		if c.TitleQueueBatchSize <= 0 {
			return errors.New("TitleQueueBatchSize must be > 0")
		}
		if c.TitleQueuePollInterval <= 0 {
			return errors.New("TitleQueuePollInterval must be > 0")
		}
		if c.TitleQueueReadBlock <= 0 {
			return errors.New("TitleQueueReadBlock must be > 0")
		}
		if c.TitleQueueMaxRetries <= 0 {
			return errors.New("TitleQueueMaxRetries must be > 0")
		}
		if c.TitleQueueRetryBaseDelay <= 0 {
			return errors.New("TitleQueueRetryBaseDelay must be > 0")
		}
		if c.TitleQueueRetryMaxDelay <= 0 {
			return errors.New("TitleQueueRetryMaxDelay must be > 0")
		}
		if c.TitleQueueReconcileBatch <= 0 {
			return errors.New("TitleQueueReconcileBatch must be > 0")
		}
		if c.TitleQueueReconcileEvery <= 0 {
			return errors.New("TitleQueueReconcileEvery must be > 0")
		}
		if c.TitleQueueDedupeTTL <= 0 {
			return errors.New("TitleQueueDedupeTTL must be > 0")
		}
		if c.TitleQueueJobTimeout <= 0 {
			return errors.New("TitleQueueJobTimeout must be > 0")
		}
	}

	if c.CodeInterpreterMemoryLimit != "" {
		switch c.CodeInterpreterMemoryLimit {
		case "1g", "4g", "16g", "64g":
		default:
			return errors.New("CodeInterpreterMemoryLimit must be one of: 1g, 4g, 16g, 64g")
		}
	}

	if strings.TrimSpace(c.SkillsDir) != "" {
		if c.SkillsSelectionLimit <= 0 {
			return errors.New("SkillsSelectionLimit must be > 0 when SkillsDir is set")
		}
		if c.SkillsMaxChars <= 0 {
			return errors.New("SkillsMaxChars must be > 0 when SkillsDir is set")
		}
	}

	return nil
}

func defaultKindPriorities() []bichatcontext.KindPriority {
	return []bichatcontext.KindPriority{
		{Kind: bichatcontext.KindPinned, MinTokens: 1000, MaxTokens: 5000, Truncatable: false},
		{Kind: bichatcontext.KindReference, MinTokens: 2000, MaxTokens: 10000, Truncatable: true},
		{Kind: bichatcontext.KindMemory, MinTokens: 1000, MaxTokens: 5000, Truncatable: true},
		{Kind: bichatcontext.KindState, MinTokens: 500, MaxTokens: 2000, Truncatable: false},
		{Kind: bichatcontext.KindToolOutput, MinTokens: 2000, MaxTokens: 20000, Truncatable: true},
		{Kind: bichatcontext.KindHistory, MinTokens: 5000, MaxTokens: 100000, Truncatable: true},
		{Kind: bichatcontext.KindTurn, MinTokens: 1000, MaxTokens: 10000, Truncatable: false},
	}
}

// DefaultContextPolicy returns a sensible default context policy for Claude 3.5 Sonnet.
// Uses OverflowTruncate strategy - history is truncated when token budget is exceeded.
// For intelligent summarization, use DefaultContextPolicyWithCompaction.
func DefaultContextPolicy() bichatcontext.ContextPolicy {
	return bichatcontext.ContextPolicy{
		ContextWindow:     180000, // Claude 3.5 context window
		CompletionReserve: 8000,   // Reserve for completion
		OverflowStrategy:  bichatcontext.OverflowTruncate,
		KindPriorities:    defaultKindPriorities(),
		Compaction: &bichatcontext.CompactionConfig{
			PruneToolOutputs:   true,
			MaxToolOutputAge:   0, // Keep all by default
			SummarizeHistory:   false,
			MaxHistoryMessages: 0, // Keep all by default
		},
		MaxSensitivity:   bichatcontext.SensitivityInternal,
		RedactRestricted: true,
	}
}

// DefaultContextPolicyWithCompaction returns a context policy with intelligent compaction enabled.
// Uses OverflowCompact strategy with LLM-based history summarization.
// Summarizer will be automatically wired during NewModuleConfig if TokenEstimator is provided.
//
// Example:
//
//	policy := bichat.DefaultContextPolicyWithCompaction()
//	cfg := bichat.NewModuleConfig(
//	    tenantID, userID, chatRepo, model, policy, parentAgent,
//	    bichat.WithTokenEstimator(agents.NewTiktokenEstimator("cl100k_base")),
//	)
func DefaultContextPolicyWithCompaction() bichatcontext.ContextPolicy {
	return bichatcontext.ContextPolicy{
		ContextWindow:     180000,                        // Claude 3.5 context window
		CompletionReserve: 8000,                          // Reserve for completion
		OverflowStrategy:  bichatcontext.OverflowCompact, // Intelligent compaction
		KindPriorities:    defaultKindPriorities(),
		Compaction: &bichatcontext.CompactionConfig{
			PruneToolOutputs:      true,
			MaxToolOutputAge:      3600, // 1 hour (remove old tool outputs)
			MaxToolOutputsPerKind: 10,   // Keep max 10 outputs per tool
			SummarizeHistory:      true, // Enable LLM-based summarization
			MaxHistoryMessages:    20,   // Summarize if >20 messages
		},
		MaxSensitivity:   bichatcontext.SensitivityInternal,
		RedactRestricted: true,
		// Summarizer will be set automatically by NewModuleConfig
	}
}

func (c *ModuleConfig) resolveProjectPromptExtension() error {
	if c.projectPromptExtensionResolved {
		return nil
	}

	staticExtension := prompts.NormalizeProjectPromptExtension(c.ProjectPromptExtension)
	resolved := staticExtension

	if c.ProjectPromptExtensionProvider != nil {
		providerExtension, err := c.ProjectPromptExtensionProvider.ProjectPromptExtension()
		if err != nil {
			return err
		}
		providerExtension = prompts.NormalizeProjectPromptExtension(providerExtension)
		if providerExtension != "" {
			resolved = providerExtension
		}
	}

	c.resolvedProjectPromptExtension = resolved
	c.projectPromptExtensionResolved = true

	return nil
}

func (c *ModuleConfig) resolveSkillsSelector() error {
	if c.skillsSelector != nil {
		return nil
	}

	skillsDir := strings.TrimSpace(c.SkillsDir)
	if skillsDir == "" {
		return nil
	}

	catalog, err := bichatskills.LoadCatalog(skillsDir)
	if err != nil {
		return err
	}

	c.skillsCatalog = catalog
	c.skillsSelector = bichatskills.NewSelector(
		catalog,
		bichatskills.WithSelectionLimit(c.SkillsSelectionLimit),
		bichatskills.WithMaxChars(c.SkillsMaxChars),
	)
	return nil
}

func (c *ModuleConfig) createFileStorage() (storage.FileStorage, error) {
	switch c.AttachmentStorageMode {
	case AttachmentStorageModeNoOp:
		return storage.NewNoOpFileStorage(), nil
	case AttachmentStorageModeLocal:
		return storage.NewLocalFileStorage(
			c.AttachmentStorageBasePath,
			c.AttachmentStorageBaseURL,
		)
	default:
		return nil, errors.New("invalid attachment storage mode")
	}
}

// BuildServices creates and initializes all BiChat services from the configuration.
// This should be called after NewModuleConfig and before registering the module.
// Services are cached in the config and reused on subsequent calls.
//
// This method fails fast - any error in service creation returns immediately.
func (c *ModuleConfig) BuildServices() error {
	const op serrors.Op = "ModuleConfig.BuildServices"

	// Validate configuration first
	if err := c.Validate(); err != nil {
		return serrors.E(op, err)
	}

	if err := c.resolveProjectPromptExtension(); err != nil {
		return serrors.E(op, err, "failed to resolve project prompt extension")
	}
	if err := c.resolveSkillsSelector(); err != nil {
		return serrors.E(op, err, "failed to load skills catalog")
	}

	// Create file storage once for attachment/artifact services and artifact_reader tool wiring.
	var fileStorage storage.FileStorage
	needsFileStorage := c.ParentAgent == nil || c.attachmentService == nil || c.artifactService == nil || c.Capabilities.CodeInterpreter || c.Capabilities.MultiAgent
	if needsFileStorage {
		fs, err := c.createFileStorage()
		if err != nil {
			return serrors.E(op, err, "failed to create file storage")
		}
		fileStorage = fs
	}

	if c.Capabilities.CodeInterpreter {
		if configurable, ok := c.Model.(interface {
			SetCodeInterpreterArtifactSource(domain.ChatRepository, storage.FileStorage)
		}); ok {
			configurable.SetCodeInterpreterArtifactSource(c.ChatRepo, fileStorage)
		}
		if c.CodeInterpreterMemoryLimit != "" {
			if configurable, ok := c.Model.(interface{ SetCodeInterpreterMemoryLimit(string) error }); ok {
				if err := configurable.SetCodeInterpreterMemoryLimit(c.CodeInterpreterMemoryLimit); err != nil {
					return serrors.E(op, err)
				}
			}
		}
	}

	// Register markdown-defined and custom sub-agents before building default parent
	// so delegation descriptions include all available agents.
	if err := c.setupConfiguredSubAgents(fileStorage); err != nil {
		return serrors.E(op, err, "failed to setup configured sub-agents")
	}

	// Build default parent agent from config when caller did not provide one.
	if err := c.buildParentAgent(fileStorage); err != nil {
		return serrors.E(op, err)
	}

	// Build AgentService first (ChatService depends on it)
	if c.agentService == nil {
		c.agentService = services.NewAgentService(services.AgentServiceConfig{
			Agent:                  c.ParentAgent,
			Model:                  c.Model,
			Policy:                 c.ContextPolicy,
			Renderer:               c.Renderer, // Use configured renderer
			Checkpointer:           c.Checkpointer,
			EventBus:               c.EventBus,
			ChatRepo:               c.ChatRepo,
			AgentRegistry:          c.AgentRegistry,
			SchemaMetadata:         c.SchemaMetadataProvider,
			ProjectPromptExtension: c.resolvedProjectPromptExtension,
			SkillsSelector:         c.skillsSelector,
			Logger:                 c.Logger,
			FormatterRegistry:      formatters.DefaultFormatterRegistry(),
		})
	}

	titleService, err := services.NewTitleGenerationService(c.Model, c.ChatRepo, c.EventBus)
	if err != nil {
		return serrors.E(op, err, "failed to create title generation service")
	}
	c.titleGenerationService = titleService

	if strings.TrimSpace(c.TitleQueueRedisURL) != "" && c.titleJobQueue == nil {
		queue, err := services.NewRedisTitleJobQueue(services.RedisTitleJobQueueConfig{
			RedisURL:  c.TitleQueueRedisURL,
			Stream:    c.TitleQueueStream,
			DedupeTTL: c.TitleQueueDedupeTTL,
		})
		if err != nil {
			return serrors.E(op, err, "failed to create redis title job queue")
		}
		c.titleJobQueue = queue
	}

	// Build ChatService
	if c.chatService == nil {
		c.chatService = services.NewChatService(
			c.ChatRepo,
			c.agentService,
			c.Model,
			titleService,
			c.titleJobQueue,
		)
	}

	// Build AttachmentService
	if c.attachmentService == nil {
		c.attachmentService = services.NewAttachmentService(fileStorage)
	}

	// Build ArtifactService
	if c.artifactService == nil {
		c.artifactService = bichatservices.NewArtifactService(c.ChatRepo, fileStorage, c.attachmentService)
	}

	return nil
}

// BuildParentAgent creates the default BI parent agent when ParentAgent is nil.
// It applies KB, learning, validated query, and code interpreter options from ModuleConfig.
func (c *ModuleConfig) BuildParentAgent() error {
	return c.buildParentAgent(nil)
}

func (c *ModuleConfig) buildParentAgent(fileStorage storage.FileStorage) error {
	const op serrors.Op = "ModuleConfig.BuildParentAgent"

	if c.ParentAgent != nil {
		return nil
	}

	if c.QueryExecutor == nil {
		return serrors.E(op, serrors.KindValidation, "ParentAgent or QueryExecutor is required")
	}

	opts := make([]bichatagents.BIAgentOption, 0, 6)
	if c.KBSearcher != nil {
		opts = append(opts, bichatagents.WithKBSearcher(c.KBSearcher))
	}
	if c.LearningStore != nil {
		opts = append(opts, bichatagents.WithLearningStore(c.LearningStore))
	}
	if c.ValidatedQueryStore != nil {
		opts = append(opts, bichatagents.WithValidatedQueryStore(c.ValidatedQueryStore))
	}
	if c.AgentRegistry != nil {
		opts = append(opts, bichatagents.WithAgentRegistry(c.AgentRegistry))
	}
	if c.Capabilities.CodeInterpreter {
		opts = append(opts, bichatagents.WithCodeInterpreter(true))
	}
	if c.Model != nil {
		modelName := strings.TrimSpace(c.Model.Info().Name)
		if modelName != "" {
			opts = append(opts, bichatagents.WithModel(modelName))
		}
	}
	if c.ChatRepo != nil && fileStorage != nil {
		opts = append(opts, bichatagents.WithArtifactReaderTool(
			bichattools.NewArtifactReaderTool(c.ChatRepo, fileStorage),
		))
	}

	parentAgent, err := bichatagents.NewDefaultBIAgent(c.QueryExecutor, opts...)
	if err != nil {
		return serrors.E(op, err)
	}
	c.ParentAgent = parentAgent

	return nil
}

// ChatService returns the cached ChatService instance.
// Returns nil if BuildServices hasn't been called yet.
func (c *ModuleConfig) ChatService() bichatservices.ChatService {
	return c.chatService
}

// AgentService returns the cached AgentService instance.
// Returns nil if BuildServices hasn't been called yet.
func (c *ModuleConfig) AgentService() bichatservices.AgentService {
	return c.agentService
}

// AttachmentService returns the cached AttachmentService instance.
// Returns nil if BuildServices hasn't been called yet.
func (c *ModuleConfig) AttachmentService() bichatservices.AttachmentService {
	return c.attachmentService
}

// ArtifactService returns the cached ArtifactService instance.
// Returns nil if BuildServices hasn't been called yet.
func (c *ModuleConfig) ArtifactService() bichatservices.ArtifactService {
	return c.artifactService
}

// NewTitleJobWorker builds a Redis-backed title generation worker when queueing is enabled.
// Returns ErrTitleJobWorkerDisabled when queueing is not configured.
func (c *ModuleConfig) NewTitleJobWorker(pool *pgxpool.Pool) (*services.TitleJobWorker, error) {
	if c.titleJobQueue == nil || c.titleGenerationService == nil {
		return nil, ErrTitleJobWorkerDisabled
	}
	if pool == nil {
		return nil, errors.New("database pool is required for title job worker")
	}

	return services.NewTitleJobWorker(services.TitleJobWorkerConfig{
		Queue:          c.titleJobQueue,
		TitleService:   c.titleGenerationService,
		Pool:           pool,
		Logger:         c.Logger,
		Group:          c.TitleQueueGroup,
		Consumer:       c.TitleQueueConsumer,
		BatchSize:      c.TitleQueueBatchSize,
		PollInterval:   c.TitleQueuePollInterval,
		ReadBlock:      c.TitleQueueReadBlock,
		MaxRetries:     c.TitleQueueMaxRetries,
		RetryBaseDelay: c.TitleQueueRetryBaseDelay,
		RetryMaxDelay:  c.TitleQueueRetryMaxDelay,
		PendingIdle:    c.TitleQueuePendingIdle,
		ReconcileEvery: c.TitleQueueReconcileEvery,
		ReconcileBatch: c.TitleQueueReconcileBatch,
		JobTimeout:     c.TitleQueueJobTimeout,
	})
}

// CloseTitleQueue releases queue resources when Redis queueing is configured.
func (c *ModuleConfig) CloseTitleQueue() error {
	if c.titleJobQueue == nil {
		return nil
	}
	err := c.titleJobQueue.Close()
	c.titleJobQueue = nil
	return err
}

// String provides a human-readable representation of the configuration
func (c *ModuleConfig) String() string {
	return fmt.Sprintf("BIChatConfig{Model: %v, SubAgents: %d, EventBus: %v, QueryExecutor: %v, KBSearcher: %v}",
		c.Model != nil,
		len(c.SubAgents),
		c.EventBus != nil,
		c.QueryExecutor != nil,
		c.KBSearcher != nil,
	)
}
