package context

// OverflowStrategy determines how to handle token budget overflow.
type OverflowStrategy string

const (
	// OverflowError throws an error when token budget is exceeded.
	OverflowError OverflowStrategy = "error"

	// OverflowTruncate removes blocks from the end until budget is met.
	OverflowTruncate OverflowStrategy = "truncate"

	// OverflowCompact runs compaction (remove old tool outputs, summarize history).
	OverflowCompact OverflowStrategy = "compact"
)

// KindPriority configures token allocation for a specific block kind.
type KindPriority struct {
	// Kind is the block kind.
	Kind BlockKind

	// MinTokens is the minimum guaranteed token reservation.
	MinTokens int

	// MaxTokens is the maximum token allocation (soft limit).
	MaxTokens int

	// Truncatable indicates if blocks of this kind can be truncated on overflow.
	Truncatable bool
}

// CompactionConfig controls automatic context compaction.
type CompactionConfig struct {
	// PruneToolOutputs enables removal of old tool outputs.
	PruneToolOutputs bool

	// MaxToolOutputAge is the maximum age (in seconds) of tool outputs to keep.
	MaxToolOutputAge int

	// MaxToolOutputsPerKind limits the number of tool outputs per kind.
	MaxToolOutputsPerKind int

	// SummarizeHistory enables conversation history summarization.
	SummarizeHistory bool

	// MaxHistoryMessages is the maximum number of messages to keep before summarization.
	MaxHistoryMessages int
}

// ContextPolicy configures context compilation and token budgeting.
type ContextPolicy struct {
	// ContextWindow is the total context window size (tokens).
	ContextWindow int

	// CompletionReserve is the number of tokens reserved for completion.
	CompletionReserve int

	// OverflowStrategy determines how to handle token budget overflow.
	OverflowStrategy OverflowStrategy

	// KindPriorities configures token allocation per kind.
	KindPriorities []KindPriority

	// Compaction configures automatic compaction (if OverflowStrategy = compact).
	Compaction *CompactionConfig

	// MaxSensitivity is the maximum sensitivity level to include.
	MaxSensitivity SensitivityLevel

	// RedactRestricted redacts restricted content instead of removing it.
	RedactRestricted bool
}

// DefaultKindPriorities returns sensible defaults for kind priorities.
func DefaultKindPriorities() []KindPriority {
	return []KindPriority{
		{Kind: KindPinned, MinTokens: 500, MaxTokens: 2000, Truncatable: false},
		{Kind: KindReference, MinTokens: 1000, MaxTokens: 10000, Truncatable: true},
		{Kind: KindMemory, MinTokens: 500, MaxTokens: 5000, Truncatable: true},
		{Kind: KindState, MinTokens: 200, MaxTokens: 2000, Truncatable: false},
		{Kind: KindToolOutput, MinTokens: 1000, MaxTokens: 20000, Truncatable: true},
		{Kind: KindHistory, MinTokens: 2000, MaxTokens: 50000, Truncatable: true},
		{Kind: KindTurn, MinTokens: 500, MaxTokens: 10000, Truncatable: false},
	}
}

// DefaultCompactionConfig returns sensible defaults for compaction.
func DefaultCompactionConfig() *CompactionConfig {
	return &CompactionConfig{
		PruneToolOutputs:      true,
		MaxToolOutputAge:      3600, // 1 hour
		MaxToolOutputsPerKind: 10,
		SummarizeHistory:      true,
		MaxHistoryMessages:    20,
	}
}

// DefaultPolicy returns a sensible default policy for Anthropic Claude 3.5 Sonnet.
func DefaultPolicy() ContextPolicy {
	return ContextPolicy{
		ContextWindow:     180000, // Claude 3.5 Sonnet context window
		CompletionReserve: 8000,
		OverflowStrategy:  OverflowCompact,
		KindPriorities:    DefaultKindPriorities(),
		Compaction:        DefaultCompactionConfig(),
		MaxSensitivity:    SensitivityPublic,
		RedactRestricted:  true,
	}
}
