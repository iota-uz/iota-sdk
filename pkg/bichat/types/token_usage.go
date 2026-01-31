package types

// TokenUsage represents token consumption metrics for LLM operations.
type TokenUsage struct {
	// PromptTokens is the number of tokens in the prompt
	PromptTokens int

	// CompletionTokens is the number of tokens in the completion
	CompletionTokens int

	// TotalTokens is the total number of tokens (prompt + completion)
	TotalTokens int

	// CachedTokens is the number of tokens served from cache
	CachedTokens int
}
