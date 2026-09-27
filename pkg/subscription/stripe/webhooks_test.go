package stripe

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/pkg/subscription"
	subrepo "github.com/iota-uz/iota-sdk/pkg/subscription/repository"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stripe/stripe-go/v82"
)

type fakeRepo struct {
	entitlements        map[uuid.UUID]*subrepo.Entitlement
	stripeRefs          map[uuid.UUID]*subrepo.StripeReferences
	findCustomerErr     error
	findSubscriptionErr error
	seenEvents          map[string]time.Time
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{
		entitlements: map[uuid.UUID]*subrepo.Entitlement{},
		stripeRefs:   map[uuid.UUID]*subrepo.StripeReferences{},
		seenEvents:   map[string]time.Time{},
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
	f.entitlements[entitlement.TenantID] = &copyEntry
	return nil
}
func (f *fakeRepo) SetStripeReferences(_ context.Context, tenantID uuid.UUID, customerID, subscriptionID *string) error {
	if _, ok := f.entitlements[tenantID]; !ok {
		return subrepo.ErrEntitlementNotFound
	}
	refs := &subrepo.StripeReferences{TenantID: tenantID}
	if existing, ok := f.stripeRefs[tenantID]; ok && existing != nil {
		refs.SubscriptionEnds = existing.SubscriptionEnds
	}
	refs.CustomerID = customerID
	refs.SubscriptionID = subscriptionID
	f.stripeRefs[tenantID] = refs
	return nil
}
func (f *fakeRepo) GetStripeReferences(_ context.Context, tenantID uuid.UUID) (*subrepo.StripeReferences, error) {
	if _, ok := f.entitlements[tenantID]; !ok {
		return nil, subrepo.ErrEntitlementNotFound
	}
	existing, ok := f.stripeRefs[tenantID]
	if !ok || existing == nil {
		return &subrepo.StripeReferences{TenantID: tenantID}, nil
	}
	copyEntry := *existing
	return &copyEntry, nil
}
func (f *fakeRepo) setStripeRefs(tenantID uuid.UUID, customerID, subscriptionID string) {
	refs := &subrepo.StripeReferences{
		TenantID: tenantID,
	}
	if customerID != "" {
		refs.CustomerID = &customerID
	}
	if subscriptionID != "" {
		refs.SubscriptionID = &subscriptionID
	}
	f.stripeRefs[tenantID] = refs
}
func (f *fakeRepo) stripeRefsFor(tenantID uuid.UUID) *subrepo.StripeReferences {
	return f.stripeRefs[tenantID]
}
func (f *fakeRepo) FindTenantByStripeCustomer(_ context.Context, customerID string) (uuid.UUID, error) {
	if f.findCustomerErr != nil {
		return uuid.Nil, f.findCustomerErr
	}
	for tenantID, refs := range f.stripeRefs {
		if refs != nil && refs.CustomerID != nil && *refs.CustomerID == customerID {
			return tenantID, nil
		}
	}
	return uuid.Nil, subrepo.ErrEntitlementNotFound
}
func (f *fakeRepo) FindTenantByStripeSubscription(_ context.Context, subscriptionID string) (uuid.UUID, error) {
	if f.findSubscriptionErr != nil {
		return uuid.Nil, f.findSubscriptionErr
	}
	for tenantID, refs := range f.stripeRefs {
		if refs != nil && refs.SubscriptionID != nil && *refs.SubscriptionID == subscriptionID {
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
func (f *fakeRepo) UpdateFeaturesAndSync(_ context.Context, tenantID uuid.UUID, features []string, syncedAt time.Time) error {
	entry, ok := f.entitlements[tenantID]
	if !ok {
		return subrepo.ErrEntitlementNotFound
	}
	entry.Features = features
	entry.LastSyncedAt = &syncedAt
	return nil
}
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
func (f *fakeRepo) IncrementSeat(_ context.Context, _ uuid.UUID) error                   { return nil }
func (f *fakeRepo) DecrementSeat(_ context.Context, _ uuid.UUID) error                   { return nil }
func (f *fakeRepo) UpsertPlans(_ context.Context, _ []subscription.PlanDefinition) error { return nil }
func (f *fakeRepo) TryMarkWebhookEventProcessed(_ context.Context, eventID, _ string, ttl time.Duration) (bool, error) {
	if eventID == "" {
		return true, nil
	}

	now := time.Now().UTC()
	for id, expiresAt := range f.seenEvents {
		if !expiresAt.After(now) {
			delete(f.seenEvents, id)
		}
	}

	if _, exists := f.seenEvents[eventID]; exists {
		return false, nil
	}

	if ttl <= 0 {
		ttl = 24 * time.Hour
	}
	f.seenEvents[eventID] = now.Add(ttl)
	return true, nil
}

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
	service, err := NewService(
		Config{SecretKey: "sk_test", GracePeriodDays: 7, DefaultPlan: "FREE"},
		repo,
		invalidator,
		fakeClient{features: []string{"core_access"}},
	)
	require.NoError(t, err)

	tenantID := uuid.New()
	customerID := "cus_123"
	subscriptionID := "sub_123"
	repo.entitlements[tenantID] = &subrepo.Entitlement{
		TenantID:     tenantID,
		PlanID:       "FREE",
		Features:     []string{},
		EntityLimits: map[string]int{},
		CreatedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now().UTC(),
	}
	repo.setStripeRefs(tenantID, customerID, subscriptionID)

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
	assert.Positive(t, invalidator.calls)
}

func TestHandleStripeEvent_DuplicateEventSkipped(t *testing.T) {
	t.Parallel()

	repo := newFakeRepo()
	invalidator := &fakeInvalidator{}
	service, err := NewService(
		Config{SecretKey: "sk_test", GracePeriodDays: 7, DefaultPlan: "FREE"},
		repo,
		invalidator,
		fakeClient{features: []string{"core_access"}},
	)
	require.NoError(t, err)

	tenantID := uuid.New()
	customerID := "cus_dupe"
	repo.entitlements[tenantID] = &subrepo.Entitlement{
		TenantID:     tenantID,
		PlanID:       "FREE",
		Features:     []string{},
		EntityLimits: map[string]int{},
		CreatedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now().UTC(),
	}
	repo.setStripeRefs(tenantID, customerID, "")

	raw, err := json.Marshal(map[string]any{
		"id": "in_duplicate",
		"customer": map[string]any{
			"id": customerID,
		},
	})
	require.NoError(t, err)

	event := stripe.Event{
		ID:   "evt_duplicate_1",
		Type: "invoice.payment_failed",
		Data: &stripe.EventData{Raw: raw},
	}

	err = service.HandleStripeEvent(context.Background(), event)
	require.NoError(t, err)
	firstCalls := invalidator.calls
	require.Positive(t, firstCalls)

	err = service.HandleStripeEvent(context.Background(), event)
	require.NoError(t, err)
	assert.Equal(t, firstCalls, invalidator.calls)
}

func TestHandleStripeEvent_InvoicePaymentFailedPropagatesLookupError(t *testing.T) {
	t.Parallel()

	expectedErr := errors.New("lookup failure")
	repo := newFakeRepo()
	repo.findCustomerErr = expectedErr
	service, svcErr := NewService(
		Config{SecretKey: "sk_test", GracePeriodDays: 7, DefaultPlan: "FREE"},
		repo,
		&fakeInvalidator{},
		fakeClient{features: []string{"core_access"}},
	)
	require.NoError(t, svcErr)

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

func TestHandleStripeEvent_SubscriptionCreatedSetsRefs(t *testing.T) {
	t.Parallel()

	repo := newFakeRepo()
	invalidator := &fakeInvalidator{}
	service, err := NewService(
		Config{SecretKey: "sk_test", GracePeriodDays: 7, DefaultPlan: "FREE"},
		repo,
		invalidator,
		fakeClient{features: []string{"core_access"}},
	)
	require.NoError(t, err)

	tenantID := uuid.New()
	customerID := "cus_sub_created"
	subscriptionID := "sub_created_123"

	// Pre-seed an entitlement so ensureEntitlement is a no-op.
	repo.entitlements[tenantID] = &subrepo.Entitlement{
		TenantID:     tenantID,
		PlanID:       "FREE",
		Features:     []string{},
		EntityLimits: map[string]int{},
		CreatedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now().UTC(),
	}
	repo.setStripeRefs(tenantID, customerID, "")

	// Build a stripe.Subscription JSON payload.
	// resolveTenantID will pick up tenant_id from metadata first.
	payload := map[string]any{
		"id": subscriptionID,
		"customer": map[string]any{
			"id": customerID,
		},
		"metadata": map[string]any{
			"tenant_id": tenantID.String(),
		},
	}
	raw, err := json.Marshal(payload)
	require.NoError(t, err)

	event := stripe.Event{
		Type: "customer.subscription.created",
		Data: &stripe.EventData{Raw: raw},
	}
	err = service.HandleStripeEvent(context.Background(), event)
	require.NoError(t, err)

	refs := repo.stripeRefsFor(tenantID)
	require.NotNil(t, refs)
	require.NotNil(t, refs.CustomerID)
	assert.Equal(t, customerID, *refs.CustomerID)
	require.NotNil(t, refs.SubscriptionID)
	assert.Equal(t, subscriptionID, *refs.SubscriptionID)
	assert.Positive(t, invalidator.calls)
}

func TestHandleStripeEvent_InvoicePaymentSucceededClearsGrace(t *testing.T) {
	t.Parallel()

	repo := newFakeRepo()
	invalidator := &fakeInvalidator{}
	service, err := NewService(
		Config{SecretKey: "sk_test", GracePeriodDays: 7, DefaultPlan: "FREE"},
		repo,
		invalidator,
		fakeClient{features: []string{"core_access"}},
	)
	require.NoError(t, err)

	tenantID := uuid.New()
	customerID := "cus_inv_paid"
	graceEndsAt := time.Now().Add(24 * time.Hour).UTC()

	// Start with in_grace_period = true.
	repo.entitlements[tenantID] = &subrepo.Entitlement{
		TenantID:          tenantID,
		PlanID:            "FREE",
		InGracePeriod:     true,
		GracePeriodEndsAt: &graceEndsAt,
		Features:          []string{},
		EntityLimits:      map[string]int{},
		CreatedAt:         time.Now().UTC(),
		UpdatedAt:         time.Now().UTC(),
	}
	repo.setStripeRefs(tenantID, customerID, "")

	// Build a stripe.Invoice JSON payload.
	// resolveTenantID will find the tenant via stored stripe refs in the repo.
	payload := map[string]any{
		"id": "in_paid_123",
		"customer": map[string]any{
			"id": customerID,
		},
	}
	raw, err := json.Marshal(payload)
	require.NoError(t, err)

	event := stripe.Event{
		Type: "invoice.payment_succeeded",
		Data: &stripe.EventData{Raw: raw},
	}
	err = service.HandleStripeEvent(context.Background(), event)
	require.NoError(t, err)

	assert.False(t, repo.entitlements[tenantID].InGracePeriod)
	assert.Positive(t, invalidator.calls)
}

func TestHandleStripeEvent_UnknownEventTypeIgnored(t *testing.T) {
	t.Parallel()

	repo := newFakeRepo()
	service, err := NewService(
		Config{SecretKey: "sk_test", GracePeriodDays: 7, DefaultPlan: "FREE"},
		repo,
		&fakeInvalidator{},
		fakeClient{features: []string{}},
	)
	require.NoError(t, err)

	// An event type the handler does not recognise should be a no-op.
	raw, err := json.Marshal(map[string]any{"id": "prod_xyz"})
	require.NoError(t, err)

	event := stripe.Event{
		Type: "product.created",
		Data: &stripe.EventData{Raw: raw},
	}
	err = service.HandleStripeEvent(context.Background(), event)
	require.NoError(t, err)
}

func TestHandleStripeEvent_SubscriptionDeletedSetsGrace(t *testing.T) {
	t.Parallel()

	repo := newFakeRepo()
	invalidator := &fakeInvalidator{}
	service, err := NewService(
		Config{SecretKey: "sk_test", GracePeriodDays: 7, DefaultPlan: "FREE"},
		repo,
		invalidator,
		fakeClient{features: []string{"core_access"}},
	)
	require.NoError(t, err)

	tenantID := uuid.New()
	customerID := "cus_sub_deleted"
	subscriptionID := "sub_deleted_456"

	// Start with in_grace_period = false.
	repo.entitlements[tenantID] = &subrepo.Entitlement{
		TenantID:      tenantID,
		PlanID:        "PRO",
		InGracePeriod: false,
		Features:      []string{"core_access"},
		EntityLimits:  map[string]int{},
		CreatedAt:     time.Now().UTC(),
		UpdatedAt:     time.Now().UTC(),
	}
	repo.setStripeRefs(tenantID, customerID, subscriptionID)

	// Build a stripe.Subscription JSON payload.
	payload := map[string]any{
		"id": subscriptionID,
		"customer": map[string]any{
			"id": customerID,
		},
		"metadata": map[string]any{
			"tenant_id": tenantID.String(),
		},
	}
	raw, err := json.Marshal(payload)
	require.NoError(t, err)

	event := stripe.Event{
		Type: "customer.subscription.deleted",
		Data: &stripe.EventData{Raw: raw},
	}
	err = service.HandleStripeEvent(context.Background(), event)
	require.NoError(t, err)

	assert.True(t, repo.entitlements[tenantID].InGracePeriod)
	assert.NotNil(t, repo.entitlements[tenantID].GracePeriodEndsAt)
	assert.Positive(t, invalidator.calls)
}

func TestRefreshTenant_NoCustomerPropagatesInvalidationError(t *testing.T) {
	t.Parallel()

	repo := newFakeRepo()
	expectedErr := errors.New("cache unavailable")
	invalidator := &fakeInvalidator{err: expectedErr}
	service, svcErr := NewService(
		Config{SecretKey: "sk_test", GracePeriodDays: 7, DefaultPlan: "FREE"},
		repo,
		invalidator,
		fakeClient{features: []string{"core_access"}},
	)
	require.NoError(t, svcErr)

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
