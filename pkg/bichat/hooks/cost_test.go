package hooks_test

import (
	"context"
	"math"
	"testing"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/pkg/bichat/hooks"
	"github.com/iota-uz/iota-sdk/pkg/bichat/hooks/events"
)

// floatEquals checks if two floats are approximately equal (within epsilon).
func floatEquals(a, b float64) bool {
	const epsilon = 0.000001
	return math.Abs(a-b) < epsilon
}

func TestCostTracker_TrackLLMResponse(t *testing.T) {
	t.Parallel()

	// Create pricing
	pricing := hooks.NewStaticModelPricing()
	pricing.AddPrice("claude-3-5-sonnet", 3.0, 15.0) // $3/$15 per 1M tokens

	// Create tracker
	tracker := hooks.NewCostTracker(pricing)

	// Publish LLM response event
	ctx := context.Background()
	sessionID := uuid.New()
	tenantID := uuid.New()

	event := events.NewLLMResponseEvent(
		sessionID, tenantID,
		"claude-3-5-sonnet", "anthropic",
		1000,   // prompt tokens
		2000,   // completion tokens
		3000,   // total tokens
		1500,   // latency ms
		"stop", // finish reason
		0,      // tool calls
		"",     // response text
	)

	err := tracker.Handle(ctx, event)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Check tenant cost
	tenantCost := tracker.GetTenantCost(tenantID)
	if tenantCost == nil {
		t.Fatal("Expected tenant cost to be tracked")
	}

	// Expected: (1000/1M * $3) + (2000/1M * $15) = $0.003 + $0.030 = $0.033
	expectedCost := 0.033
	if !floatEquals(tenantCost.TotalCost, expectedCost) {
		t.Errorf("Expected cost $%.6f, got $%.6f", expectedCost, tenantCost.TotalCost)
	}

	if tenantCost.PromptTokens != 1000 {
		t.Errorf("Expected 1000 prompt tokens, got %d", tenantCost.PromptTokens)
	}

	if tenantCost.CompletionTokens != 2000 {
		t.Errorf("Expected 2000 completion tokens, got %d", tenantCost.CompletionTokens)
	}

	if tenantCost.RequestCount != 1 {
		t.Errorf("Expected 1 request, got %d", tenantCost.RequestCount)
	}

	// Check session cost
	sessionCost := tracker.GetSessionCost(sessionID)
	if sessionCost == nil {
		t.Fatal("Expected session cost to be tracked")
	}

	if !floatEquals(sessionCost.TotalCost, expectedCost) {
		t.Errorf("Expected session cost $%.6f, got $%.6f", expectedCost, sessionCost.TotalCost)
	}
}

func TestCostTracker_MultipleSessions(t *testing.T) {
	t.Parallel()

	// Create pricing
	pricing := hooks.NewStaticModelPricing()
	pricing.AddPrice("gpt-4", 10.0, 30.0) // $10/$30 per 1M tokens

	// Create tracker
	tracker := hooks.NewCostTracker(pricing)
	ctx := context.Background()

	tenantID := uuid.New()
	session1 := uuid.New()
	session2 := uuid.New()

	// Session 1: 500 prompt + 1000 completion
	event1 := events.NewLLMResponseEvent(session1, tenantID, "gpt-4", "openai", 500, 1000, 1500, 1000, "stop", 0, "")
	if err := tracker.Handle(ctx, event1); err != nil {
		t.Fatalf("Failed to handle event1: %v", err)
	}

	// Session 2: 300 prompt + 600 completion
	event2 := events.NewLLMResponseEvent(session2, tenantID, "gpt-4", "openai", 300, 600, 900, 800, "stop", 0, "")
	if err := tracker.Handle(ctx, event2); err != nil {
		t.Fatalf("Failed to handle event2: %v", err)
	}

	// Check tenant cost (sum of both sessions)
	tenantCost := tracker.GetTenantCost(tenantID)
	if tenantCost == nil {
		t.Fatal("Expected tenant cost to be tracked")
	}

	// Expected: session1 + session2
	// Session 1: (500/1M * $10) + (1000/1M * $30) = $0.005 + $0.030 = $0.035
	// Session 2: (300/1M * $10) + (600/1M * $30) = $0.003 + $0.018 = $0.021
	// Total: $0.056
	expectedCost := 0.056
	if !floatEquals(tenantCost.TotalCost, expectedCost) {
		t.Errorf("Expected total cost $%.6f, got $%.6f", expectedCost, tenantCost.TotalCost)
	}

	if tenantCost.PromptTokens != 800 {
		t.Errorf("Expected 800 total prompt tokens, got %d", tenantCost.PromptTokens)
	}

	if tenantCost.CompletionTokens != 1600 {
		t.Errorf("Expected 1600 total completion tokens, got %d", tenantCost.CompletionTokens)
	}

	if tenantCost.RequestCount != 2 {
		t.Errorf("Expected 2 requests, got %d", tenantCost.RequestCount)
	}

	// Check individual session costs
	session1Cost := tracker.GetSessionCost(session1)
	if !floatEquals(session1Cost.TotalCost, 0.035) {
		t.Errorf("Expected session1 cost $0.035, got $%.6f", session1Cost.TotalCost)
	}

	session2Cost := tracker.GetSessionCost(session2)
	if !floatEquals(session2Cost.TotalCost, 0.021) {
		t.Errorf("Expected session2 cost $0.021, got $%.6f", session2Cost.TotalCost)
	}
}

func TestCostTracker_UnknownModel(t *testing.T) {
	t.Parallel()

	// Create pricing with limited models
	pricing := hooks.NewStaticModelPricing()
	pricing.AddPrice("claude-3-5-sonnet", 3.0, 15.0)

	// Create tracker
	tracker := hooks.NewCostTracker(pricing)
	ctx := context.Background()

	sessionID := uuid.New()
	tenantID := uuid.New()

	// Publish event for unknown model
	event := events.NewLLMResponseEvent(sessionID, tenantID, "unknown-model", "provider", 1000, 2000, 3000, 1000, "stop", 0, "")
	err := tracker.Handle(ctx, event)
	if err != nil {
		t.Fatalf("Expected no error (should silently skip), got: %v", err)
	}

	// Cost should not be tracked for unknown model
	tenantCost := tracker.GetTenantCost(tenantID)
	if tenantCost != nil {
		t.Errorf("Expected no cost tracking for unknown model, got cost: $%.6f", tenantCost.TotalCost)
	}
}

func TestCostTracker_Reset(t *testing.T) {
	t.Parallel()

	pricing := hooks.NewStaticModelPricing()
	pricing.AddPrice("claude-3-5-sonnet", 3.0, 15.0)

	tracker := hooks.NewCostTracker(pricing)
	ctx := context.Background()

	tenantID := uuid.New()
	sessionID := uuid.New()

	// Track some cost
	event := events.NewLLMResponseEvent(sessionID, tenantID, "claude-3-5-sonnet", "anthropic", 1000, 2000, 3000, 1000, "stop", 0, "")
	if err := tracker.Handle(ctx, event); err != nil {
		t.Fatalf("Failed to handle event: %v", err)
	}

	// Verify cost is tracked
	if tracker.GetTenantCost(tenantID) == nil {
		t.Fatal("Expected cost to be tracked before reset")
	}

	// Reset
	tracker.Reset()

	// Verify cost is cleared
	if tracker.GetTenantCost(tenantID) != nil {
		t.Error("Expected cost to be cleared after reset")
	}
}

func TestCostTracker_CacheTokens(t *testing.T) {
	t.Parallel()

	// Create pricing with cache support
	pricing := hooks.NewStaticModelPricing()
	// OpenAI GPT-4o pricing with cache
	// Input: $2.50/1M, Output: $10/1M, Cache Write: $1.25/1M, Cache Read: $0.625/1M
	pricing.AddPriceWithCache("gpt-4o", 2.50, 10.0, 1.25, 0.625)

	// Create tracker
	tracker := hooks.NewCostTracker(pricing)
	ctx := context.Background()

	tenantID := uuid.New()
	sessionID := uuid.New()

	// Create event with cache tokens
	// 1000 prompt tokens, 500 completion tokens, 0 cache write, 2000 cache read
	event := events.NewLLMResponseEventWithCache(
		sessionID, tenantID,
		"gpt-4o", "openai",
		1000, // prompt tokens
		500,  // completion tokens
		3500, // total tokens
		0,    // cache write tokens
		2000, // cache read tokens (50% discount applies)
		1000, // latency ms
		"stop",
		0,  // tool calls
		"", // response text
	)

	err := tracker.Handle(ctx, event)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Check tenant cost
	tenantCost := tracker.GetTenantCost(tenantID)
	if tenantCost == nil {
		t.Fatal("Expected tenant cost to be tracked")
	}

	// Expected cost calculation:
	// Prompt: (1000 / 1M) * $2.50 = $0.0025
	// Completion: (500 / 1M) * $10.00 = $0.0050
	// Cache Write: (0 / 1M) * $1.25 = $0.0000
	// Cache Read: (2000 / 1M) * $0.625 = $0.00125
	// Total: $0.00875
	expectedCost := 0.00875
	if !floatEquals(tenantCost.TotalCost, expectedCost) {
		t.Errorf("Expected cost $%.6f, got $%.6f", expectedCost, tenantCost.TotalCost)
	}

	// Verify token tracking
	if tenantCost.PromptTokens != 1000 {
		t.Errorf("Expected 1000 prompt tokens, got %d", tenantCost.PromptTokens)
	}

	if tenantCost.CompletionTokens != 500 {
		t.Errorf("Expected 500 completion tokens, got %d", tenantCost.CompletionTokens)
	}

	if tenantCost.CacheWriteTokens != 0 {
		t.Errorf("Expected 0 cache write tokens, got %d", tenantCost.CacheWriteTokens)
	}

	if tenantCost.CacheReadTokens != 2000 {
		t.Errorf("Expected 2000 cache read tokens, got %d", tenantCost.CacheReadTokens)
	}

	if tenantCost.RequestCount != 1 {
		t.Errorf("Expected 1 request, got %d", tenantCost.RequestCount)
	}
}

func TestCostTracker_CacheTokensWithWrite(t *testing.T) {
	t.Parallel()

	// Create pricing with cache support
	pricing := hooks.NewStaticModelPricing()
	// OpenAI GPT-4o mini pricing
	pricing.AddPriceWithCache("gpt-4o-mini", 0.150, 0.600, 0.075, 0.038)

	tracker := hooks.NewCostTracker(pricing)
	ctx := context.Background()

	tenantID := uuid.New()
	sessionID := uuid.New()

	// Create event with both cache write and read
	event := events.NewLLMResponseEventWithCache(
		sessionID, tenantID,
		"gpt-4o-mini", "openai",
		1000, // prompt tokens
		500,  // completion tokens
		4500, // total tokens
		2000, // cache write tokens
		1000, // cache read tokens
		800,  // latency ms
		"stop",
		0,
		"", // response text
	)

	err := tracker.Handle(ctx, event)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	tenantCost := tracker.GetTenantCost(tenantID)
	if tenantCost == nil {
		t.Fatal("Expected tenant cost to be tracked")
	}

	// Expected cost:
	// Prompt: (1000/1M) * $0.150 = $0.00015
	// Completion: (500/1M) * $0.600 = $0.00030
	// Cache Write: (2000/1M) * $0.075 = $0.00015
	// Cache Read: (1000/1M) * $0.038 = $0.000038
	// Total: $0.000638
	expectedCost := 0.000638
	if !floatEquals(tenantCost.TotalCost, expectedCost) {
		t.Errorf("Expected cost $%.6f, got $%.6f", expectedCost, tenantCost.TotalCost)
	}

	if tenantCost.CacheWriteTokens != 2000 {
		t.Errorf("Expected 2000 cache write tokens, got %d", tenantCost.CacheWriteTokens)
	}

	if tenantCost.CacheReadTokens != 1000 {
		t.Errorf("Expected 1000 cache read tokens, got %d", tenantCost.CacheReadTokens)
	}
}

func TestCostTracker_ModelWithoutCachePricing(t *testing.T) {
	t.Parallel()

	// Create pricing without cache support (e.g., older models)
	pricing := hooks.NewStaticModelPricing()
	pricing.AddPrice("gpt-3.5-turbo", 0.50, 1.50)

	tracker := hooks.NewCostTracker(pricing)
	ctx := context.Background()

	tenantID := uuid.New()
	sessionID := uuid.New()

	// Create event with cache tokens (but model doesn't support caching)
	event := events.NewLLMResponseEventWithCache(
		sessionID, tenantID,
		"gpt-3.5-turbo", "openai",
		1000, // prompt tokens
		500,  // completion tokens
		1500, // total tokens
		0,    // cache write tokens
		500,  // cache read tokens (should be ignored for cost calculation)
		600,  // latency ms
		"stop",
		0,
		"", // response text
	)

	err := tracker.Handle(ctx, event)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	tenantCost := tracker.GetTenantCost(tenantID)
	if tenantCost == nil {
		t.Fatal("Expected tenant cost to be tracked")
	}

	// Expected cost (cache tokens should be tracked but not priced):
	// Prompt: (1000/1M) * $0.50 = $0.0005
	// Completion: (500/1M) * $1.50 = $0.00075
	// Cache tokens: ignored (pricing is 0)
	// Total: $0.00125
	expectedCost := 0.00125
	if !floatEquals(tenantCost.TotalCost, expectedCost) {
		t.Errorf("Expected cost $%.6f, got $%.6f", expectedCost, tenantCost.TotalCost)
	}

	// Cache tokens should still be tracked
	if tenantCost.CacheReadTokens != 500 {
		t.Errorf("Expected 500 cache read tokens tracked, got %d", tenantCost.CacheReadTokens)
	}
}
