package hooks

import (
	"context"
	"fmt"
	"sync"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/pkg/bichat/hooks/events"
)

// ModelPricing provides pricing information for LLM models.
// Implementations return per-million token pricing for input, output, and cache operations.
type ModelPricing interface {
	// GetPrice returns the price per 1M tokens for input and output.
	// Returns an error if pricing is not available for the model.
	GetPrice(model string) (inputPer1M, outputPer1M float64, err error)

	// GetCachePrice returns the price per 1M tokens for cache write and read operations.
	// Returns an error if pricing is not available for the model.
	// For models without cache pricing, both values should be 0.
	GetCachePrice(model string) (cacheWritePer1M, cacheReadPer1M float64, err error)
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
	CacheWriteTokens int     // Total cache write tokens
	CacheReadTokens  int     // Total cache read tokens
	RequestCount     int     // Number of LLM requests
}

// SessionCost tracks cumulative costs for a session.
type SessionCost struct {
	SessionID        uuid.UUID
	TenantID         uuid.UUID
	TotalCost        float64
	PromptTokens     int
	CompletionTokens int
	CacheWriteTokens int
	CacheReadTokens  int
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
	tenantCost.CacheWriteTokens += e.CacheWriteTokens
	tenantCost.CacheReadTokens += e.CacheReadTokens
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
	sessionCost.CacheWriteTokens += e.CacheWriteTokens
	sessionCost.CacheReadTokens += e.CacheReadTokens
	sessionCost.RequestCount++

	return nil
}

// calculateCost computes the cost of an LLM response based on token usage and pricing.
// Supports cache tokens (CacheWriteTokens and CacheReadTokens) if provided.
func (t *CostTracker) calculateCost(e *events.LLMResponseEvent) (float64, error) {
	// Get pricing for the model
	inputPer1M, outputPer1M, err := t.pricing.GetPrice(e.Model)
	if err != nil {
		return 0, err
	}

	// Calculate basic cost
	inputCost := (float64(e.PromptTokens) / 1_000_000) * inputPer1M
	outputCost := (float64(e.CompletionTokens) / 1_000_000) * outputPer1M

	// Calculate cache costs if cache tokens are present
	var cacheWriteCost, cacheReadCost float64
	if e.CacheWriteTokens > 0 || e.CacheReadTokens > 0 {
		cacheWritePer1M, cacheReadPer1M, err := t.pricing.GetCachePrice(e.Model)
		if err == nil {
			// Only calculate if pricing is available
			cacheWriteCost = (float64(e.CacheWriteTokens) / 1_000_000) * cacheWritePer1M
			cacheReadCost = (float64(e.CacheReadTokens) / 1_000_000) * cacheReadPer1M
		}
		// If GetCachePrice returns error, cache costs remain 0 (silently skip)
	}

	return inputCost + outputCost + cacheWriteCost + cacheReadCost, nil
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
		CacheWriteTokens: cost.CacheWriteTokens,
		CacheReadTokens:  cost.CacheReadTokens,
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
		CacheWriteTokens: cost.CacheWriteTokens,
		CacheReadTokens:  cost.CacheReadTokens,
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
			CacheWriteTokens: cost.CacheWriteTokens,
			CacheReadTokens:  cost.CacheReadTokens,
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
	inputPer1M      float64
	outputPer1M     float64
	cacheWritePer1M float64
	cacheReadPer1M  float64
}

// NewStaticModelPricing creates a StaticModelPricing with no prices.
// Use AddPrice to register model pricing.
func NewStaticModelPricing() *StaticModelPricing {
	return &StaticModelPricing{
		prices: make(map[string]modelPrice),
	}
}

// AddPrice registers pricing for a model without cache pricing.
func (p *StaticModelPricing) AddPrice(model string, inputPer1M, outputPer1M float64) {
	p.prices[model] = modelPrice{
		inputPer1M:      inputPer1M,
		outputPer1M:     outputPer1M,
		cacheWritePer1M: 0,
		cacheReadPer1M:  0,
	}
}

// AddPriceWithCache registers pricing for a model including cache pricing.
func (p *StaticModelPricing) AddPriceWithCache(model string, inputPer1M, outputPer1M, cacheWritePer1M, cacheReadPer1M float64) {
	p.prices[model] = modelPrice{
		inputPer1M:      inputPer1M,
		outputPer1M:     outputPer1M,
		cacheWritePer1M: cacheWritePer1M,
		cacheReadPer1M:  cacheReadPer1M,
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

// GetCachePrice implements ModelPricing.
func (p *StaticModelPricing) GetCachePrice(model string) (float64, float64, error) {
	price, exists := p.prices[model]
	if !exists {
		return 0, 0, fmt.Errorf("pricing not available for model: %s", model)
	}
	return price.cacheWritePer1M, price.cacheReadPer1M, nil
}
