package testing

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/pkg/subscription"
)

type MockEntitlementService struct {
	mu          sync.RWMutex
	tier        string
	features    map[string]bool
	limits      map[string]int
	counts      map[string]int
	seatLimit   *int
	currentSeat int
}

func NewMockEntitlementService() *MockEntitlementService {
	return &MockEntitlementService{
		tier:     "FREE",
		features: map[string]bool{},
		limits:   map[string]int{},
		counts:   map[string]int{},
	}
}

func (m *MockEntitlementService) SetTier(tier string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.tier = tier
}

func (m *MockEntitlementService) SetFeature(feature string, enabled bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.features[feature] = enabled
}

func (m *MockEntitlementService) SetLimit(entityType string, limit int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.limits[entityType] = limit
}

func (m *MockEntitlementService) SetCurrentCount(entityType string, count int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.counts[entityType] = count
}

func (m *MockEntitlementService) SetSeatLimit(limit *int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.seatLimit = limit
}

func (m *MockEntitlementService) HasFeature(_ context.Context, _ uuid.UUID, feature string) (bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.features[feature], nil
}

func (m *MockEntitlementService) HasFeatures(_ context.Context, _ uuid.UUID, features ...string) (map[string]bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make(map[string]bool, len(features))
	for _, feature := range features {
		out[feature] = m.features[feature]
	}
	return out, nil
}

func (m *MockEntitlementService) GetFeatures(_ context.Context, _ uuid.UUID) ([]string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]string, 0, len(m.features))
	for key, enabled := range m.features {
		if enabled {
			out = append(out, key)
		}
	}
	return out, nil
}

func (m *MockEntitlementService) CheckLimit(_ context.Context, _ uuid.UUID, entityType string) (*subscription.LimitResult, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	limit, ok := m.limits[entityType]
	if !ok {
		limit = -1
	}
	current := m.counts[entityType]
	if limit < 0 {
		return &subscription.LimitResult{Allowed: true, Current: current, Limit: -1}, nil
	}
	return &subscription.LimitResult{
		Allowed:    current < limit,
		Current:    current,
		Limit:      limit,
		Percentage: float64(current) / float64(limit),
	}, nil
}

func (m *MockEntitlementService) GetLimits(_ context.Context, _ uuid.UUID) (map[string]subscription.Limit, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make(map[string]subscription.Limit, len(m.limits))
	for entityType, max := range m.limits {
		current := m.counts[entityType]
		percentage := 0.0
		if max > 0 {
			percentage = float64(current) / float64(max)
		}
		out[entityType] = subscription.Limit{
			EntityType:  entityType,
			Current:     current,
			Max:         max,
			IsUnlimited: max < 0,
			Percentage:  percentage,
		}
	}
	return out, nil
}

func (m *MockEntitlementService) IncrementCount(_ context.Context, _ uuid.UUID, entityType string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.counts[entityType]++
	return nil
}

func (m *MockEntitlementService) DecrementCount(_ context.Context, _ uuid.UUID, entityType string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.counts[entityType] > 0 {
		m.counts[entityType]--
	}
	return nil
}

func (m *MockEntitlementService) SetCount(_ context.Context, _ uuid.UUID, entityType string, count int) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.counts[entityType] = count
	return nil
}

func (m *MockEntitlementService) CheckSeatLimit(_ context.Context, _ uuid.UUID) (*subscription.LimitResult, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.seatLimit == nil {
		return &subscription.LimitResult{Allowed: true, Current: m.currentSeat, Limit: -1}, nil
	}
	return &subscription.LimitResult{
		Allowed:    m.currentSeat < *m.seatLimit,
		Current:    m.currentSeat,
		Limit:      *m.seatLimit,
		Percentage: float64(m.currentSeat) / float64(*m.seatLimit),
	}, nil
}

func (m *MockEntitlementService) AddSeat(_ context.Context, _ uuid.UUID) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.currentSeat++
	return nil
}

func (m *MockEntitlementService) RemoveSeat(_ context.Context, _ uuid.UUID) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.currentSeat > 0 {
		m.currentSeat--
	}
	return nil
}

func (m *MockEntitlementService) GetTier(_ context.Context, _ uuid.UUID) (*subscription.TierInfo, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return &subscription.TierInfo{
		Tier:      m.tier,
		Features:  []string{},
		Limits:    m.limits,
		SeatLimit: m.seatLimit,
	}, nil
}

func (m *MockEntitlementService) GetAllTiers(_ context.Context) ([]subscription.TierDefinition, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return []subscription.TierDefinition{{Tier: m.tier, DisplayName: m.tier}}, nil
}

func (m *MockEntitlementService) IsInGracePeriod(_ context.Context, _ uuid.UUID) (bool, *time.Time, error) {
	return false, nil, nil
}

func (m *MockEntitlementService) StartGracePeriod(_ context.Context, _ uuid.UUID) error {
	return nil
}

func (m *MockEntitlementService) ClearGracePeriod(_ context.Context, _ uuid.UUID) error {
	return nil
}

func (m *MockEntitlementService) RefreshEntitlements(_ context.Context, _ uuid.UUID) error {
	return nil
}

func (m *MockEntitlementService) InvalidateCache(_ context.Context, _ uuid.UUID) error {
	return nil
}
