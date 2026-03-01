package testing

import (
	"context"
	"fmt"
	"sort"
	"sync"

	"github.com/iota-uz/iota-sdk/pkg/subscription"
)

type MockEngine struct {
	mu sync.RWMutex

	plan         string
	features     map[subscription.FeatureKey]bool
	limits       map[string]int
	counts       map[string]int
	usageByKey   map[string]int
	grants       map[string]subscription.Grant
	reservations map[string]subscription.Reservation
	nextResID    int64
}

func NewMockEngine() *MockEngine {
	return &MockEngine{
		plan:         "FREE",
		features:     map[subscription.FeatureKey]bool{},
		limits:       map[string]int{},
		counts:       map[string]int{},
		usageByKey:   map[string]int{},
		grants:       map[string]subscription.Grant{},
		reservations: map[string]subscription.Reservation{},
	}
}

func (m *MockEngine) SetPlan(planID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.plan = planID
}

func (m *MockEngine) SetFeature(feature string, enabled bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.features[subscription.FeatureKey(feature)] = enabled
}

func (m *MockEngine) SetLimit(entityType string, limit int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.limits[entityType] = limit
}

func (m *MockEngine) SetCurrentCount(entityType string, count int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.counts[entityType] = count
}

func (m *MockEngine) UpsertGrant(_ context.Context, grant subscription.Grant) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.grants[grant.ID] = grant
	return nil
}

func (m *MockEngine) RevokeGrant(_ context.Context, grantID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.grants, grantID)
	return nil
}

func (m *MockEngine) ListGrants(_ context.Context, subject subscription.SubjectRef) ([]subscription.Grant, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]subscription.Grant, 0, len(m.grants))
	for _, grant := range m.grants {
		if grant.Subject != subject {
			continue
		}
		out = append(out, grant)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

func (m *MockEngine) AssignPlan(_ context.Context, _ subscription.SubjectRef, planID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.plan = planID
	return nil
}

func (m *MockEngine) CurrentPlan(_ context.Context, _ subscription.Subject) (subscription.PlanInfo, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return subscription.PlanInfo{
		ID:          m.plan,
		DisplayName: m.plan,
	}, nil
}

func (m *MockEngine) EvaluateFeature(_ context.Context, subject subscription.Subject, feature subscription.FeatureKey) (subscription.Decision, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return subscription.Decision{
		Allowed: m.features[feature],
		Subject: subject.Ref(),
		Feature: feature,
		PlanID:  m.plan,
		Reason:  "mock decision",
		Version: "mock",
	}, nil
}

func (m *MockEngine) EvaluateLimit(_ context.Context, subject subscription.Subject, quota subscription.QuotaKey) (subscription.LimitDecision, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	limit, ok := m.limits[quota.Resource]
	if !ok {
		limit = -1
	}
	key := usageKey(subject.Ref(), quota)
	current := m.usageByKey[key]
	if current == 0 {
		current = m.counts[quota.Resource]
	}
	current += m.pendingAmount(subject.Ref(), quota)
	allowed := limit < 0 || current < limit
	remaining := -1
	if limit >= 0 {
		remaining = max(limit-current, 0)
	}

	return subscription.LimitDecision{
		Allowed:   allowed,
		Subject:   subject.Ref(),
		Quota:     quota,
		Current:   current,
		Limit:     limit,
		Remaining: remaining,
		PlanID:    m.plan,
		Reason:    "mock decision",
		Version:   "mock",
	}, nil
}

func (m *MockEngine) Reserve(_ context.Context, subject subscription.Subject, quota subscription.QuotaKey, amount int, token string) (subscription.Reservation, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if existing, ok := m.reservations[token]; ok {
		if existing.Status == subscription.ReservationReleased || existing.Status == subscription.ReservationExpired {
			delete(m.reservations, token)
		} else if existing.Subject == subject.Ref() && existing.Quota == quota && existing.Amount == amount {
			return existing, nil
		} else {
			return subscription.Reservation{}, fmt.Errorf("reservation token conflict")
		}
	}
	limit, ok := m.limits[quota.Resource]
	if !ok {
		limit = -1
	}
	key := usageKey(subject.Ref(), quota)
	current := m.usageByKey[key]
	if current == 0 {
		current = m.counts[quota.Resource]
	}
	current += m.pendingAmount(subject.Ref(), quota)
	if limit >= 0 && current+amount > limit {
		return subscription.Reservation{}, subscription.LimitExceededError{
			Quota:   quota,
			Current: current,
			Limit:   limit,
		}
	}
	m.nextResID++
	res := subscription.Reservation{
		ID:      fmt.Sprintf("resv-%d", m.nextResID),
		Token:   token,
		Subject: subject.Ref(),
		Quota:   quota,
		Amount:  amount,
		Status:  subscription.ReservationPending,
	}
	m.reservations[token] = res
	return res, nil
}

func (m *MockEngine) Commit(_ context.Context, reservationID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for token, reservation := range m.reservations {
		if reservation.ID != reservationID {
			continue
		}
		if reservation.Status == subscription.ReservationPending {
			key := usageKey(reservation.Subject, reservation.Quota)
			m.usageByKey[key] += reservation.Amount
			m.counts[reservation.Quota.Resource] += reservation.Amount
		}
		reservation.Status = subscription.ReservationCommitted
		m.reservations[token] = reservation
		return nil
	}
	return subscription.ErrReservationNotFound
}

func (m *MockEngine) Release(_ context.Context, reservationID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for token, reservation := range m.reservations {
		if reservation.ID != reservationID {
			continue
		}
		if reservation.Status == subscription.ReservationCommitted {
			key := usageKey(reservation.Subject, reservation.Quota)
			current := m.usageByKey[key]
			m.usageByKey[key] = max(current-reservation.Amount, 0)
			m.counts[reservation.Quota.Resource] = max(m.counts[reservation.Quota.Resource]-reservation.Amount, 0)
		}
		reservation.Status = subscription.ReservationReleased
		m.reservations[token] = reservation
		return nil
	}
	return subscription.ErrReservationNotFound
}

func (m *MockEngine) SetUsage(_ context.Context, subject subscription.SubjectRef, quota subscription.QuotaKey, amount int) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := usageKey(subject, quota)
	m.usageByKey[key] = amount
	m.counts[quota.Resource] = amount
	return nil
}

func usageKey(subject subscription.SubjectRef, quota subscription.QuotaKey) string {
	return fmt.Sprintf("%s|%s|%s|%s", subject.Scope, subject.ID.String(), quota.Resource, quota.Window)
}

func (m *MockEngine) pendingAmount(subject subscription.SubjectRef, quota subscription.QuotaKey) int {
	total := 0
	for _, reservation := range m.reservations {
		if reservation.Status == subscription.ReservationPending &&
			reservation.Subject == subject &&
			reservation.Quota == quota {
			total += reservation.Amount
		}
	}
	return total
}

var _ subscription.Engine = (*MockEngine)(nil)
