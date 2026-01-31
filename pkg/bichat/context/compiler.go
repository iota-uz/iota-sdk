package context

import (
	"fmt"
	"sort"
	"time"
)

// CompiledContext is the result of compiling a context graph with a renderer and policy.
type CompiledContext struct {
	// SystemPrompt is the combined system prompt from all pinned blocks.
	SystemPrompt string

	// Messages are the rendered messages for the provider.
	Messages []any

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
			// TODO: Implement compaction (requires summarizer interface)
			// For now, fall back to truncation
			finalBlocks, totalTokens, tokensByKind = truncateBlocks(sortedBlocks, blockTokens, availableTokens, policy.KindPriorities)
			compacted = true
		}
	} else {
		finalBlocks = sortedBlocks
	}

	// Render blocks
	systemPrompt, messages, err := renderBlocks(finalBlocks, renderer)
	if err != nil {
		return nil, fmt.Errorf("%s: failed to render blocks: %w", op, err)
	}

	return &CompiledContext{
		SystemPrompt:   systemPrompt,
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
			// TODO: Replace payload with redacted stub
			// For now, just exclude
			excludedCount++
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

// renderBlocks renders all blocks using the renderer.
func renderBlocks(blocks []ContextBlock, renderer Renderer) (string, []any, error) {
	var systemPrompt string
	var messages []any

	for _, block := range blocks {
		rendered, err := renderer.Render(block)
		if err != nil {
			return "", nil, fmt.Errorf("failed to render block %s: %w", block.Hash, err)
		}

		// System content goes into system prompt
		if rendered.SystemContent != "" {
			if systemPrompt != "" {
				systemPrompt += "\n\n"
			}
			systemPrompt += rendered.SystemContent
		}

		// Messages go into message array
		if rendered.Message != nil {
			messages = append(messages, rendered.Message)
		}
	}

	return systemPrompt, messages, nil
}
