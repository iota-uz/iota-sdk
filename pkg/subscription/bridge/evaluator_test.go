package bridge

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/pkg/subscription"
	subrepo "github.com/iota-uz/iota-sdk/pkg/subscription/repository"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeRepository struct {
	entitlements map[uuid.UUID]*subrepo.Entitlement
	entityCounts map[string]int
	getErr       error
}

func newFakeRepository() *fakeRepository {
	return &fakeRepository{
		entitlements: map[uuid.UUID]*subrepo.Entitlement{},
		entityCounts: map[string]int{},
	}
}

func (f *fakeRepository) GetEntitlement(_ context.Context, tenantID uuid.UUID) (*subrepo.Entitlement, error) {
	if f.getErr != nil {
		return nil, f.getErr
	}
	entitlement, ok := f.entitlements[tenantID]
	if !ok {
		return nil, subrepo.ErrEntitlementNotFound
	}
	copyEntitlement := *entitlement
	return &copyEntitlement, nil
}

func (f *fakeRepository) UpsertEntitlement(_ context.Context, entitlement *subrepo.Entitlement) error {
	copyEntitlement := *entitlement
	f.entitlements[entitlement.TenantID] = &copyEntitlement
	return nil
}

func (f *fakeRepository) SetGracePeriod(_ context.Context, tenantID uuid.UUID, inGrace bool, endsAt *time.Time) error {
	entitlement, ok := f.entitlements[tenantID]
	if !ok {
		return subrepo.ErrEntitlementNotFound
	}
	entitlement.InGracePeriod = inGrace
	entitlement.GracePeriodEndsAt = endsAt
	return nil
}

func (f *fakeRepository) SetPlan(_ context.Context, tenantID uuid.UUID, planID string) error {
	entitlement, ok := f.entitlements[tenantID]
	if !ok {
		return subrepo.ErrEntitlementNotFound
	}
	entitlement.PlanID = planID
	return nil
}

func (f *fakeRepository) TouchSyncedAt(_ context.Context, _ uuid.UUID, _ time.Time) error { return nil }
func (f *fakeRepository) UpdateFeaturesAndSync(
	_ context.Context,
	tenantID uuid.UUID,
	features []string,
	_ time.Time,
) error {
	entitlement, ok := f.entitlements[tenantID]
	if !ok {
		return subrepo.ErrEntitlementNotFound
	}
	entitlement.Features = features
	return nil
}

func (f *fakeRepository) GetEntityCounts(_ context.Context, _ uuid.UUID) (map[string]int, error) {
	return map[string]int{}, nil
}

func (f *fakeRepository) GetEntityCount(_ context.Context, _ uuid.UUID, entityType string) (int, error) {
	return f.entityCounts[entityType], nil
}

func (f *fakeRepository) SetEntityCount(_ context.Context, _ uuid.UUID, entityType string, count int) error {
	f.entityCounts[entityType] = count
	return nil
}

func (f *fakeRepository) IncrementEntityCount(_ context.Context, _ uuid.UUID, _ string) error {
	return nil
}
func (f *fakeRepository) IncrementEntityCountIfBelow(_ context.Context, _ uuid.UUID, _ string, _ int) (bool, error) {
	return false, nil
}
func (f *fakeRepository) DecrementEntityCount(_ context.Context, _ uuid.UUID, _ string) error {
	return nil
}
func (f *fakeRepository) AddSeatIfBelow(_ context.Context, _ uuid.UUID, _ int) (bool, error) {
	return false, nil
}
func (f *fakeRepository) IncrementSeat(_ context.Context, _ uuid.UUID) error { return nil }
func (f *fakeRepository) DecrementSeat(_ context.Context, _ uuid.UUID) error { return nil }
func (f *fakeRepository) UpsertPlans(_ context.Context, _ []subscription.PlanDefinition) error {
	return nil
}

func TestEvaluator_ReflectsPlanAndFeatureChangesFromRepository(t *testing.T) {
	t.Parallel()

	repo := newFakeRepository()
	tenantID := uuid.New()
	repo.entitlements[tenantID] = &subrepo.Entitlement{
		TenantID:     tenantID,
		PlanID:       "FREE",
		Features:     []string{},
		EntityLimits: map[string]int{},
	}

	evaluator, err := NewEvaluator(subscription.Config{
		DefaultPlan: "FREE",
		Plans: []subscription.PlanDefinition{
			{
				PlanID:       "FREE",
				Features:     []string{"core_access"},
				EntityLimits: map[string]int{"drivers": 1},
			},
			{
				PlanID:       "PRO",
				Features:     []string{"analytics"},
				EntityLimits: map[string]int{"drivers": 10},
			},
		},
	}, repo)
	require.NoError(t, err)

	subject := subscription.Subject{Scope: subscription.ScopeTenant, ID: tenantID}

	decision, err := evaluator.EvaluateFeature(context.Background(), subject, "analytics")
	require.NoError(t, err)
	assert.False(t, decision.Allowed)

	repo.entitlements[tenantID].PlanID = "PRO"
	decision, err = evaluator.EvaluateFeature(context.Background(), subject, "analytics")
	require.NoError(t, err)
	assert.True(t, decision.Allowed)

	repo.entitlements[tenantID].PlanID = "FREE"
	repo.entitlements[tenantID].Features = []string{"analytics"}
	decision, err = evaluator.EvaluateFeature(context.Background(), subject, "analytics")
	require.NoError(t, err)
	assert.True(t, decision.Allowed)
}

func TestEvaluator_GracePeriodVisibleInDecisions(t *testing.T) {
	t.Parallel()

	repo := newFakeRepository()
	tenantID := uuid.New()
	graceEndsAt := time.Now().Add(24 * time.Hour).UTC()
	repo.entitlements[tenantID] = &subrepo.Entitlement{
		TenantID:          tenantID,
		PlanID:            "FREE",
		Features:          []string{},
		EntityLimits:      map[string]int{"drivers": 1},
		InGracePeriod:     true,
		GracePeriodEndsAt: &graceEndsAt,
	}
	repo.entityCounts["drivers"] = 1
	now := graceEndsAt.Add(-time.Minute)

	evaluator, err := NewEvaluator(subscription.Config{
		DefaultPlan: "FREE",
		Plans: []subscription.PlanDefinition{
			{
				PlanID:       "FREE",
				Features:     []string{"core_access"},
				EntityLimits: map[string]int{"drivers": 1},
			},
		},
	}, repo, WithClock(func() time.Time { return now }))
	require.NoError(t, err)

	subject := subscription.Subject{Scope: subscription.ScopeTenant, ID: tenantID}

	featureDecision, err := evaluator.EvaluateFeature(context.Background(), subject, "unknown")
	require.NoError(t, err)
	assert.Contains(t, featureDecision.Reason, "grace period active")

	limitDecision, err := evaluator.EvaluateLimit(
		context.Background(),
		subject,
		subscription.QuotaKey{Resource: "drivers", Window: subscription.WindowNone},
	)
	require.NoError(t, err)
	assert.Contains(t, limitDecision.Reason, "grace period active")
	assert.False(t, limitDecision.Allowed)
}

func TestEvaluator_MissingEntitlementWrappedErrorFallsBack(t *testing.T) {
	t.Parallel()

	repo := newFakeRepository()
	repo.getErr = fmt.Errorf("wrapped: %w", subrepo.ErrEntitlementNotFound)
	tenantID := uuid.New()

	evaluator, err := NewEvaluator(subscription.Config{
		DefaultPlan: "FREE",
		Plans: []subscription.PlanDefinition{
			{
				PlanID:       "FREE",
				Features:     []string{"core_access"},
				EntityLimits: map[string]int{},
			},
		},
	}, repo)
	require.NoError(t, err)

	subject := subscription.Subject{Scope: subscription.ScopeTenant, ID: tenantID}
	decision, err := evaluator.EvaluateFeature(context.Background(), subject, "core_access")
	require.NoError(t, err)
	assert.True(t, decision.Allowed)
	assert.Equal(t, "FREE", decision.PlanID)
}
