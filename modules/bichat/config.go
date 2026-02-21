package bichat

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
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
		TitleQueueConsumer:          defaultTitleQueueConsumer(),
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

func defaultTitleQueueConsumer() string {
	host, err := os.Hostname()
	if err == nil {
		host = strings.TrimSpace(host)
		if host != "" {
			return host
		}
	}
	return fmt.Sprintf("consumer-%d", os.Getpid())
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
