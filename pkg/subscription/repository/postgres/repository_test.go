package postgres_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	subrepo "github.com/iota-uz/iota-sdk/pkg/subscription/repository"
	subpostgres "github.com/iota-uz/iota-sdk/pkg/subscription/repository/postgres"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRepository_EntitlementCRUD(t *testing.T) {
	t.Parallel()

	f := setupTest(t)
	repo := subpostgres.NewRepository(f.Pool)

	tenantID, err := composables.UseTenantID(f.Ctx)
	require.NoError(t, err)

	seatLimit := 3
	now := time.Now().UTC()
	entitlement := &subrepo.Entitlement{
		TenantID:      tenantID,
		Tier:          "PRO",
		Features:      []string{"core_access", "shyona_access"},
		EntityLimits:  map[string]int{"drivers": 10},
		SeatLimit:     &seatLimit,
		CurrentSeats:  1,
		InGracePeriod: false,
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	err = repo.UpsertEntitlement(f.Ctx, entitlement)
	require.NoError(t, err)

	got, err := repo.GetEntitlement(f.Ctx, tenantID)
	require.NoError(t, err)
	assert.Equal(t, "PRO", got.Tier)
	assert.Equal(t, 1, got.CurrentSeats)
	assert.Contains(t, got.Features, "shyona_access")
	assert.Equal(t, 10, got.EntityLimits["drivers"])
}

func TestRepository_EntityCountsAndSeats(t *testing.T) {
	t.Parallel()

	f := setupTest(t)
	repo := subpostgres.NewRepository(f.Pool)

	tenantID, err := composables.UseTenantID(f.Ctx)
	require.NoError(t, err)

	err = repo.UpsertEntitlement(f.Ctx, &subrepo.Entitlement{
		TenantID:     tenantID,
		Tier:         "FREE",
		Features:     []string{},
		EntityLimits: map[string]int{},
		CurrentSeats: 0,
		CreatedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now().UTC(),
	})
	require.NoError(t, err)

	err = repo.SetEntityCount(f.Ctx, tenantID, "drivers", 1)
	require.NoError(t, err)
	err = repo.IncrementEntityCount(f.Ctx, tenantID, "drivers")
	require.NoError(t, err)
	err = repo.DecrementEntityCount(f.Ctx, tenantID, "drivers")
	require.NoError(t, err)

	count, err := repo.GetEntityCount(f.Ctx, tenantID, "drivers")
	require.NoError(t, err)
	assert.Equal(t, 1, count)

	ok, err := repo.AddSeatIfBelow(f.Ctx, tenantID, 2)
	require.NoError(t, err)
	assert.True(t, ok)
	ok, err = repo.AddSeatIfBelow(f.Ctx, tenantID, 2)
	require.NoError(t, err)
	assert.True(t, ok)
	ok, err = repo.AddSeatIfBelow(f.Ctx, tenantID, 2)
	require.NoError(t, err)
	assert.False(t, ok)
}

func TestRepository_FindByStripeRefs(t *testing.T) {
	t.Parallel()

	f := setupTest(t)
	repo := subpostgres.NewRepository(f.Pool)

	tenantID, err := composables.UseTenantID(f.Ctx)
	require.NoError(t, err)
	customerID := "cus_123"
	subscriptionID := "sub_123"
	now := time.Now().UTC()

	err = repo.UpsertEntitlement(f.Ctx, &subrepo.Entitlement{
		TenantID:             tenantID,
		Tier:                 "FREE",
		StripeCustomerID:     &customerID,
		StripeSubscriptionID: &subscriptionID,
		Features:             []string{},
		EntityLimits:         map[string]int{},
		CreatedAt:            now,
		UpdatedAt:            now,
	})
	require.NoError(t, err)

	gotTenantByCustomer, err := repo.FindTenantByStripeCustomer(f.Ctx, customerID)
	require.NoError(t, err)
	assert.Equal(t, tenantID, gotTenantByCustomer)

	gotTenantBySub, err := repo.FindTenantByStripeSubscription(f.Ctx, subscriptionID)
	require.NoError(t, err)
	assert.Equal(t, tenantID, gotTenantBySub)

	unknownTenant, err := repo.FindTenantByStripeCustomer(f.Ctx, "cus_unknown")
	require.Error(t, err)
	assert.Equal(t, uuid.Nil, unknownTenant)
}
