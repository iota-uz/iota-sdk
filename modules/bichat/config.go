package bichat

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	bichatcontext "github.com/iota-uz/iota-sdk/pkg/bichat/context"
	"github.com/iota-uz/iota-sdk/pkg/bichat/domain"
	"github.com/iota-uz/iota-sdk/pkg/bichat/hooks"
	"github.com/iota-uz/iota-sdk/pkg/bichat/kb"
	"github.com/iota-uz/iota-sdk/pkg/bichat/services"
	"github.com/sirupsen/logrus"
)

// TODO: Remove these placeholder types when Phase 1 (Agent Framework) is complete.
// These are temporary stubs to allow compilation until the agent framework is fully implemented.

// Agent is a placeholder for the agent framework interface (Phase 1 pending).
type Agent interface{}

// Model is a placeholder for the LLM model interface (Phase 1 pending).
type Model interface{}

// ModelRegistry is a placeholder for multi-model support (Phase 1 pending).
type ModelRegistry interface{}

// Checkpointer is a placeholder for HITL state persistence (Phase 1 pending).
type Checkpointer interface{}

// ModuleConfig holds configuration for the bichat module.
// It uses functional options pattern for optional dependencies.
type ModuleConfig struct {
	// Required: Core dependencies
	TenantID func(ctx context.Context) uuid.UUID
	UserID   func(ctx context.Context) int64
	ChatRepo domain.ChatRepository

	// Required: LLM Model (TODO: Use agents.Model when Phase 1 is complete)
	Model Model

	// Optional: LLM Model Registry (for multi-model support)
	ModelRegistry ModelRegistry

	// Required: Context management
	ContextPolicy bichatcontext.ContextPolicy

	// Required: Agents (TODO: Use agents.Agent when Phase 1 is complete)
	ParentAgent Agent
	SubAgents   []Agent

	// Optional services (can be nil)
	QueryExecutor services.QueryExecutorService
	KBSearcher    kb.KBSearcher
	// TODO: Add Summarizer when HistorySummarizer interface is implemented in pkg/bichat/context
	// Summarizer    bichatcontext.HistorySummarizer

	// Optional: Observability
	Logger   *logrus.Logger
	EventBus hooks.EventBus

	// Optional: Checkpointer for HITL (defaults to in-memory)
	Checkpointer Checkpointer
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
func WithQueryExecutor(executor services.QueryExecutorService) ConfigOption {
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

// TODO: Uncomment when HistorySummarizer interface is implemented
// WithSummarizer sets the history summarizer for context compaction
// func WithSummarizer(summarizer bichatcontext.HistorySummarizer) ConfigOption {
// 	return func(c *ModuleConfig) {
// 		c.Summarizer = summarizer
// 	}
// }

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

// NewModuleConfig creates a new module configuration with required dependencies.
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

	// TODO: Uncomment when Phase 1 is complete
	// if cfg.Checkpointer == nil {
	// 	cfg.Checkpointer = agents.NewInMemoryCheckpointer()
	// }

	return cfg
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
	if c.ParentAgent == nil {
		return errors.New("ParentAgent is required")
	}
	if c.ContextPolicy.ContextWindow == 0 {
		return errors.New("ContextPolicy.ContextWindow must be set")
	}
	return nil
}

// DefaultContextPolicy returns a sensible default context policy for Claude 3.5 Sonnet
func DefaultContextPolicy() bichatcontext.ContextPolicy {
	return bichatcontext.ContextPolicy{
		ContextWindow:     180000, // Claude 3.5 context window
		CompletionReserve: 8000,   // Reserve for completion
		OverflowStrategy:  bichatcontext.OverflowTruncate,
		KindPriorities: []bichatcontext.KindPriority{
			{Kind: bichatcontext.KindPinned, MinTokens: 1000, MaxTokens: 5000},
			{Kind: bichatcontext.KindReference, MinTokens: 2000, MaxTokens: 10000},
			{Kind: bichatcontext.KindMemory, MinTokens: 1000, MaxTokens: 5000},
			{Kind: bichatcontext.KindState, MinTokens: 500, MaxTokens: 2000},
			{Kind: bichatcontext.KindToolOutput, MinTokens: 2000, MaxTokens: 20000},
			{Kind: bichatcontext.KindHistory, MinTokens: 5000, MaxTokens: 100000},
			{Kind: bichatcontext.KindTurn, MinTokens: 1000, MaxTokens: 10000},
		},
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
