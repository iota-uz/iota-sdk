package bichat

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	bichatagents "github.com/iota-uz/iota-sdk/modules/bichat/agents"
	"github.com/iota-uz/iota-sdk/modules/bichat/services"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/permission"
	coreservices "github.com/iota-uz/iota-sdk/modules/core/services"
	"github.com/iota-uz/iota-sdk/pkg/analytics"
	"github.com/iota-uz/iota-sdk/pkg/bichat/agents"
	bichatcontext "github.com/iota-uz/iota-sdk/pkg/bichat/context"
	"github.com/iota-uz/iota-sdk/pkg/bichat/context/renderers"
	"github.com/iota-uz/iota-sdk/pkg/bichat/domain"
	"github.com/iota-uz/iota-sdk/pkg/bichat/hooks"
	"github.com/iota-uz/iota-sdk/pkg/bichat/kb"
	"github.com/iota-uz/iota-sdk/pkg/bichat/learning"
	"github.com/iota-uz/iota-sdk/pkg/bichat/observability"
	"github.com/iota-uz/iota-sdk/pkg/bichat/prompts"
	"github.com/iota-uz/iota-sdk/pkg/bichat/schema"
	bichatservices "github.com/iota-uz/iota-sdk/pkg/bichat/services"
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

	// Optional: Project-scoped prompt extension appended to parent agent system prompt.
	ProjectPromptExtension         string
	ProjectPromptExtensionProvider prompts.ProjectPromptExtensionProvider

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

	// Feature Flags (code-based configuration)
	// These flags are passed to the React frontend via applet context
	EnableVision          bool // Enable vision/image analysis capabilities
	EnableWebSearch       bool // Enable web search tool
	EnableCodeInterpreter bool // Enable code execution capabilities
	EnableMultiAgent      bool // Enable multi-agent orchestration

	// Renderer for context block formatting (required, defaults to AnthropicRenderer)
	Renderer bichatcontext.Renderer

	// Attachment storage configuration
	AttachmentStorageBasePath string // e.g., "/var/lib/bichat/attachments"
	AttachmentStorageBaseURL  string // e.g., "https://example.com/bichat/attachments"
	DisableAttachmentStorage  bool   // Explicit disable (testing only)

	// Feature toggles
	DisableTitleGeneration bool // Disable auto-title generation

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
}

// ConfigOption is a functional option for ModuleConfig
type ConfigOption func(*ModuleConfig)

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

// WithVision enables vision/image analysis capabilities
func WithVision(enabled bool) ConfigOption {
	return func(c *ModuleConfig) {
		c.EnableVision = enabled
	}
}

// WithWebSearch enables web search tool
func WithWebSearch(enabled bool) ConfigOption {
	return func(c *ModuleConfig) {
		c.EnableWebSearch = enabled
	}
}

// WithCodeInterpreter enables code execution capabilities
func WithCodeInterpreter(enabled bool) ConfigOption {
	return func(c *ModuleConfig) {
		c.EnableCodeInterpreter = enabled
	}
}

// WithMultiAgent enables multi-agent orchestration
func WithMultiAgent(enabled bool) ConfigOption {
	return func(c *ModuleConfig) {
		c.EnableMultiAgent = enabled
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

// WithNoOpAttachmentStorage disables attachment storage (testing only).
// Use this explicitly in tests to bypass attachment storage requirements.
func WithNoOpAttachmentStorage() ConfigOption {
	return func(c *ModuleConfig) {
		c.DisableAttachmentStorage = true
	}
}

// WithTitleGenerationDisabled disables automatic session title generation.
// When disabled, users must manually provide session titles.
func WithTitleGenerationDisabled() ConfigOption {
	return func(c *ModuleConfig) {
		c.DisableTitleGeneration = true
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
		TenantID:      tenantID,
		UserID:        userID,
		ChatRepo:      chatRepo,
		Model:         model,
		ContextPolicy: policy,
		ParentAgent:   parentAgent,
		SubAgents:     []Agent{},
		Logger:        logrus.New(),
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

	// Setup multi-agent system if enabled
	if cfg.EnableMultiAgent && cfg.QueryExecutor != nil {
		if err := cfg.setupMultiAgentSystem(); err != nil {
			cfg.Logger.WithError(err).Warn("Failed to setup multi-agent system, continuing without delegation")
		}
	}

	return cfg
}

// setupMultiAgentSystem creates and configures the agent registry with sub-agents.
// This is called automatically during NewModuleConfig if EnableMultiAgent is true.
//
// The registry is stored in ModuleConfig and can be accessed at execution time
// to create delegation tools with runtime session/tenant IDs.
//
// If the ParentAgent is a DefaultBIAgent, it will be updated with the registry
// so it can include available agents in its system prompt.
func (c *ModuleConfig) setupMultiAgentSystem() error {
	const op serrors.Op = "ModuleConfig.setupMultiAgentSystem"

	// Create agent registry if not already created
	if c.AgentRegistry == nil {
		c.AgentRegistry = agents.NewAgentRegistry()
	}

	// Create and register SQL agent if query executor is available
	if c.QueryExecutor != nil {
		sqlAgent, err := bichatagents.NewSQLAgent(c.QueryExecutor)
		if err != nil {
			return serrors.E(op, err, "failed to create SQL agent")
		}

		// Register SQL agent in registry
		if err := c.AgentRegistry.Register(sqlAgent); err != nil {
			return serrors.E(op, err, "failed to register SQL agent")
		}

		// Add to SubAgents list for tracking
		c.SubAgents = append(c.SubAgents, sqlAgent)

		c.Logger.Info("Multi-agent system initialized with SQL analyst agent")
	}

	// If parent agent is a DefaultBIAgent, update it with registry for system prompt
	// Note: We don't recreate the agent here because the agent options (tools, KB searcher)
	// may have already been configured. Instead, we rely on the agent being created
	// with WithAgentRegistry option in the module setup code.
	// The registry is available via c.AgentRegistry for service layer to use.

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

	// Validate attachment storage if enabled
	if !c.DisableAttachmentStorage {
		if c.AttachmentStorageBasePath == "" {
			return errors.New("AttachmentStorageBasePath required - use WithAttachmentStorage(path, url) or WithNoOpAttachmentStorage()")
		}
		if c.AttachmentStorageBaseURL == "" {
			return errors.New("AttachmentStorageBaseURL required - use WithAttachmentStorage(path, url) or WithNoOpAttachmentStorage()")
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

	// Create file storage once for attachment/artifact services and artifact_reader tool wiring.
	var fileStorage storage.FileStorage
	if c.ParentAgent == nil || c.attachmentService == nil || c.artifactService == nil {
		if c.DisableAttachmentStorage {
			fileStorage = storage.NewNoOpFileStorage()
		} else {
			fs, err := storage.NewLocalFileStorage(
				c.AttachmentStorageBasePath,
				c.AttachmentStorageBaseURL,
			)
			if err != nil {
				return serrors.E(op, err, "failed to create file storage")
			}
			fileStorage = fs
		}
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
		})
	}

	// Build TitleGenerationService (required unless disabled)
	var titleService services.TitleGenerationService
	if !c.DisableTitleGeneration {
		titleSvc, err := services.NewTitleGenerationService(c.Model, c.ChatRepo)
		if err != nil {
			return serrors.E(op, err, "failed to create title generation service")
		}
		titleService = titleSvc
	}

	// Build ChatService
	if c.chatService == nil {
		c.chatService = services.NewChatService(
			c.ChatRepo,
			c.agentService,
			c.Model,
			titleService, // Can be nil if disabled
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
	if c.EnableCodeInterpreter {
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
