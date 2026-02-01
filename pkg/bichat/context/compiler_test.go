package context_test

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/iota-uz/iota-sdk/pkg/bichat/context"
	"github.com/iota-uz/iota-sdk/pkg/bichat/context/codecs"
)

// mockRenderer is a simple test renderer that estimates tokens based on string length
type mockRenderer struct {
	tokensPerChar float64
}

func newMockRenderer() *mockRenderer {
	return &mockRenderer{
		tokensPerChar: 0.25, // 1 token per 4 characters
	}
}

func (r *mockRenderer) Render(block context.ContextBlock) (context.RenderedBlock, error) {
	switch block.Meta.Kind {
	case context.KindPinned:
		// System blocks go in SystemContent
		content, ok := block.Payload.(string)
		if !ok {
			return context.RenderedBlock{}, errors.New("invalid system payload")
		}
		return context.RenderedBlock{
			SystemContent: content,
		}, nil

	default:
		// Other blocks go in Message
		content, ok := block.Payload.(string)
		if !ok {
			// Try to extract from structured payload
			content = "rendered content"
		}
		return context.RenderedBlock{
			Message: map[string]interface{}{
				"role":    "user",
				"content": content,
			},
		}, nil
	}
}

func (r *mockRenderer) EstimateTokens(block context.ContextBlock) (int, error) {
	// Simple estimation based on payload string length
	var text string

	switch payload := block.Payload.(type) {
	case string:
		text = payload
	case codecs.SystemRulesPayload:
		text = payload.Text
	case codecs.ConversationHistoryPayload:
		for _, msg := range payload.Messages {
			text += msg.Content + " "
		}
	case codecs.DatabaseSchemaPayload:
		// Rough estimate for schema size
		text = strings.Repeat("x", len(payload.Tables)*50)
	default:
		text = "default"
	}

	return int(float64(len(text)) * r.tokensPerChar), nil
}

func (r *mockRenderer) Provider() string {
	return "mock"
}

func TestCompiler_TokenBudget(t *testing.T) {
	t.Parallel()

	builder := context.NewBuilder()
	systemCodec := codecs.NewSystemRulesCodec()
	renderer := newMockRenderer()

	// Add a system block (approx 25 tokens: "System prompt" = 13 chars * 0.25)
	builder.System(systemCodec, "System prompt")

	// Policy with small budget
	policy := context.ContextPolicy{
		ContextWindow:     100,
		CompletionReserve: 20,
		OverflowStrategy:  context.OverflowError,
		KindPriorities:    context.DefaultKindPriorities(),
		MaxSensitivity:    context.SensitivityPublic,
	}

	// Compile should succeed (within budget)
	compiled, err := builder.Compile(renderer, policy)
	if err != nil {
		t.Fatalf("Expected compilation to succeed, got error: %v", err)
	}

	// Verify token count
	if compiled.TotalTokens <= 0 {
		t.Error("Expected positive token count")
	}

	// Verify budget was respected
	availableTokens := policy.ContextWindow - policy.CompletionReserve
	if compiled.TotalTokens > availableTokens {
		t.Errorf("Token count (%d) exceeds available budget (%d)", compiled.TotalTokens, availableTokens)
	}
}

func TestCompiler_Overflow_Error(t *testing.T) {
	t.Parallel()

	builder := context.NewBuilder()
	systemCodec := codecs.NewSystemRulesCodec()
	renderer := newMockRenderer()

	// Add a large block that exceeds budget
	largeContent := strings.Repeat("a", 1000) // ~250 tokens
	builder.System(systemCodec, largeContent)

	// Policy with very small budget
	policy := context.ContextPolicy{
		ContextWindow:     100, // Total window
		CompletionReserve: 50,  // Reserve for completion
		OverflowStrategy:  context.OverflowError,
		KindPriorities:    context.DefaultKindPriorities(),
		MaxSensitivity:    context.SensitivityPublic,
	}

	// Compilation should fail with overflow error
	_, err := builder.Compile(renderer, policy)
	if err == nil {
		t.Fatal("Expected overflow error, got nil")
	}

	if !strings.Contains(err.Error(), "overflow") {
		t.Errorf("Expected 'overflow' in error message, got: %v", err)
	}
}

func TestCompiler_Overflow_Truncate(t *testing.T) {
	t.Parallel()

	builder := context.NewBuilder()
	systemCodec := codecs.NewSystemRulesCodec()
	historyCodec := codecs.NewConversationHistoryCodec()
	renderer := newMockRenderer()

	// Add non-truncatable system block
	builder.System(systemCodec, "System rules")

	// Add truncatable history blocks with unique large content to ensure overflow
	// Each block must have unique content to avoid deduplication in content-addressed graph
	for i := 0; i < 20; i++ {
		builder.History(historyCodec, codecs.ConversationHistoryPayload{
			Messages: []codecs.ConversationMessage{
				{Role: "user", Content: fmt.Sprintf("Message %d: %s", i, strings.Repeat("word ", 50))},
			},
		})
	}

	// Policy with truncation
	policy := context.ContextPolicy{
		ContextWindow:     200,
		CompletionReserve: 50,
		OverflowStrategy:  context.OverflowTruncate,
		KindPriorities:    context.DefaultKindPriorities(),
		MaxSensitivity:    context.SensitivityPublic,
	}

	// Compilation should succeed with truncation
	compiled, err := builder.Compile(renderer, policy)
	if err != nil {
		t.Fatalf("Expected compilation to succeed with truncation, got error: %v", err)
	}

	// Verify truncation occurred
	if !compiled.Truncated {
		t.Errorf("Expected Truncated flag to be true (total tokens: %d, available: %d)",
			compiled.TotalTokens, policy.ContextWindow-policy.CompletionReserve)
	}

	// Verify token budget was respected
	availableTokens := policy.ContextWindow - policy.CompletionReserve
	if compiled.TotalTokens > availableTokens {
		t.Errorf("Token count (%d) exceeds available budget (%d)", compiled.TotalTokens, availableTokens)
	}

	// System block should be preserved
	if compiled.SystemPrompt == "" {
		t.Error("Expected system prompt to be preserved")
	}
}

func TestCompiler_EmptyContext(t *testing.T) {
	t.Parallel()

	builder := context.NewBuilder()
	renderer := newMockRenderer()

	policy := context.ContextPolicy{
		ContextWindow:     1000,
		CompletionReserve: 100,
		OverflowStrategy:  context.OverflowError,
		KindPriorities:    context.DefaultKindPriorities(),
		MaxSensitivity:    context.SensitivityPublic,
	}

	// Compile empty context
	compiled, err := builder.Compile(renderer, policy)
	if err != nil {
		t.Fatalf("Expected compilation to succeed for empty context, got error: %v", err)
	}

	// Verify empty results
	if compiled.SystemPrompt != "" {
		t.Error("Expected empty system prompt")
	}
	if len(compiled.Messages) != 0 {
		t.Errorf("Expected 0 messages, got %d", len(compiled.Messages))
	}
	if compiled.TotalTokens != 0 {
		t.Errorf("Expected 0 tokens, got %d", compiled.TotalTokens)
	}
}

func TestCompiler_SensitivityFiltering(t *testing.T) {
	t.Parallel()

	builder := context.NewBuilder()
	systemCodec := codecs.NewSystemRulesCodec()
	renderer := newMockRenderer()

	// Add blocks with different sensitivity levels
	builder.System(systemCodec, "Public content", context.BlockOptions{
		Sensitivity: context.SensitivityPublic,
	})
	builder.System(systemCodec, "Internal content", context.BlockOptions{
		Sensitivity: context.SensitivityInternal,
	})
	builder.System(systemCodec, "Restricted content", context.BlockOptions{
		Sensitivity: context.SensitivityRestricted,
	})

	tests := []struct {
		name              string
		maxSensitivity    context.SensitivityLevel
		expectedExcluded  int
		shouldContainText string
		shouldNotContain  string
	}{
		{
			name:              "Public only",
			maxSensitivity:    context.SensitivityPublic,
			expectedExcluded:  2, // Exclude Internal and Restricted
			shouldContainText: "Public",
			shouldNotContain:  "Internal",
		},
		{
			name:              "Public and Internal",
			maxSensitivity:    context.SensitivityInternal,
			expectedExcluded:  1, // Exclude Restricted only
			shouldContainText: "Internal",
			shouldNotContain:  "Restricted",
		},
		{
			name:              "All levels",
			maxSensitivity:    context.SensitivityRestricted,
			expectedExcluded:  0, // Include all
			shouldContainText: "Restricted",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			policy := context.ContextPolicy{
				ContextWindow:     1000,
				CompletionReserve: 100,
				OverflowStrategy:  context.OverflowError,
				KindPriorities:    context.DefaultKindPriorities(),
				MaxSensitivity:    tt.maxSensitivity,
			}

			compiled, err := builder.Compile(renderer, policy)
			if err != nil {
				t.Fatalf("Compilation failed: %v", err)
			}

			if compiled.ExcludedBlocks != tt.expectedExcluded {
				t.Errorf("Expected %d excluded blocks, got %d", tt.expectedExcluded, compiled.ExcludedBlocks)
			}

			if !strings.Contains(compiled.SystemPrompt, tt.shouldContainText) {
				t.Errorf("Expected system prompt to contain '%s'", tt.shouldContainText)
			}

			if tt.shouldNotContain != "" && strings.Contains(compiled.SystemPrompt, tt.shouldNotContain) {
				t.Errorf("Expected system prompt NOT to contain '%s'", tt.shouldNotContain)
			}
		})
	}
}

func TestCompiler_TokensByKind(t *testing.T) {
	t.Parallel()

	builder := context.NewBuilder()
	systemCodec := codecs.NewSystemRulesCodec()
	historyCodec := codecs.NewConversationHistoryCodec()
	renderer := newMockRenderer()

	// Add blocks of different kinds
	builder.System(systemCodec, strings.Repeat("a", 100)) // ~25 tokens

	builder.History(historyCodec, codecs.ConversationHistoryPayload{
		Messages: []codecs.ConversationMessage{
			{Role: "user", Content: strings.Repeat("b", 100)}, // ~25 tokens
		},
	})

	builder.Turn(systemCodec, strings.Repeat("c", 100)) // ~25 tokens

	policy := context.ContextPolicy{
		ContextWindow:     1000,
		CompletionReserve: 100,
		OverflowStrategy:  context.OverflowError,
		KindPriorities:    context.DefaultKindPriorities(),
		MaxSensitivity:    context.SensitivityPublic,
	}

	compiled, err := builder.Compile(renderer, policy)
	if err != nil {
		t.Fatalf("Compilation failed: %v", err)
	}

	// Verify TokensByKind breakdown
	if len(compiled.TokensByKind) == 0 {
		t.Fatal("Expected TokensByKind to be populated")
	}

	// All three kinds should be present
	if _, ok := compiled.TokensByKind[context.KindPinned]; !ok {
		t.Error("Expected Pinned tokens to be tracked")
	}
	if _, ok := compiled.TokensByKind[context.KindHistory]; !ok {
		t.Error("Expected History tokens to be tracked")
	}
	if _, ok := compiled.TokensByKind[context.KindTurn]; !ok {
		t.Error("Expected Turn tokens to be tracked")
	}

	// Sum of TokensByKind should equal TotalTokens
	sum := 0
	for _, tokens := range compiled.TokensByKind {
		sum += tokens
	}
	if sum != compiled.TotalTokens {
		t.Errorf("Sum of TokensByKind (%d) != TotalTokens (%d)", sum, compiled.TotalTokens)
	}
}

func TestCompiler_Metadata(t *testing.T) {
	t.Parallel()

	builder := context.NewBuilder()
	systemCodec := codecs.NewSystemRulesCodec()
	renderer := newMockRenderer()

	builder.System(systemCodec, "Test")

	policy := context.ContextPolicy{
		ContextWindow:     2000,
		CompletionReserve: 500,
		OverflowStrategy:  context.OverflowError,
		KindPriorities:    context.DefaultKindPriorities(),
		MaxSensitivity:    context.SensitivityPublic,
	}

	compiled, err := builder.Compile(renderer, policy)
	if err != nil {
		t.Fatalf("Compilation failed: %v", err)
	}

	// Verify metadata fields
	if compiled.Metadata.ContextWindow != 2000 {
		t.Errorf("Expected ContextWindow 2000, got %d", compiled.Metadata.ContextWindow)
	}
	if compiled.Metadata.CompletionReserve != 500 {
		t.Errorf("Expected CompletionReserve 500, got %d", compiled.Metadata.CompletionReserve)
	}
	if compiled.Metadata.AvailableTokens != 1500 {
		t.Errorf("Expected AvailableTokens 1500, got %d", compiled.Metadata.AvailableTokens)
	}
	if compiled.Metadata.Overflowed {
		t.Error("Expected Overflowed to be false")
	}
	if compiled.Metadata.CompiledAt.IsZero() {
		t.Error("Expected CompiledAt to be set")
	}
}

func TestCompiler_MultipleSystemBlocks(t *testing.T) {
	t.Parallel()

	builder := context.NewBuilder()
	systemCodec := codecs.NewSystemRulesCodec()
	renderer := newMockRenderer()

	// Add multiple system blocks
	builder.
		System(systemCodec, "Rule 1").
		System(systemCodec, "Rule 2").
		System(systemCodec, "Rule 3")

	policy := context.ContextPolicy{
		ContextWindow:     1000,
		CompletionReserve: 100,
		OverflowStrategy:  context.OverflowError,
		KindPriorities:    context.DefaultKindPriorities(),
		MaxSensitivity:    context.SensitivityPublic,
	}

	compiled, err := builder.Compile(renderer, policy)
	if err != nil {
		t.Fatalf("Compilation failed: %v", err)
	}

	// All system blocks should be combined in SystemPrompt
	if !strings.Contains(compiled.SystemPrompt, "Rule 1") {
		t.Error("Expected SystemPrompt to contain 'Rule 1'")
	}
	if !strings.Contains(compiled.SystemPrompt, "Rule 2") {
		t.Error("Expected SystemPrompt to contain 'Rule 2'")
	}
	if !strings.Contains(compiled.SystemPrompt, "Rule 3") {
		t.Error("Expected SystemPrompt to contain 'Rule 3'")
	}
}

func TestCompiler_BlockOrdering(t *testing.T) {
	t.Parallel()

	builder := context.NewBuilder()
	systemCodec := codecs.NewSystemRulesCodec()
	historyCodec := codecs.NewConversationHistoryCodec()
	renderer := newMockRenderer()

	// Add blocks in reverse order
	builder.
		Turn(systemCodec, "Current message").
		History(historyCodec, codecs.ConversationHistoryPayload{
			Messages: []codecs.ConversationMessage{
				{Role: "user", Content: "Past message"},
			},
		}).
		System(systemCodec, "System rules")

	policy := context.ContextPolicy{
		ContextWindow:     1000,
		CompletionReserve: 100,
		OverflowStrategy:  context.OverflowError,
		KindPriorities:    context.DefaultKindPriorities(),
		MaxSensitivity:    context.SensitivityPublic,
	}

	compiled, err := builder.Compile(renderer, policy)
	if err != nil {
		t.Fatalf("Compilation failed: %v", err)
	}

	// System should come first (in SystemPrompt)
	if compiled.SystemPrompt == "" {
		t.Error("Expected SystemPrompt to be populated")
	}

	// History and Turn should be in Messages
	if len(compiled.Messages) < 2 {
		t.Errorf("Expected at least 2 messages, got %d", len(compiled.Messages))
	}
}

func TestCompiler_Overflow_Compact(t *testing.T) {
	t.Parallel()

	// Note: This test verifies that compaction with summarization works end-to-end.
	// For full compaction, a summarizer must be configured in the policy.
	// Without a summarizer, compaction falls back to truncation.

	builder := context.NewBuilder()
	systemCodec := codecs.NewSystemRulesCodec()
	historyCodec := codecs.NewConversationHistoryCodec()
	renderer := newMockRenderer()

	// Add system rules
	builder.System(systemCodec, "You are a helpful assistant.")

	// Add many history blocks to trigger overflow
	for i := 0; i < 20; i++ {
		builder.History(historyCodec, codecs.ConversationHistoryPayload{
			Messages: []codecs.ConversationMessage{
				{Role: "user", Content: fmt.Sprintf("Question %d: %s", i, strings.Repeat("word ", 30))},
				{Role: "assistant", Content: fmt.Sprintf("Answer %d: %s", i, strings.Repeat("response ", 30))},
			},
		})
	}

	// Policy with compaction but NO summarizer configured
	// This should fall back to truncation
	policy := context.ContextPolicy{
		ContextWindow:     500,
		CompletionReserve: 100,
		OverflowStrategy:  context.OverflowCompact,
		KindPriorities:    context.DefaultKindPriorities(),
		Compaction: &context.CompactionConfig{
			SummarizeHistory:   true,
			MaxHistoryMessages: 3,
		},
		Summarizer:       nil, // No summarizer - will fall back to truncation
		MaxSensitivity:   context.SensitivityInternal,
		RedactRestricted: true,
	}

	// Compile - should fall back to truncation
	compiled, err := builder.Compile(renderer, policy)
	if err != nil {
		t.Fatalf("Expected compilation to succeed with fallback, got error: %v", err)
	}

	// Should fall back to truncation when no summarizer is configured
	if !compiled.Truncated {
		t.Errorf("Expected truncation fallback when summarizer is not configured")
	}

	if compiled.Compacted {
		t.Errorf("Should not report compaction when summarizer is not configured")
	}

	// Verify token budget was respected
	availableTokens := policy.ContextWindow - policy.CompletionReserve
	if compiled.TotalTokens > availableTokens {
		t.Errorf("Token count (%d) exceeds available budget (%d)", compiled.TotalTokens, availableTokens)
	}
}
