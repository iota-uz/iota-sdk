package subscription

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	subrepo "github.com/iota-uz/iota-sdk/pkg/subscription/repository"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeRepo struct {
	entitlements map[uuid.UUID]*subrepo.Entitlement
	counts       map[uuid.UUID]map[string]int
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{
		entitlements: map[uuid.UUID]*subrepo.Entitlement{},
		counts:       map[uuid.UUID]map[string]int{},
	}
}

func (f *fakeRepo) GetEntitlement(_ context.Context, tenantID uuid.UUID) (*subrepo.Entitlement, error) {
	entry, ok := f.entitlements[tenantID]
	if !ok {
		return nil, subrepo.ErrEntitlementNotFound
	}
	copyEntry := *entry
	return &copyEntry, nil
}

func (f *fakeRepo) UpsertEntitlement(_ context.Context, entitlement *subrepo.Entitlement) error {
	copyEntry := *entitlement
	if copyEntry.CreatedAt.IsZero() {
		copyEntry.CreatedAt = time.Now().UTC()
	}
	if copyEntry.UpdatedAt.IsZero() {
		copyEntry.UpdatedAt = time.Now().UTC()
	}
	f.entitlements[copyEntry.TenantID] = &copyEntry
	return nil
}

func (f *fakeRepo) SetStripeReferences(_ context.Context, tenantID uuid.UUID, customerID, subscriptionID *string) error {
	entry, ok := f.entitlements[tenantID]
	if !ok {
		return subrepo.ErrEntitlementNotFound
	}
	entry.StripeCustomerID = customerID
	entry.StripeSubscriptionID = subscriptionID
	return nil
}

func (f *fakeRepo) FindTenantByStripeCustomer(_ context.Context, customerID string) (uuid.UUID, error) {
	for tenantID, entry := range f.entitlements {
		if entry.StripeCustomerID != nil && *entry.StripeCustomerID == customerID {
			return tenantID, nil
		}
	}
	return uuid.Nil, subrepo.ErrEntitlementNotFound
}

func (f *fakeRepo) FindTenantByStripeSubscription(_ context.Context, subscriptionID string) (uuid.UUID, error) {
	for tenantID, entry := range f.entitlements {
		if entry.StripeSubscriptionID != nil && *entry.StripeSubscriptionID == subscriptionID {
			return tenantID, nil
		}
	}
	return uuid.Nil, subrepo.ErrEntitlementNotFound
}

func (f *fakeRepo) SetGracePeriod(_ context.Context, tenantID uuid.UUID, inGrace bool, endsAt *time.Time) error {
	entry, ok := f.entitlements[tenantID]
	if !ok {
		return subrepo.ErrEntitlementNotFound
	}
	entry.InGracePeriod = inGrace
	entry.GracePeriodEndsAt = endsAt
	return nil
}

func (f *fakeRepo) SetTier(_ context.Context, tenantID uuid.UUID, tier string) error {
	entry, ok := f.entitlements[tenantID]
	if !ok {
		return subrepo.ErrEntitlementNotFound
	}
	entry.Tier = tier
	return nil
}

func (f *fakeRepo) TouchSyncedAt(_ context.Context, tenantID uuid.UUID, syncedAt time.Time) error {
	entry, ok := f.entitlements[tenantID]
	if !ok {
		return subrepo.ErrEntitlementNotFound
	}
	entry.LastSyncedAt = &syncedAt
	return nil
}

func (f *fakeRepo) GetEntityCounts(_ context.Context, tenantID uuid.UUID) (map[string]int, error) {
	src := f.counts[tenantID]
	dst := map[string]int{}
	for k, v := range src {
		dst[k] = v
	}
	return dst, nil
}

func (f *fakeRepo) GetEntityCount(_ context.Context, tenantID uuid.UUID, entityType string) (int, error) {
	return f.counts[tenantID][entityType], nil
}

func (f *fakeRepo) SetEntityCount(_ context.Context, tenantID uuid.UUID, entityType string, count int) error {
	if _, ok := f.counts[tenantID]; !ok {
		f.counts[tenantID] = map[string]int{}
	}
	f.counts[tenantID][entityType] = count
	return nil
}

func (f *fakeRepo) IncrementEntityCount(_ context.Context, tenantID uuid.UUID, entityType string) error {
	if _, ok := f.counts[tenantID]; !ok {
		f.counts[tenantID] = map[string]int{}
	}
	f.counts[tenantID][entityType]++
	return nil
}

func (f *fakeRepo) DecrementEntityCount(_ context.Context, tenantID uuid.UUID, entityType string) error {
	if _, ok := f.counts[tenantID]; !ok {
		f.counts[tenantID] = map[string]int{}
	}
	if f.counts[tenantID][entityType] > 0 {
		f.counts[tenantID][entityType]--
	}
	return nil
}

func (f *fakeRepo) AddSeatIfBelow(_ context.Context, tenantID uuid.UUID, max int) (bool, error) {
	entry, ok := f.entitlements[tenantID]
	if !ok {
		return false, subrepo.ErrEntitlementNotFound
	}
	if entry.CurrentSeats >= max {
		return false, nil
	}
	entry.CurrentSeats++
	return true, nil
}

func (f *fakeRepo) IncrementSeat(_ context.Context, tenantID uuid.UUID) error {
	entry, ok := f.entitlements[tenantID]
	if !ok {
		return subrepo.ErrEntitlementNotFound
	}
	entry.CurrentSeats++
	return nil
}

func (f *fakeRepo) DecrementSeat(_ context.Context, tenantID uuid.UUID) error {
	entry, ok := f.entitlements[tenantID]
	if !ok {
		return subrepo.ErrEntitlementNotFound
	}
	if entry.CurrentSeats > 0 {
		entry.CurrentSeats--
	}
	return nil
}

func (f *fakeRepo) UpsertPlans(_ context.Context, _ []subrepo.Plan) error {
	return nil
}

func TestNewService_TierInheritanceAndFeatureChecks(t *testing.T) {
	t.Parallel()

	repo := newFakeRepo()
	cfg := Config{
		DefaultTier: "FREE",
		Tiers: []TierDefinition{
			{
				Tier:         "FREE",
				DisplayName:  "Free",
				Features:     []string{"core_access"},
				EntityLimits: map[string]int{"drivers": 1},
			},
			{
				Tier:         "PRO",
				DisplayName:  "Pro",
				ParentTier:   "FREE",
				Features:     []string{"shyona_access"},
				EntityLimits: map[string]int{"drivers": -1},
			},
		},
	}

	svc, err := NewService(cfg, nil, WithRepository(repo), WithCache(NewMemoryCache()))
	require.NoError(t, err)

	tenantID := uuid.New()
	repo.entitlements[tenantID] = &subrepo.Entitlement{
		TenantID:      tenantID,
		Tier:          "PRO",
		Features:      []string{},
		EntityLimits:  map[string]int{},
		CurrentSeats:  0,
		CreatedAt:     time.Now().UTC(),
		UpdatedAt:     time.Now().UTC(),
		InGracePeriod: false,
	}

	hasCore, err := svc.HasFeature(context.Background(), tenantID, "core_access")
	require.NoError(t, err)
	assert.True(t, hasCore)

	hasShyona, err := svc.HasFeature(context.Background(), tenantID, "shyona_access")
	require.NoError(t, err)
	assert.True(t, hasShyona)
}

func TestNewService_TierInheritanceCycle(t *testing.T) {
	t.Parallel()

	cfg := Config{
		DefaultTier: "A",
		Tiers: []TierDefinition{
			{Tier: "A", ParentTier: "B"},
			{Tier: "B", ParentTier: "A"},
		},
	}

	_, err := NewService(cfg, nil, WithRepository(newFakeRepo()), WithCache(NewMemoryCache()))
	require.Error(t, err)
}

func TestService_CheckLimitAndSeatLimit(t *testing.T) {
	t.Parallel()

	repo := newFakeRepo()
	seatLimit := 1
	cfg := Config{
		DefaultTier: "FREE",
		Tiers: []TierDefinition{
			{
				Tier:         "FREE",
				DisplayName:  "Free",
				EntityLimits: map[string]int{"drivers": 1},
				SeatLimit:    &seatLimit,
			},
		},
	}
	svc, err := NewService(cfg, nil, WithRepository(repo), WithCache(NewMemoryCache()))
	require.NoError(t, err)

	tenantID := uuid.New()
	repo.entitlements[tenantID] = &subrepo.Entitlement{
		TenantID:      tenantID,
		Tier:          "FREE",
		Features:      []string{},
		EntityLimits:  map[string]int{},
		CurrentSeats:  0,
		CreatedAt:     time.Now().UTC(),
		UpdatedAt:     time.Now().UTC(),
		InGracePeriod: false,
	}
	repo.counts[tenantID] = map[string]int{"drivers": 1}

	limit, err := svc.CheckLimit(context.Background(), tenantID, "drivers")
	require.NoError(t, err)
	assert.False(t, limit.Allowed)

	seat, err := svc.CheckSeatLimit(context.Background(), tenantID)
	require.NoError(t, err)
	assert.True(t, seat.Allowed)

	err = svc.AddSeat(context.Background(), tenantID)
	require.NoError(t, err)
	err = svc.AddSeat(context.Background(), tenantID)
	require.Error(t, err)
}
