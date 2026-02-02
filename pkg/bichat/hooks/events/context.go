package events

import (
	"github.com/google/uuid"
)

// ContextCompileEvent captures details of context compilation.
// Emitted when a context builder compiles blocks into provider-specific format.
type ContextCompileEvent struct {
	baseEvent
	Provider       string         // LLM provider ("anthropic", "openai", etc.)
	TotalTokens    int            // Total tokens in compiled context
	TokensByKind   map[string]int // Token count per BlockKind
	BlockCount     int            // Total number of blocks
	Compacted      bool           // True if history was summarized
	Truncated      bool           // True if content was truncated due to overflow
	ExcludedBlocks int            // Number of blocks excluded (e.g., due to sensitivity filtering)
}

// NewContextCompileEvent creates a new ContextCompileEvent.
func NewContextCompileEvent(
	sessionID, tenantID uuid.UUID,
	provider string,
	totalTokens int,
	tokensByKind map[string]int,
	blockCount int,
	compacted, truncated bool,
	excludedBlocks int,
) *ContextCompileEvent {
	return &ContextCompileEvent{
		baseEvent:      newBaseEvent("context.compile", sessionID, tenantID),
		Provider:       provider,
		TotalTokens:    totalTokens,
		TokensByKind:   tokensByKind,
		BlockCount:     blockCount,
		Compacted:      compacted,
		Truncated:      truncated,
		ExcludedBlocks: excludedBlocks,
	}
}

// ContextCompactEvent indicates context history was summarized.
// Emitted when automatic compaction reduces conversation history to fit token budget.
type ContextCompactEvent struct {
	baseEvent
	OriginalMessages int    // Number of messages before compaction
	CompactedTo      int    // Number of messages after compaction
	TokensSaved      int    // Tokens saved by compaction
	SummaryText      string // Generated summary (if applicable)
}

// NewContextCompactEvent creates a new ContextCompactEvent.
func NewContextCompactEvent(sessionID, tenantID uuid.UUID, originalMessages, compactedTo, tokensSaved int, summaryText string) *ContextCompactEvent {
	return &ContextCompactEvent{
		baseEvent:        newBaseEvent("context.compact", sessionID, tenantID),
		OriginalMessages: originalMessages,
		CompactedTo:      compactedTo,
		TokensSaved:      tokensSaved,
		SummaryText:      summaryText,
	}
}

// ContextOverflowEvent indicates the context exceeded the token budget.
// Emitted when compilation fails due to insufficient token budget.
type ContextOverflowEvent struct {
	baseEvent
	RequestedTokens  int    // Tokens needed to fit all content
	AvailableTokens  int    // Tokens available (context window - completion reserve)
	OverflowStrategy string // Strategy used: "error", "truncate", "compact"
	Resolved         bool   // True if overflow was resolved by the strategy
}

// NewContextOverflowEvent creates a new ContextOverflowEvent.
func NewContextOverflowEvent(sessionID, tenantID uuid.UUID, requestedTokens, availableTokens int, strategy string, resolved bool) *ContextOverflowEvent {
	return &ContextOverflowEvent{
		baseEvent:        newBaseEvent("context.overflow", sessionID, tenantID),
		RequestedTokens:  requestedTokens,
		AvailableTokens:  availableTokens,
		OverflowStrategy: strategy,
		Resolved:         resolved,
	}
}
