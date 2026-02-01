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
	event1 := events.NewLLMResponseEvent(session1, tenantID, "gpt-4", "openai", 500, 1000, 1500, 1000, "stop", 0)
	if err := tracker.Handle(ctx, event1); err != nil {
		t.Fatalf("Failed to handle event1: %v", err)
	}

	// Session 2: 300 prompt + 600 completion
	event2 := events.NewLLMResponseEvent(session2, tenantID, "gpt-4", "openai", 300, 600, 900, 800, "stop", 0)
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
	event := events.NewLLMResponseEvent(sessionID, tenantID, "unknown-model", "provider", 1000, 2000, 3000, 1000, "stop", 0)
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
	event := events.NewLLMResponseEvent(sessionID, tenantID, "claude-3-5-sonnet", "anthropic", 1000, 2000, 3000, 1000, "stop", 0)
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
