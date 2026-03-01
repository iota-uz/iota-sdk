package stripe

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	subrepo "github.com/iota-uz/iota-sdk/pkg/subscription/repository"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stripe/stripe-go/v82"
)

type fakeRepo struct {
	entitlements        map[uuid.UUID]*subrepo.Entitlement
	findCustomerErr     error
	findSubscriptionErr error
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{entitlements: map[uuid.UUID]*subrepo.Entitlement{}}
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
	f.entitlements[entitlement.TenantID] = &copyEntry
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
	if f.findCustomerErr != nil {
		return uuid.Nil, f.findCustomerErr
	}
	for tenantID, entry := range f.entitlements {
		if entry.StripeCustomerID != nil && *entry.StripeCustomerID == customerID {
			return tenantID, nil
		}
	}
	return uuid.Nil, subrepo.ErrEntitlementNotFound
}
func (f *fakeRepo) FindTenantByStripeSubscription(_ context.Context, subscriptionID string) (uuid.UUID, error) {
	if f.findSubscriptionErr != nil {
		return uuid.Nil, f.findSubscriptionErr
	}
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
func (f *fakeRepo) SetPlan(_ context.Context, tenantID uuid.UUID, planID string) error {
	entry, ok := f.entitlements[tenantID]
	if !ok {
		return subrepo.ErrEntitlementNotFound
	}
	entry.PlanID = planID
	return nil
}
func (f *fakeRepo) TouchSyncedAt(_ context.Context, _ uuid.UUID, _ time.Time) error { return nil }
func (f *fakeRepo) GetEntityCounts(_ context.Context, _ uuid.UUID) (map[string]int, error) {
	return map[string]int{}, nil
}
func (f *fakeRepo) GetEntityCount(_ context.Context, _ uuid.UUID, _ string) (int, error) {
	return 0, nil
}
func (f *fakeRepo) SetEntityCount(_ context.Context, _ uuid.UUID, _ string, _ int) error { return nil }
func (f *fakeRepo) IncrementEntityCount(_ context.Context, _ uuid.UUID, _ string) error  { return nil }
func (f *fakeRepo) IncrementEntityCountIfBelow(_ context.Context, _ uuid.UUID, _ string, _ int) (bool, error) {
	return false, nil
}
func (f *fakeRepo) DecrementEntityCount(_ context.Context, _ uuid.UUID, _ string) error { return nil }
func (f *fakeRepo) AddSeatIfBelow(_ context.Context, _ uuid.UUID, _ int) (bool, error) {
	return false, nil
}
func (f *fakeRepo) IncrementSeat(_ context.Context, _ uuid.UUID) error    { return nil }
func (f *fakeRepo) DecrementSeat(_ context.Context, _ uuid.UUID) error    { return nil }
func (f *fakeRepo) UpsertPlans(_ context.Context, _ []subrepo.Plan) error { return nil }

type fakeClient struct {
	features []string
}

func (f fakeClient) ListActiveEntitlements(_ context.Context, _ string) ([]string, error) {
	return append([]string{}, f.features...), nil
}

type fakeInvalidator struct {
	calls int
	err   error
}

func (f *fakeInvalidator) InvalidateCache(_ context.Context, _ uuid.UUID) error {
	f.calls++
	return f.err
}

func TestHandleStripeEvent_InvoicePaymentFailedSetsGracePeriod(t *testing.T) {
	t.Parallel()

	repo := newFakeRepo()
	invalidator := &fakeInvalidator{}
	service := NewService(
		Config{SecretKey: "sk_test", GracePeriodDays: 7, DefaultPlan: "FREE"},
		repo,
		invalidator,
		fakeClient{features: []string{"core_access"}},
	)

	tenantID := uuid.New()
	customerID := "cus_123"
	subscriptionID := "sub_123"
	repo.entitlements[tenantID] = &subrepo.Entitlement{
		TenantID:             tenantID,
		PlanID:               "FREE",
		StripeCustomerID:     &customerID,
		StripeSubscriptionID: &subscriptionID,
		Features:             []string{},
		EntityLimits:         map[string]int{},
		CreatedAt:            time.Now().UTC(),
		UpdatedAt:            time.Now().UTC(),
	}

	payload := map[string]any{
		"id": "in_123",
		"customer": map[string]any{
			"id": customerID,
		},
		"subscription": map[string]any{
			"id": subscriptionID,
		},
	}
	raw, err := json.Marshal(payload)
	require.NoError(t, err)

	event := stripe.Event{
		Type: "invoice.payment_failed",
		Data: &stripe.EventData{Raw: raw},
	}
	err = service.HandleStripeEvent(context.Background(), event)
	require.NoError(t, err)

	assert.True(t, repo.entitlements[tenantID].InGracePeriod)
	assert.NotNil(t, repo.entitlements[tenantID].GracePeriodEndsAt)
	assert.Greater(t, invalidator.calls, 0)
}

func TestHandleStripeEvent_InvoicePaymentFailedPropagatesLookupError(t *testing.T) {
	t.Parallel()

	expectedErr := errors.New("lookup failure")
	repo := newFakeRepo()
	repo.findCustomerErr = expectedErr
	service := NewService(
		Config{SecretKey: "sk_test", GracePeriodDays: 7, DefaultPlan: "FREE"},
		repo,
		&fakeInvalidator{},
		fakeClient{features: []string{"core_access"}},
	)

	raw, err := json.Marshal(map[string]any{
		"id": "in_456",
		"customer": map[string]any{
			"id": "cus_error",
		},
	})
	require.NoError(t, err)

	event := stripe.Event{
		Type: "invoice.payment_failed",
		Data: &stripe.EventData{Raw: raw},
	}
	err = service.HandleStripeEvent(context.Background(), event)
	require.Error(t, err)
	assert.ErrorIs(t, err, expectedErr)
}

func TestRefreshTenant_NoCustomerPropagatesInvalidationError(t *testing.T) {
	t.Parallel()

	repo := newFakeRepo()
	expectedErr := errors.New("cache unavailable")
	invalidator := &fakeInvalidator{err: expectedErr}
	service := NewService(
		Config{SecretKey: "sk_test", GracePeriodDays: 7, DefaultPlan: "FREE"},
		repo,
		invalidator,
		fakeClient{features: []string{"core_access"}},
	)

	tenantID := uuid.New()
	repo.entitlements[tenantID] = &subrepo.Entitlement{
		TenantID:  tenantID,
		PlanID:    "FREE",
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}

	err := service.RefreshTenant(context.Background(), tenantID)
	require.Error(t, err)
	assert.ErrorIs(t, err, expectedErr)
}
