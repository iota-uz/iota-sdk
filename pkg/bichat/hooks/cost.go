package hooks

import (
	"context"
	"fmt"
	"sync"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/pkg/bichat/hooks/events"
)

// ModelPricing provides pricing information for LLM models.
// Implementations return per-million token pricing for input and output.
type ModelPricing interface {
	// GetPrice returns the price per 1M tokens for input and output.
	// Returns an error if pricing is not available for the model.
	GetPrice(model string) (inputPer1M, outputPer1M float64, err error)
}

// CostTracker tracks LLM usage costs by listening to LLMResponseEvents.
// It implements EventHandler and can be registered with an EventBus.
type CostTracker struct {
	pricing ModelPricing
	mu      sync.RWMutex
	// Costs aggregated by tenant ID
	costsByTenant map[uuid.UUID]*TenantCost
	// Costs aggregated by session ID
	costsBySession map[uuid.UUID]*SessionCost
}

// TenantCost tracks cumulative costs for a tenant.
type TenantCost struct {
	TenantID         uuid.UUID
	TotalCost        float64 // Total cost in dollars
	PromptTokens     int     // Total prompt tokens
	CompletionTokens int     // Total completion tokens
	RequestCount     int     // Number of LLM requests
}

// SessionCost tracks cumulative costs for a session.
type SessionCost struct {
	SessionID        uuid.UUID
	TenantID         uuid.UUID
	TotalCost        float64
	PromptTokens     int
	CompletionTokens int
	RequestCount     int
}

// NewCostTracker creates a new CostTracker with the given pricing provider.
func NewCostTracker(pricing ModelPricing) *CostTracker {
	return &CostTracker{
		pricing:        pricing,
		costsByTenant:  make(map[uuid.UUID]*TenantCost),
		costsBySession: make(map[uuid.UUID]*SessionCost),
	}
}

// Handle implements EventHandler.
func (t *CostTracker) Handle(ctx context.Context, event Event) error {
	// Only process LLMResponseEvents
	e, ok := event.(*events.LLMResponseEvent)
	if !ok {
		return nil
	}

	// Calculate cost for this response
	cost, err := t.calculateCost(e)
	if err != nil {
		// If pricing is not available, skip cost tracking
		return nil
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	// Update tenant cost
	tenantCost, exists := t.costsByTenant[e.TenantID()]
	if !exists {
		tenantCost = &TenantCost{
			TenantID: e.TenantID(),
		}
		t.costsByTenant[e.TenantID()] = tenantCost
	}
	tenantCost.TotalCost += cost
	tenantCost.PromptTokens += e.PromptTokens
	tenantCost.CompletionTokens += e.CompletionTokens
	tenantCost.RequestCount++

	// Update session cost
	sessionCost, exists := t.costsBySession[e.SessionID()]
	if !exists {
		sessionCost = &SessionCost{
			SessionID: e.SessionID(),
			TenantID:  e.TenantID(),
		}
		t.costsBySession[e.SessionID()] = sessionCost
	}
	sessionCost.TotalCost += cost
	sessionCost.PromptTokens += e.PromptTokens
	sessionCost.CompletionTokens += e.CompletionTokens
	sessionCost.RequestCount++

	return nil
}

// calculateCost computes the cost of an LLM response based on token usage and pricing.
func (t *CostTracker) calculateCost(e *events.LLMResponseEvent) (float64, error) {
	// Get pricing for the model
	inputPer1M, outputPer1M, err := t.pricing.GetPrice(e.Model)
	if err != nil {
		return 0, err
	}

	// Calculate cost
	inputCost := (float64(e.PromptTokens) / 1_000_000) * inputPer1M
	outputCost := (float64(e.CompletionTokens) / 1_000_000) * outputPer1M

	return inputCost + outputCost, nil
}

// GetTenantCost returns the cumulative cost for a tenant.
// Returns nil if no costs have been tracked for this tenant.
func (t *CostTracker) GetTenantCost(tenantID uuid.UUID) *TenantCost {
	t.mu.RLock()
	defer t.mu.RUnlock()

	cost, exists := t.costsByTenant[tenantID]
	if !exists {
		return nil
	}

	// Return a copy to prevent external mutation
	return &TenantCost{
		TenantID:         cost.TenantID,
		TotalCost:        cost.TotalCost,
		PromptTokens:     cost.PromptTokens,
		CompletionTokens: cost.CompletionTokens,
		RequestCount:     cost.RequestCount,
	}
}

// GetSessionCost returns the cumulative cost for a session.
// Returns nil if no costs have been tracked for this session.
func (t *CostTracker) GetSessionCost(sessionID uuid.UUID) *SessionCost {
	t.mu.RLock()
	defer t.mu.RUnlock()

	cost, exists := t.costsBySession[sessionID]
	if !exists {
		return nil
	}

	// Return a copy to prevent external mutation
	return &SessionCost{
		SessionID:        cost.SessionID,
		TenantID:         cost.TenantID,
		TotalCost:        cost.TotalCost,
		PromptTokens:     cost.PromptTokens,
		CompletionTokens: cost.CompletionTokens,
		RequestCount:     cost.RequestCount,
	}
}

// GetAllTenantCosts returns costs for all tenants.
func (t *CostTracker) GetAllTenantCosts() []*TenantCost {
	t.mu.RLock()
	defer t.mu.RUnlock()

	costs := make([]*TenantCost, 0, len(t.costsByTenant))
	for _, cost := range t.costsByTenant {
		costs = append(costs, &TenantCost{
			TenantID:         cost.TenantID,
			TotalCost:        cost.TotalCost,
			PromptTokens:     cost.PromptTokens,
			CompletionTokens: cost.CompletionTokens,
			RequestCount:     cost.RequestCount,
		})
	}

	return costs
}

// Reset clears all tracked costs.
// Useful for testing or periodic reset scenarios.
func (t *CostTracker) Reset() {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.costsByTenant = make(map[uuid.UUID]*TenantCost)
	t.costsBySession = make(map[uuid.UUID]*SessionCost)
}

// StaticModelPricing provides fixed pricing for known models.
// Useful for testing or when pricing doesn't change frequently.
type StaticModelPricing struct {
	prices map[string]modelPrice
}

type modelPrice struct {
	inputPer1M  float64
	outputPer1M float64
}

// NewStaticModelPricing creates a StaticModelPricing with no prices.
// Use AddPrice to register model pricing.
func NewStaticModelPricing() *StaticModelPricing {
	return &StaticModelPricing{
		prices: make(map[string]modelPrice),
	}
}

// AddPrice registers pricing for a model.
func (p *StaticModelPricing) AddPrice(model string, inputPer1M, outputPer1M float64) {
	p.prices[model] = modelPrice{
		inputPer1M:  inputPer1M,
		outputPer1M: outputPer1M,
	}
}

// GetPrice implements ModelPricing.
func (p *StaticModelPricing) GetPrice(model string) (float64, float64, error) {
	price, exists := p.prices[model]
	if !exists {
		return 0, 0, fmt.Errorf("pricing not available for model: %s", model)
	}
	return price.inputPer1M, price.outputPer1M, nil
}
