package services_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/iota-uz/iota-sdk/modules/billing/domain/aggregates/billing"
	"github.com/iota-uz/iota-sdk/modules/billing/domain/aggregates/details"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/billing/services"
	"github.com/iota-uz/iota-sdk/pkg/composables"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Test ---

func TestBillingService_CreateTransaction_Click(t *testing.T) {
	t.Helper()
	t.Parallel()
	f := setupTest(t)

	tenant, err := composables.UseTenantID(f.ctx)
	require.NoError(t, err)

	cmd := &services.CreateTransactionCommand{
		TenantID: tenant,
		Quantity: 1001,
		Currency: billing.UZS,
		Gateway:  billing.Click,
		Details: details.NewClickDetails(
			"granit_test",
			details.ClickWithParams(map[string]any{}),
		),
	}

	result, err := f.billingService.Create(f.ctx, cmd)
	require.NoError(t, err, "Create should succeed")
	require.NotNil(t, result, "Transaction should not be nil")

	assert.Equal(t, billing.Created, result.Status())
	assert.Equal(t, billing.UZS, result.Amount().Currency())
	assert.InDelta(t, float64(1001), result.Amount().Quantity(), 0.0001)
	assert.NotEqual(t, uuid.Nil, result.ID(), "Expected non-nil transaction ID")
	assert.WithinDuration(t, time.Now(), result.CreatedAt(), time.Second*2)
}

func TestBillingService_CreateTransaction_Payme(t *testing.T) {
	t.Helper()
	t.Parallel()
	f := setupTest(t)

	tenant, err := composables.UseTenantID(f.ctx)
	require.NoError(t, err)

	for i := 1; i <= 40; i++ {
		t.Run(fmt.Sprintf("Payme_Transaction_%d", i), func(t *testing.T) {
			t.Parallel()

			orderID := fmt.Sprintf("%d", i)
			amount := float64(1000 + i)

			cmd := &services.CreateTransactionCommand{
				TenantID: tenant,
				Quantity: amount,
				Currency: billing.UZS,
				Gateway:  billing.Payme,
				Details: details.NewPaymeDetails(
					uuid.New().String(),
					details.PaymeWithAccount(map[string]any{
						"order_id": orderID,
					}),
				),
			}

			result, err := f.billingService.Create(f.ctx, cmd)
			require.NoError(t, err, "Create should succeed")
			require.NotNil(t, result, "Transaction should not be nil")

			assert.Equal(t, billing.Created, result.Status())
			assert.Equal(t, billing.UZS, result.Amount().Currency())
			assert.InDelta(t, amount, result.Amount().Quantity(), 0.0001)
			assert.NotEqual(t, uuid.Nil, result.ID(), "Expected non-nil transaction ID")
			assert.WithinDuration(t, time.Now(), result.CreatedAt(), time.Second*2)

			payme := result.Details().(details.PaymeDetails)
			assert.Equal(t, orderID, payme.Account()["order_id"])
		})
	}
}

//func TestBillingService_CreateTransaction_Octo(t *testing.T) {
//	t.Helper()
//	t.Parallel()
//	f := setupTest(t)
//
//	tenant, err := composables.UseTenantID(f.ctx)
//	require.NoError(t, err)
//
//	shopTransactionId := uuid.New().String()
//
//	cmd := &services.CreateTransactionCommand{
//		TenantID: tenant,
//		Quantity: 1000,
//		Currency: billing.UZS,
//		Gateway:  billing.Octo,
//		Details: details.NewOctoDetails(
//			shopTransactionId,
//			details.OctoWithAutoCapture(false),
//			details.OctoWithTest(true),
//			details.OctoWithDescription("Test"),
//			details.OctoWithReturnUrl("https://octo.uz"),
//		),
//	}
//
//	result, err := f.billingService.Create(f.ctx, cmd)
//
//	require.NoError(t, err, "Create should succeed")
//	require.NotNil(t, result, "Transaction should not be nil")
//
//	assert.Equal(t, billing.Created, result.Status())
//	assert.Equal(t, billing.UZS, result.Amount().Currency())
//	assert.InDelta(t, 1000, result.Amount().Quantity(), 0.0001)
//	assert.NotEqual(t, uuid.Nil, result.ID(), "Expected non-nil transaction ID")
//
//	octo := result.Details().(details.OctoDetails)
//	assert.Equal(t, shopTransactionId, octo.ShopTransactionId())
//}

//func TestBillingService_CreateTransaction_Stripe(t *testing.T) {
//	t.Helper()
//	t.Parallel()
//	f := setupTest(t)
//
//	tenant, err := composables.UseTenantID(f.ctx)
//	require.NoError(t, err)
//
//	cmd := &services.CreateTransactionCommand{
//		TenantID: tenant,
//		Quantity: 10,
//		Currency: billing.USD,
//		Gateway:  billing.Stripe,
//		Details: details.NewStripeDetails(
//			uuid.New().String(),
//			details.StripeWithMode("subscription"),
//			details.StripeWithSuccessURL("https://iota-sdk-staging.up.railway.app/success"),
//			details.StripeWithCancelURL("https://iota-sdk-staging.up.railway.app/cancel"),
//			details.StripeWithItems([]details.StripeItem{
//				details.NewStripeItem("price_1RThAKFds3jHiGEnoRBQBBmr", 1),
//			}),
//		),
//	}
//
//	result, err := f.billingService.Create(f.ctx, cmd)
//	require.NoError(t, err, "Create should succeed")
//	require.NotNil(t, result, "Transaction should not be nil")
//
//	assert.Equal(t, billing.Created, result.Status())
//	assert.Equal(t, billing.USD, result.Amount().Currency())
//	assert.InDelta(t, float64(10), result.Amount().Quantity(), 0.0001)
//	assert.NotEqual(t, uuid.Nil, result.ID(), "Expected non-nil transaction ID")
//	assert.WithinDuration(t, time.Now(), result.CreatedAt(), time.Second*2)
//}
