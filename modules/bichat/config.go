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
	defaultSkillsSelectionLimit = 3
	defaultSkillsMaxChars       = 8000
)

// TitleQueueConfig holds Redis-backed async title generation queue settings.
// A nil *TitleQueueConfig means title queueing is disabled (titles are generated synchronously).
type TitleQueueConfig struct {
	RedisURL       string
	Stream         string
	Group          string
	Consumer       string
	PollInterval   time.Duration
	ReadBlock      time.Duration
	BatchSize      int
	MaxRetries     int
	RetryBaseDelay time.Duration
	RetryMaxDelay  time.Duration
	PendingIdle    time.Duration
	ReconcileEvery time.Duration
	ReconcileBatch int
	DedupeTTL      time.Duration
	JobTimeout     time.Duration
}

// DefaultTitleQueueConfig returns a TitleQueueConfig with sensible defaults for the given Redis URL.
func DefaultTitleQueueConfig(redisURL string) *TitleQueueConfig {
	return &TitleQueueConfig{
		RedisURL:       strings.TrimSpace(redisURL),
		Stream:         "bichat:title:jobs",
		Group:          "bichat-title-workers",
		Consumer:       defaultTitleQueueConsumer(),
		PollInterval:   300 * time.Millisecond,
		ReadBlock:      2 * time.Second,
		BatchSize:      16,
		MaxRetries:     3,
		RetryBaseDelay: 5 * time.Second,
		RetryMaxDelay:  2 * time.Minute,
		PendingIdle:    30 * time.Second,
		ReconcileEvery: 1 * time.Minute,
		ReconcileBatch: 200,
		DedupeTTL:      30 * time.Minute,
		JobTimeout:     20 * time.Second,
	}
}

// Validate checks that all TitleQueueConfig fields are valid.
func (tq *TitleQueueConfig) Validate() error {
	if strings.TrimSpace(tq.RedisURL) == "" {
		return errors.New("TitleQueueConfig.RedisURL is required")
	}
	if strings.TrimSpace(tq.Stream) == "" {
		return errors.New("TitleQueueConfig.Stream is required")
	}
	if strings.TrimSpace(tq.Group) == "" {
		return errors.New("TitleQueueConfig.Group is required")
	}
	if tq.BatchSize <= 0 {
		return errors.New("TitleQueueConfig.BatchSize must be > 0")
	}
	if tq.PollInterval <= 0 {
		return errors.New("TitleQueueConfig.PollInterval must be > 0")
	}
	if tq.ReadBlock <= 0 {
		return errors.New("TitleQueueConfig.ReadBlock must be > 0")
	}
	if tq.MaxRetries <= 0 {
		return errors.New("TitleQueueConfig.MaxRetries must be > 0")
	}
	if tq.RetryBaseDelay <= 0 {
		return errors.New("TitleQueueConfig.RetryBaseDelay must be > 0")
	}
	if tq.RetryMaxDelay <= 0 {
		return errors.New("TitleQueueConfig.RetryMaxDelay must be > 0")
	}
	if tq.ReconcileBatch <= 0 {
		return errors.New("TitleQueueConfig.ReconcileBatch must be > 0")
	}
	if tq.ReconcileEvery <= 0 {
		return errors.New("TitleQueueConfig.ReconcileEvery must be > 0")
	}
	if tq.DedupeTTL <= 0 {
		return errors.New("TitleQueueConfig.DedupeTTL must be > 0")
	}
	if tq.JobTimeout <= 0 {
		return errors.New("TitleQueueConfig.JobTimeout must be > 0")
	}
	return nil
}

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

	// Optional: Redis-backed title generation queue.
	// nil means title queueing is disabled (titles generated synchronously).
	// Use WithTitleQueueRedis(url) or set directly.
	TitleQueue *TitleQueueConfig

	// Optional: ViewManager manages analytics view definitions and syncs them to DB.
	// When configured, views are synced on startup and used for permission-based access control.
	ViewManager *analytics.ViewManager

	// Optional: Permission overrides for StreamController.
	// When not set, the module uses BiChat defaults.
	StreamRequireAccessPermission permission.Permission
	StreamReadAllPermission       permission.Permission

	// Internal: Build-time state (resolved once during BuildServices).
	resolvedProjectPromptExtension string
	projectPromptExtensionResolved bool
	capabilitiesConfigured         bool
	subAgentsInitialized           bool
	skillsCatalog                  *bichatskills.Catalog
	skillsSelector                 bichatskills.Selector
}

// ServiceContainer holds built services created by ModuleConfig.BuildServices().
// Use accessor methods to retrieve individual services.
type ServiceContainer struct {
	chatService       bichatservices.ChatService
	agentService      bichatservices.AgentService
	attachmentService bichatservices.AttachmentService
	artifactService   bichatservices.ArtifactService
	titleService      services.TitleService
	titleJobQueue     *services.RedisTitleJobQueue
	titleQueueConfig  *TitleQueueConfig
	logger            *logrus.Logger
}

// ChatService returns the ChatService.
func (sc *ServiceContainer) ChatService() bichatservices.ChatService { return sc.chatService }

// AgentService returns the AgentService.
func (sc *ServiceContainer) AgentService() bichatservices.AgentService { return sc.agentService }

// AttachmentService returns the AttachmentService.
func (sc *ServiceContainer) AttachmentService() bichatservices.AttachmentService {
	return sc.attachmentService
}

// ArtifactService returns the ArtifactService.
func (sc *ServiceContainer) ArtifactService() bichatservices.ArtifactService {
	return sc.artifactService
}

// NewTitleJobWorker builds a Redis-backed title generation worker when queueing is enabled.
// Returns ErrTitleJobWorkerDisabled when queueing is not configured.
func (sc *ServiceContainer) NewTitleJobWorker(pool *pgxpool.Pool) (*services.TitleJobWorker, error) {
	if sc.titleJobQueue == nil || sc.titleService == nil {
		return nil, ErrTitleJobWorkerDisabled
	}
	if pool == nil {
		return nil, errors.New("database pool is required for title job worker")
	}

	tq := sc.titleQueueConfig
	return services.NewTitleJobWorker(services.TitleJobWorkerConfig{
		Queue:          sc.titleJobQueue,
		TitleService:   sc.titleService,
		Pool:           pool,
		Logger:         sc.logger,
		Group:          tq.Group,
		Consumer:       tq.Consumer,
		BatchSize:      tq.BatchSize,
		PollInterval:   tq.PollInterval,
		ReadBlock:      tq.ReadBlock,
		MaxRetries:     tq.MaxRetries,
		RetryBaseDelay: tq.RetryBaseDelay,
		RetryMaxDelay:  tq.RetryMaxDelay,
		PendingIdle:    tq.PendingIdle,
		ReconcileEvery: tq.ReconcileEvery,
		ReconcileBatch: tq.ReconcileBatch,
		JobTimeout:     tq.JobTimeout,
	})
}

// CloseTitleQueue releases queue resources when Redis queueing is configured.
func (sc *ServiceContainer) CloseTitleQueue() error {
	if sc.titleJobQueue == nil {
		return nil
	}
	err := sc.titleJobQueue.Close()
	sc.titleJobQueue = nil
	return err
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

	if c.TitleQueue != nil {
		if err := c.TitleQueue.Validate(); err != nil {
			return err
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

// DefaultContextPolicy returns a sensible default context policy for modern models (e.g. claude-sonnet-4-6, gpt-5.2).
// Uses OverflowTruncate strategy - history is truncated when token budget is exceeded.
// For intelligent summarization, use DefaultContextPolicyWithCompaction.
func DefaultContextPolicy() bichatcontext.ContextPolicy {
	return bichatcontext.ContextPolicy{
		ContextWindow:     200000, // SOTA context window
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
		ContextWindow:     200000,                        // SOTA context window
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
