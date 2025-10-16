package services_test

import (
	"context"
	"errors"
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
	billingService := getBillingService(f)

	tenant, err := composables.UseTenantID(f.Ctx)
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

	result, err := billingService.Create(f.Ctx, cmd)
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
	billingService := getBillingService(f)

	tenant, err := composables.UseTenantID(f.Ctx)
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

			result, err := billingService.Create(f.Ctx, cmd)
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
//				details.NewStripeItem("price_1RThAKFds3jHiGEnoRBQBBmr", 1, details.StripeItemWithAdjustableQuantity(true, 0, 100)),
//			}),
//			details.StripeWithSubscription(details.NewStripeSubscriptionData(details.StripeSubscriptionDataWithTrialPeriodDays(7))),
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

func TestBillingService_RegisterCallback(t *testing.T) {
	t.Helper()
	t.Parallel()
	f := setupTest(t)
	billingService := getBillingService(f)

	callbackInvoked := false
	testCallback := func(ctx context.Context, tx billing.Transaction) error {
		callbackInvoked = true
		return nil
	}

	billingService.RegisterCallback(testCallback)

	// Create a test transaction
	transaction := billing.New(
		100.0,
		billing.UZS,
		billing.Click,
		details.NewClickDetails("test-123"),
	)

	err := billingService.InvokeCallback(f.Ctx, transaction)
	require.NoError(t, err, "InvokeCallback should not return error")

	assert.True(t, callbackInvoked, "Callback should have been invoked")
}

func TestBillingService_InvokeCallback_WithError(t *testing.T) {
	t.Helper()
	t.Parallel()
	f := setupTest(t)
	billingService := getBillingService(f)

	expectedError := errors.New("callback processing failed")
	testCallback := func(ctx context.Context, tx billing.Transaction) error {
		return expectedError
	}

	billingService.RegisterCallback(testCallback)

	transaction := billing.New(
		100.0,
		billing.UZS,
		billing.Click,
		details.NewClickDetails("test-123"),
	)

	err := billingService.InvokeCallback(f.Ctx, transaction)
	require.Error(t, err, "Expected error from callback")
	assert.Equal(t, expectedError.Error(), err.Error(), "Error should match expected error")
}

func TestBillingService_InvokeCallback_NoCallbackRegistered(t *testing.T) {
	t.Helper()
	t.Parallel()
	f := setupTest(t)
	billingService := getBillingService(f)

	// No callback registered
	transaction := billing.New(
		100.0,
		billing.UZS,
		billing.Click,
		details.NewClickDetails("test-123"),
	)

	err := billingService.InvokeCallback(f.Ctx, transaction)
	require.NoError(t, err, "InvokeCallback should not error when no callback is registered")
}

func TestBillingService_Callback_ThreadSafety(t *testing.T) {
	t.Helper()
	t.Parallel()
	f := setupTest(t)
	billingService := getBillingService(f)

	transaction := billing.New(
		100.0,
		billing.UZS,
		billing.Click,
		details.NewClickDetails("test-123"),
	)

	const goroutines = 10
	done := make(chan bool, goroutines*2)

	// Concurrently register callbacks and invoke them
	for i := 0; i < goroutines; i++ {
		// Register callback
		go func(idx int) {
			testCallback := func(ctx context.Context, tx billing.Transaction) error {
				return nil
			}
			billingService.RegisterCallback(testCallback)
			done <- true
		}(i)

		// Invoke callback
		go func() {
			_ = billingService.InvokeCallback(f.Ctx, transaction)
			done <- true
		}()
	}

	// Wait for all goroutines to complete
	for i := 0; i < goroutines*2; i++ {
		<-done
	}

	// Should not panic or deadlock - test passes if we reach here
	assert.True(t, true, "Thread safety test completed without deadlock or panic")
}
