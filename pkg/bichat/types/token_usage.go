package types

// TokenUsage represents token consumption metrics for LLM operations.
type TokenUsage struct {
	// PromptTokens is the number of tokens in the prompt
	PromptTokens int

	// CompletionTokens is the number of tokens in the completion
	CompletionTokens int

	// TotalTokens is the total number of tokens (prompt + completion + cache)
	TotalTokens int

	// CachedTokens is the number of tokens served from cache (legacy)
	// Deprecated: Use CacheReadTokens instead for accurate tracking
	CachedTokens int

	// CacheWriteTokens is the number of tokens written to the cache
	// (prompt caching). Non-zero when model supports prompt caching.
	CacheWriteTokens int

	// CacheReadTokens is the number of tokens read from the cache
	// (prompt caching). Non-zero when cached content is reused.
	CacheReadTokens int
}

// RecalculateTotal updates TotalTokens to include all token types.
// Call this after setting individual token counts.
func (u *TokenUsage) RecalculateTotal() {
	u.TotalTokens = u.PromptTokens + u.CompletionTokens + u.CacheWriteTokens + u.CacheReadTokens
}
