package context

import (
	stdctx "context"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/iota-uz/iota-sdk/pkg/bichat/types"
)

// CompiledContext is the result of compiling a context graph with a renderer and policy.
type CompiledContext struct {
	// Messages are the canonical rendered messages (no provider-specific formats).
	Messages []types.Message

	// TotalTokens is the total estimated token count.
	TotalTokens int

	// TokensByKind breaks down token usage by block kind.
	TokensByKind map[BlockKind]int

	// Truncated indicates if any blocks were truncated due to overflow.
	Truncated bool

	// Compacted indicates if compaction was applied.
	Compacted bool

	// ExcludedBlocks is the number of blocks excluded due to sensitivity filtering.
	ExcludedBlocks int

	// Metadata contains additional compilation metadata.
	Metadata CompilationMetadata
}

// CompilationMetadata contains metadata about the compilation process.
type CompilationMetadata struct {
	// CompiledAt is when the context was compiled.
	CompiledAt time.Time

	// ContextWindow is the total context window size.
	ContextWindow int

	// CompletionReserve is the number of tokens reserved for completion.
	CompletionReserve int

	// AvailableTokens is the total tokens available for context.
	AvailableTokens int

	// Overflowed indicates if the context exceeded the token budget.
	Overflowed bool
}

// Compile compiles the context using the given renderer and policy.
func (b *ContextBuilder) Compile(renderer Renderer, policy ContextPolicy) (*CompiledContext, error) {
	const op = "ContextBuilder.Compile"

	// Get all blocks from graph
	allBlocks := b.graph.GetAllBlocks()

	// Filter by sensitivity
	filteredBlocks, excludedCount := filterBySensitivity(allBlocks, policy.MaxSensitivity, policy.RedactRestricted)

	// Sort blocks by kind order, then by hash
	sortedBlocks := sortBlocks(filteredBlocks)

	// Estimate tokens for each block
	blockTokens := make(map[string]int)
	tokensByKind := make(map[BlockKind]int)

	for _, block := range sortedBlocks {
		tokens, err := renderer.EstimateTokens(block)
		if err != nil {
			return nil, fmt.Errorf("%s: failed to estimate tokens for block %s: %w", op, block.Hash, err)
		}

		blockTokens[block.Hash] = tokens
		tokensByKind[block.Meta.Kind] += tokens
	}

	// Calculate available budget
	availableTokens := policy.ContextWindow - policy.CompletionReserve
	totalTokens := 0
	for _, tokens := range blockTokens {
		totalTokens += tokens
	}

	// Handle overflow
	var truncated, compacted bool
	var finalBlocks []ContextBlock

	if totalTokens > availableTokens {
		switch policy.OverflowStrategy {
		case OverflowError:
			return nil, fmt.Errorf("%s: context overflow (%d tokens exceeds budget of %d)", op, totalTokens, availableTokens)

		case OverflowTruncate:
			finalBlocks, totalTokens, tokensByKind = truncateBlocks(sortedBlocks, blockTokens, availableTokens, policy.KindPriorities)
			truncated = true

		case OverflowCompact:
			// Try intelligent compaction with summarization if configured
			if policy.Summarizer != nil && policy.Compaction != nil && policy.Compaction.SummarizeHistory {
				var compactErr error
				finalBlocks, totalTokens, tokensByKind, compactErr = compactBlocks(stdctx.Background(), sortedBlocks, blockTokens, availableTokens, policy, renderer)
				if compactErr != nil {
					return nil, fmt.Errorf("%s: compaction failed: %w", op, compactErr)
				}
				compacted = true
			} else {
				// Fall back to truncation if summarizer not configured
				finalBlocks, totalTokens, tokensByKind = truncateBlocks(sortedBlocks, blockTokens, availableTokens, policy.KindPriorities)
				truncated = true
			}
		}
	} else {
		finalBlocks = sortedBlocks
	}

	// Render blocks
	messages, err := renderBlocks(finalBlocks, renderer)
	if err != nil {
		return nil, fmt.Errorf("%s: failed to render blocks: %w", op, err)
	}

	return &CompiledContext{
		Messages:       messages,
		TotalTokens:    totalTokens,
		TokensByKind:   tokensByKind,
		Truncated:      truncated,
		Compacted:      compacted,
		ExcludedBlocks: excludedCount,
		Metadata: CompilationMetadata{
			CompiledAt:        time.Now(),
			ContextWindow:     policy.ContextWindow,
			CompletionReserve: policy.CompletionReserve,
			AvailableTokens:   availableTokens,
			Overflowed:        totalTokens > availableTokens,
		},
	}, nil
}

// filterBySensitivity filters blocks by sensitivity level.
func filterBySensitivity(blocks []ContextBlock, maxSensitivity SensitivityLevel, redactRestricted bool) ([]ContextBlock, int) {
	sensitivityOrder := map[SensitivityLevel]int{
		SensitivityPublic:     0,
		SensitivityInternal:   1,
		SensitivityRestricted: 2,
	}

	maxLevel := sensitivityOrder[maxSensitivity]
	var filtered []ContextBlock
	excludedCount := 0

	for _, block := range blocks {
		blockLevel := sensitivityOrder[block.Meta.Sensitivity]

		if blockLevel <= maxLevel {
			filtered = append(filtered, block)
		} else if redactRestricted && block.Meta.Sensitivity == SensitivityRestricted {
			// Replace with redacted stub (preserve structure)
			redactedBlock := ContextBlock{
				Hash:    block.Hash,
				Meta:    block.Meta,
				Payload: "[REDACTED - Restricted Content]",
			}
			filtered = append(filtered, redactedBlock)
			// Don't count as excluded since we include a stub
		} else {
			excludedCount++
		}
	}

	return filtered, excludedCount
}

// sortBlocks sorts blocks by kind order, then by hash.
func sortBlocks(blocks []ContextBlock) []ContextBlock {
	sorted := make([]ContextBlock, len(blocks))
	copy(sorted, blocks)

	sort.Slice(sorted, func(i, j int) bool {
		kindI := kindOrderIndex(sorted[i].Meta.Kind)
		kindJ := kindOrderIndex(sorted[j].Meta.Kind)

		if kindI != kindJ {
			return kindI < kindJ
		}

		return sorted[i].Hash < sorted[j].Hash
	})

	return sorted
}

// truncateBlocks removes blocks until the token budget is met.
func truncateBlocks(
	blocks []ContextBlock,
	blockTokens map[string]int,
	availableTokens int,
	kindPriorities []KindPriority,
) ([]ContextBlock, int, map[BlockKind]int) {
	// Build priority map
	priorityMap := make(map[BlockKind]KindPriority)
	for _, p := range kindPriorities {
		priorityMap[p.Kind] = p
	}

	// Start from the end and remove truncatable blocks
	totalTokens := 0
	for _, tokens := range blockTokens {
		totalTokens += tokens
	}

	var finalBlocks []ContextBlock
	newTokensByKind := make(map[BlockKind]int)

	for _, block := range blocks {
		tokens := blockTokens[block.Hash]
		priority, hasPriority := priorityMap[block.Meta.Kind]

		// Always include non-truncatable blocks
		if !hasPriority || !priority.Truncatable {
			finalBlocks = append(finalBlocks, block)
			newTokensByKind[block.Meta.Kind] += tokens
			continue
		}

		// Check if we have budget for this block
		currentTotal := 0
		for _, t := range newTokensByKind {
			currentTotal += t
		}

		if currentTotal+tokens <= availableTokens {
			finalBlocks = append(finalBlocks, block)
			newTokensByKind[block.Meta.Kind] += tokens
		}
	}

	// Recalculate total
	totalTokens = 0
	for _, tokens := range newTokensByKind {
		totalTokens += tokens
	}

	return finalBlocks, totalTokens, newTokensByKind
}

// compactBlocks intelligently reduces context using summarization.
// It summarizes history blocks to save tokens while preserving critical context.
func compactBlocks(
	ctx stdctx.Context,
	blocks []ContextBlock,
	blockTokens map[string]int,
	availableTokens int,
	policy ContextPolicy,
	renderer Renderer,
) ([]ContextBlock, int, map[BlockKind]int, error) {
	// Build priority map
	priorityMap := make(map[BlockKind]KindPriority)
	for _, p := range policy.KindPriorities {
		priorityMap[p.Kind] = p
	}

	// Separate history blocks from others
	var historyBlocks []ContextBlock
	var otherBlocks []ContextBlock
	historyTokens := 0

	for _, block := range blocks {
		if block.Meta.Kind == KindHistory {
			historyBlocks = append(historyBlocks, block)
			historyTokens += blockTokens[block.Hash]
		} else {
			otherBlocks = append(otherBlocks, block)
		}
	}

	// Calculate tokens used by non-history blocks
	otherTokens := 0
	for _, block := range otherBlocks {
		otherTokens += blockTokens[block.Hash]
	}

	// If history is empty or fits within budget, no compaction needed
	if len(historyBlocks) == 0 || otherTokens+historyTokens <= availableTokens {
		finalBlocks, totalTokens, tokensByKind := truncateBlocks(blocks, blockTokens, availableTokens, policy.KindPriorities)
		return finalBlocks, totalTokens, tokensByKind, nil
	}

	// Calculate target tokens for summarized history
	historyBudget := availableTokens - otherTokens
	if historyBudget < 100 {
		// Not enough budget for meaningful summary, just truncate
		finalBlocks, totalTokens, tokensByKind := truncateBlocks(blocks, blockTokens, availableTokens, policy.KindPriorities)
		return finalBlocks, totalTokens, tokensByKind, nil
	}

	// Target 50% of history budget for summary (leave room for compression)
	targetSummaryTokens := historyBudget / 2

	// Extract messages from history blocks for summarization
	historyMessages := extractMessagesFromHistoryBlocks(historyBlocks)

	// If we couldn't extract meaningful messages, fall back to truncation
	if len(historyMessages) == 0 {
		finalBlocks, totalTokens, tokensByKind := truncateBlocks(blocks, blockTokens, availableTokens, policy.KindPriorities)
		return finalBlocks, totalTokens, tokensByKind, nil
	}

	// Check if we have enough messages to warrant summarization
	if policy.Compaction != nil && policy.Compaction.MaxHistoryMessages > 0 {
		if len(historyMessages) < policy.Compaction.MaxHistoryMessages {
			// Not enough messages to summarize, just use truncation
			finalBlocks, totalTokens, tokensByKind := truncateBlocks(blocks, blockTokens, availableTokens, policy.KindPriorities)
			return finalBlocks, totalTokens, tokensByKind, nil
		}
	}

	// Generate summary using configured summarizer
	summary, summaryTokens, err := policy.Summarizer.SummarizeMessages(ctx, historyMessages, targetSummaryTokens)
	if err != nil {
		// Summarization failed, fall back to truncation
		finalBlocks, totalTokens, tokensByKind := truncateBlocks(blocks, blockTokens, availableTokens, policy.KindPriorities)
		return finalBlocks, totalTokens, tokensByKind, err
	}

	// Create a new summarized history block to replace the original history blocks
	summarizedBlock := createSummaryBlock(summary, summaryTokens)

	// Combine non-history blocks with the summary block
	compactedBlocks := append(otherBlocks, summarizedBlock)

	// Update blockTokens map to include summary block
	updatedBlockTokens := make(map[string]int)
	for hash, tokens := range blockTokens {
		updatedBlockTokens[hash] = tokens
	}
	updatedBlockTokens[summarizedBlock.Hash] = summaryTokens

	// Recalculate token totals
	compactedTokens := otherTokens + summaryTokens
	compactedTokensByKind := make(map[BlockKind]int)
	for _, block := range otherBlocks {
		compactedTokensByKind[block.Meta.Kind] += blockTokens[block.Hash]
	}
	compactedTokensByKind[KindHistory] = summaryTokens

	// If still over budget, apply truncation with updated token map
	if compactedTokens > availableTokens {
		finalBlocks, totalTokens, tokensByKind := truncateBlocks(compactedBlocks, updatedBlockTokens, availableTokens, policy.KindPriorities)
		return finalBlocks, totalTokens, tokensByKind, nil
	}

	return compactedBlocks, compactedTokens, compactedTokensByKind, nil
}

// conversationHistoryPayload is a simplified version of codecs.ConversationHistoryPayload
// to avoid circular import dependency.
type conversationHistoryPayload struct {
	Messages []conversationMessage `json:"messages"`
	Summary  string                `json:"summary,omitempty"`
}

type conversationMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// extractMessagesFromHistoryBlocks extracts types.Message from history blocks.
// Converts history block payloads to the standard message format for summarization.
func extractMessagesFromHistoryBlocks(historyBlocks []ContextBlock) []types.Message {
	var messages []types.Message

	for _, block := range historyBlocks {
		// Extract payload as map[string]any or struct
		var payload conversationHistoryPayload

		switch v := block.Payload.(type) {
		case map[string]any:
			// Extract from map format
			if messagesRaw, ok := v["messages"].([]any); ok {
				for _, msgRaw := range messagesRaw {
					if msgMap, ok := msgRaw.(map[string]any); ok {
						role, _ := msgMap["role"].(string)
						content, _ := msgMap["content"].(string)
						payload.Messages = append(payload.Messages, conversationMessage{
							Role:    role,
							Content: content,
						})
					}
				}
			}
			if summary, ok := v["summary"].(string); ok {
				payload.Summary = summary
			}
		default:
			// Try JSON unmarshal as fallback
			if data, err := json.Marshal(block.Payload); err == nil {
				_ = json.Unmarshal(data, &payload)
			}
		}

		// Convert to types.Message
		for _, msg := range payload.Messages {
			role := types.RoleUser // Default fallback
			switch msg.Role {
			case "user":
				role = types.RoleUser
			case "assistant":
				role = types.RoleAssistant
			case "system":
				role = types.RoleSystem
			case "tool":
				role = types.RoleTool
			}

			messages = append(messages, types.NewMessage(
				types.WithRole(role),
				types.WithContent(msg.Content),
			))
		}

		// If there's a summary already, include it as a system message
		if payload.Summary != "" {
			messages = append(messages, types.SystemMessage(fmt.Sprintf("Previous conversation summary: %s", payload.Summary)))
		}
	}

	return messages
}

// createSummaryBlock creates a new history block containing the summarized conversation.
func createSummaryBlock(summary string, summaryTokens int) ContextBlock {
	// Create a conversation history payload with just the summary
	payload := conversationHistoryPayload{
		Summary: summary,
		Messages: []conversationMessage{
			{
				Role:    "system",
				Content: fmt.Sprintf("Conversation Summary (%d tokens):\n\n%s", summaryTokens, summary),
			},
		},
	}

	// Create block metadata
	meta := BlockMeta{
		Kind:         KindHistory,
		Sensitivity:  SensitivityInternal,
		CodecID:      "conversation-history",
		CodecVersion: "1.0.0",
		Timestamp:    time.Now(),
		Tags:         []string{"summarized", "compacted"},
	}

	// Canonicalize payload for hashing
	canonicalized, err := SortedJSONBytes(payload)
	if err != nil {
		// Fallback: use regular JSON encoding
		canonicalized, err = json.Marshal(payload)
		if err != nil {
			canonicalized = []byte("{}")
		}
	}

	// Compute content-addressed hash
	hash := ComputeBlockHash(meta.ToStableSubset(), canonicalized)

	// Create the block
	block := ContextBlock{
		Hash:    hash,
		Meta:    meta,
		Payload: payload,
	}

	return block
}

// renderBlocks renders all blocks using the renderer.
func renderBlocks(blocks []ContextBlock, renderer Renderer) ([]types.Message, error) {
	var messages []types.Message

	for _, block := range blocks {
		rendered, err := renderer.Render(block)
		if err != nil {
			return nil, fmt.Errorf("failed to render block %s: %w", block.Hash, err)
		}

		// Append all rendered messages
		messages = append(messages, rendered.Messages...)
	}

	return messages, nil
}
