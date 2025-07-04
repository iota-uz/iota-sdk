package persistence_test

import (
	"testing"
	"time"

	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/repo"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/modules/billing/domain/aggregates/billing"
	"github.com/iota-uz/iota-sdk/modules/billing/domain/aggregates/details"
	"github.com/iota-uz/iota-sdk/modules/billing/infrastructure/persistence"
)

func createTestClickTransaction(merchantTransID string, tenantID uuid.UUID) billing.Transaction {
	click := details.NewClickDetails(
		merchantTransID,
		details.ClickWithLink("https://example.com/pay"),
	)
	return billing.New(
		150.75,
		billing.UZS,
		billing.Click,
		click,
		billing.WithTenantID(tenantID),
		billing.WithStatus(billing.Created),
	)
}

func TestBillingRepository_Create(t *testing.T) {
	t.Parallel()
	f := setupTest(t)
	repo := persistence.NewBillingRepository()

	tenant, err := composables.UseTenantID(f.Ctx)
	require.NoError(t, err)

	tx := createTestClickTransaction("click-merchant-1", tenant)

	created, err := repo.Save(f.Ctx, tx)
	require.NoError(t, err)

	assert.NotEqual(t, uuid.Nil, created.ID())
	assert.Equal(t, billing.Click, created.Gateway())
	assert.Equal(t, billing.Created, created.Status())

	click := created.Details().(details.ClickDetails)
	assert.Equal(t, "click-merchant-1", click.MerchantTransID())
	assert.Equal(t, "https://example.com/pay", click.Link())
}

func TestBillingRepository_GetByID(t *testing.T) {
	t.Parallel()
	f := setupTest(t)
	repo := persistence.NewBillingRepository()

	tenant, err := composables.UseTenantID(f.Ctx)
	require.NoError(t, err)

	tx := createTestClickTransaction("click-merchant-2", tenant)
	created, err := repo.Save(f.Ctx, tx)
	require.NoError(t, err)

	t.Run("found", func(t *testing.T) {
		fetched, err := repo.GetByID(f.Ctx, created.ID())
		require.NoError(t, err)
		assert.Equal(t, created.ID(), fetched.ID())
	})

	t.Run("not found", func(t *testing.T) {
		_, err := repo.GetByID(f.Ctx, uuid.New())
		require.ErrorIs(t, err, persistence.ErrTransactionNotFound)
	})
}

func TestBillingRepository_GetByDetailsField(t *testing.T) {
	t.Parallel()
	f := setupTest(t)
	repo := persistence.NewBillingRepository()

	tenant, err := composables.UseTenantID(f.Ctx)
	require.NoError(t, err)

	tx := createTestClickTransaction("click-merchant-3", tenant)
	created, err := repo.Save(f.Ctx, tx)
	require.NoError(t, err)

	filters := []billing.DetailsFieldFilter{
		{
			Path:     []string{"merchant_trans_id"},
			Operator: billing.OpEqual,
			Value:    "click-merchant-3",
		},
	}

	foundList, err := repo.GetByDetailsFields(f.Ctx, billing.Click, filters)
	require.NoError(t, err)
	require.Len(t, foundList, 1)

	found := foundList[0]
	assert.Equal(t, created.ID(), found.ID())

	click := found.Details().(details.ClickDetails)
	assert.Equal(t, "click-merchant-3", click.MerchantTransID())
}

func TestBillingRepository_Update(t *testing.T) {
	t.Parallel()
	f := setupTest(t)
	repo := persistence.NewBillingRepository()

	tenant, err := composables.UseTenantID(f.Ctx)
	require.NoError(t, err)

	tx := createTestClickTransaction("click-merchant-4", tenant)
	created, err := repo.Save(f.Ctx, tx)
	require.NoError(t, err)

	updated := created.SetStatus(billing.Completed)
	result, err := repo.Save(f.Ctx, updated)
	require.NoError(t, err)

	assert.Equal(t, billing.Completed, result.Status())
	assert.Equal(t, created.ID(), result.ID())
	assert.True(t, result.UpdatedAt().After(result.CreatedAt()) || result.UpdatedAt().Equal(result.CreatedAt()))
}

func TestBillingRepository_Delete(t *testing.T) {
	t.Parallel()
	f := setupTest(t)
	repo := persistence.NewBillingRepository()

	tenant, err := composables.UseTenantID(f.Ctx)
	require.NoError(t, err)

	tx := createTestClickTransaction("click-merchant-5", tenant)
	created, err := repo.Save(f.Ctx, tx)
	require.NoError(t, err)

	err = repo.Delete(f.Ctx, created.ID())
	require.NoError(t, err)

	_, err = repo.GetByID(f.Ctx, created.ID())
	require.ErrorIs(t, err, persistence.ErrTransactionNotFound)
}

func TestBillingRepository_Count(t *testing.T) {
	t.Parallel()
	f := setupTest(t)
	bRepo := persistence.NewBillingRepository()

	tenant, err := composables.UseTenantID(f.Ctx)
	require.NoError(t, err)

	initial, err := bRepo.Count(f.Ctx, &billing.FindParams{
		Filters: []billing.Filter{
			{
				Column: billing.TenantIDField,
				Filter: repo.Eq(tenant),
			},
		},
	})
	require.NoError(t, err)

	_, err = bRepo.Save(f.Ctx, createTestClickTransaction("click-merchant-6", tenant))
	require.NoError(t, err)

	after, err := bRepo.Count(f.Ctx, &billing.FindParams{})
	require.NoError(t, err)

	assert.Equal(t, initial+1, after)
}

func TestBillingRepository_GetAll(t *testing.T) {
	t.Parallel()
	f := setupTest(t)
	repo := persistence.NewBillingRepository()

	tenant, err := composables.UseTenantID(f.Ctx)
	require.NoError(t, err)

	_, err = repo.Save(f.Ctx, createTestClickTransaction("click-merchant-7", tenant))
	require.NoError(t, err)

	all, err := repo.GetAll(f.Ctx)
	require.NoError(t, err)

	assert.NotEmpty(t, all)
}

func TestBillingRepository_GetPaginated(t *testing.T) {
	t.Parallel()
	f := setupTest(t)
	repo := persistence.NewBillingRepository()

	tenant, err := composables.UseTenantID(f.Ctx)
	require.NoError(t, err)

	for i := 0; i < 4; i++ {
		id := "click-merchant-" + uuid.New().String()
		_, err := repo.Save(f.Ctx, createTestClickTransaction(id, tenant))
		require.NoError(t, err)
		time.Sleep(10 * time.Millisecond)
	}

	t.Run("Limit + Offset", func(t *testing.T) {
		params := &billing.FindParams{
			Limit:  2,
			Offset: 1,
			SortBy: billing.SortBy{
				Fields: []billing.SortByField{{Field: billing.CreatedAt, Ascending: true}},
			},
		}

		page, err := repo.GetPaginated(f.Ctx, params)
		require.NoError(t, err)
		assert.Len(t, page, 2)
	})
}
