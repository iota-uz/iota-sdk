package services_test

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/iota-uz/iota-sdk/modules/billing/domain/aggregates/billing"
	"github.com/iota-uz/iota-sdk/modules/billing/domain/aggregates/details"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/billing/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/modules/billing/services"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/itf"
	"github.com/iota-uz/iota-sdk/pkg/serrors"

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

	for i := 1; i <= 10; i++ {
		t.Run(fmt.Sprintf("Payme_Transaction_%d", i), func(t *testing.T) {
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

// TestBillingService_Cancel_RefusesTerminalStatus asserts that a transaction
// which already left the active set is refused instead of being rewritten to
// "cancelled" — for Refunded that rewrite erases a refund that really happened.
//
// This test is falsely green if Cancel starts failing for an unrelated reason
// (a missing transaction, a provider outage) and the assertion only checked
// that an error came back: it pins the sentinel with ErrorIs and re-reads the
// row to prove the stored status survived, so a refusal that still wrote to the
// database would fail here.
func TestBillingService_Cancel_RefusesTerminalStatus(t *testing.T) {
	t.Parallel()
	f := setupTest(t)
	billingService := getBillingService(f)

	tenant, err := composables.UseTenantID(f.Ctx)
	require.NoError(t, err)

	for _, status := range []billing.Status{
		billing.Refunded,
		billing.PartiallyRefunded,
		billing.Expired,
		billing.Failed,
		billing.Completed,
	} {
		t.Run(string(status), func(t *testing.T) {
			created, err := billingService.Create(f.Ctx, &services.CreateTransactionCommand{
				TenantID: tenant,
				Quantity: 1000,
				Currency: billing.UZS,
				Gateway:  billing.Click,
				Details:  details.NewClickDetails(fmt.Sprintf("terminal_%s", status)),
			})
			require.NoError(t, err, "Create should succeed")

			terminal, err := billingService.Save(f.Ctx, created.SetStatus(status))
			require.NoError(t, err, "Save should persist the terminal status")
			require.Equal(t, status, terminal.Status())

			_, err = billingService.Cancel(f.Ctx, &services.CancelTransactionCommand{
				TransactionID: terminal.ID(),
			})
			require.ErrorIs(t, err, services.ErrTransactionNotCancellable)

			var wrapped *serrors.Error
			require.ErrorAs(t, err, &wrapped, "the refusal carries its operation")
			assert.Equal(t, serrors.Op("BillingService.Cancel"), serrors.OpOf(wrapped))

			stored, err := billingService.GetByID(f.Ctx, terminal.ID())
			require.NoError(t, err)
			assert.Equal(t, status, stored.Status(), "a refused cancellation must leave the stored status untouched")
		})
	}
}

// TestBillingService_Cancel_RefusesRepeatCancellation asserts that cancelling a
// transaction twice is refused rather than treated as a no-op, and that the
// refusal publishes no event — the second call has no status change to announce.
//
// This test is falsely green if the event assertion counts events from another
// test sharing the bus; it filters by transaction id, so only events about this
// transaction can satisfy or break it.
func TestBillingService_Cancel_RefusesRepeatCancellation(t *testing.T) {
	t.Parallel()
	f := setupTest(t)
	billingService := getBillingService(f)

	tenant, err := composables.UseTenantID(f.Ctx)
	require.NoError(t, err)

	created, err := billingService.Create(f.Ctx, &services.CreateTransactionCommand{
		TenantID: tenant,
		Quantity: 1000,
		Currency: billing.UZS,
		Gateway:  billing.Click,
		Details:  details.NewClickDetails("repeat_cancel"),
	})
	require.NoError(t, err, "Create should succeed")

	canceled, err := billingService.Cancel(f.Ctx, &services.CancelTransactionCommand{
		TransactionID: created.ID(),
	})
	require.NoError(t, err, "the first cancellation of an active transaction should succeed")
	require.Equal(t, billing.Canceled, canceled.Status())

	var (
		mu       sync.Mutex
		observed int
	)
	unsubscribe := f.App.EventPublisher().Subscribe(func(event *billing.UpdatedEvent) {
		if event.Result == nil || event.Result.ID() != created.ID() {
			return
		}
		mu.Lock()
		observed++
		mu.Unlock()
	})
	defer unsubscribe()

	_, err = billingService.Cancel(f.Ctx, &services.CancelTransactionCommand{
		TransactionID: created.ID(),
	})
	require.ErrorIs(t, err, services.ErrTransactionNotCancellable)

	mu.Lock()
	defer mu.Unlock()
	assert.Zero(t, observed, "a refused cancellation must not publish an update event")
}

// rendezvousProvider stands in for a gateway. Each call waits until a second
// call arrives or the wait elapses, so callers racing past an unlocked status
// check are guaranteed to overlap here, while a caller serialized by the row
// lock simply runs out the wait.
type rendezvousProvider struct {
	gateway billing.Gateway
	wait    time.Duration
	calls   atomic.Int32
	second  chan struct{}
}

func newRendezvousProvider(gateway billing.Gateway, wait time.Duration) *rendezvousProvider {
	return &rendezvousProvider{gateway: gateway, wait: wait, second: make(chan struct{})}
}

func (p *rendezvousProvider) meet() {
	if p.calls.Add(1) == 2 {
		close(p.second)
	}
	select {
	case <-p.second:
	case <-time.After(p.wait):
	}
}

func (p *rendezvousProvider) Gateway() billing.Gateway { return p.gateway }

func (p *rendezvousProvider) Create(_ context.Context, t billing.Transaction) (billing.Transaction, error) {
	return t, nil
}

func (p *rendezvousProvider) Cancel(_ context.Context, t billing.Transaction) (billing.Transaction, error) {
	p.meet()
	return t.SetStatus(billing.Canceled), nil
}

func (p *rendezvousProvider) Refund(_ context.Context, t billing.Transaction, _ float64) (billing.Transaction, error) {
	p.meet()
	return t.SetStatus(billing.Refunded), nil
}

// setupCommittedTest commits the scope's seed data, so that the tenant is visible
// to the independent transactions these tests open from the pool.
func setupCommittedTest(t *testing.T) *itf.TestEnvironment {
	t.Helper()

	f := setupTest(t)
	f.CommitTx(t)
	return f
}

func poolContext(f *itf.TestEnvironment) context.Context {
	return composables.WithPool(context.Background(), f.Pool)
}

// seedTransaction commits a transaction so that callers on other connections
// can see and lock it.
func seedTransaction(t *testing.T, f *itf.TestEnvironment, status billing.Status, gateway billing.Gateway, d details.Details) billing.Transaction {
	t.Helper()

	var saved billing.Transaction
	err := composables.InTx(poolContext(f), func(txCtx context.Context) error {
		var err error
		saved, err = persistence.NewBillingRepository().Save(
			txCtx,
			billing.New(1000, billing.UZS, gateway, d, billing.WithTenantID(f.TenantID())).SetStatus(status),
		)
		return err
	})
	require.NoError(t, err)
	return saved
}

func newServiceWithProvider(f *itf.TestEnvironment, provider billing.Provider) *services.BillingService {
	return services.NewBillingService(
		persistence.NewBillingRepository(),
		[]billing.Provider{provider},
		f.App.EventPublisher(),
	)
}

// raceTwice runs call twice at once, each on its own connection from the pool,
// and returns both errors.
func raceTwice(f *itf.TestEnvironment, call func(ctx context.Context) error) [2]error {
	var (
		wg   sync.WaitGroup
		errs [2]error
	)
	for i := range errs {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs[i] = call(poolContext(f))
		}()
	}
	wg.Wait()
	return errs
}

// assertOneWinner requires one call to succeed and the other to be refused with
// refusal, whichever order they ran in.
func assertOneWinner(t *testing.T, errs [2]error, refusal error) {
	t.Helper()

	winner, loser := errs[0], errs[1]
	if winner != nil {
		winner, loser = loser, winner
	}
	require.NoError(t, winner)
	require.ErrorIs(t, loser, refusal)
}

// TestBillingService_Refund_RefusesStatusWithoutCapturedMoney pins the predicate
// to captured money rather than to !IsActive: Failed, Canceled and Expired are
// inactive but hold nothing to send back. It would be falsely green if the
// refusal came from the provider, so it also asserts the provider was never
// reached and the stored status survived.
func TestBillingService_Refund_RefusesStatusWithoutCapturedMoney(t *testing.T) {
	t.Parallel()
	f := setupCommittedTest(t)

	for _, status := range []billing.Status{
		billing.Created,
		billing.Pending,
		billing.Failed,
		billing.Canceled,
		billing.Expired,
		billing.Refunded,
	} {
		t.Run(string(status), func(t *testing.T) {
			provider := newRendezvousProvider(billing.Click, 0)
			billingService := newServiceWithProvider(f, provider)
			transaction := seedTransaction(t, f, status, billing.Click, details.NewClickDetails(string(status)))

			_, err := billingService.Refund(poolContext(f), &services.RefundTransactionCommand{
				TransactionID: transaction.ID(),
				Quantity:      1000,
			})
			require.ErrorIs(t, err, services.ErrTransactionNotRefundable)

			var wrapped *serrors.Error
			require.ErrorAs(t, err, &wrapped, "the refusal carries its operation")
			assert.Equal(t, serrors.Op("BillingService.Refund"), serrors.OpOf(wrapped))
			assert.Zero(t, provider.calls.Load(), "a refused refund must not reach the gateway")

			stored, err := billingService.GetByID(poolContext(f), transaction.ID())
			require.NoError(t, err)
			assert.Equal(t, status, stored.Status())
		})
	}
}

func TestBillingService_Refund_RefusesInvalidAmount(t *testing.T) {
	t.Parallel()
	f := setupCommittedTest(t)

	tests := []struct {
		name     string
		details  details.Details
		gateway  billing.Gateway
		quantity float64
	}{
		{name: "zero", gateway: billing.Click, details: details.NewClickDetails("zero"), quantity: 0},
		{name: "negative", gateway: billing.Click, details: details.NewClickDetails("negative"), quantity: -1},
		{name: "NaN", gateway: billing.Click, details: details.NewClickDetails("nan"), quantity: math.NaN()},
		{name: "infinite", gateway: billing.Click, details: details.NewClickDetails("inf"), quantity: math.Inf(1)},
		{name: "above total", gateway: billing.Click, details: details.NewClickDetails("above"), quantity: 1000.5},
		{
			name:     "above what remains",
			gateway:  billing.Octo,
			details:  details.NewOctoDetails("remains", details.OctoWithRefundedSum(600)),
			quantity: 500,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := newRendezvousProvider(tt.gateway, 0)
			billingService := newServiceWithProvider(f, provider)
			transaction := seedTransaction(t, f, billing.PartiallyRefunded, tt.gateway, tt.details)

			_, err := billingService.Refund(poolContext(f), &services.RefundTransactionCommand{
				TransactionID: transaction.ID(),
				Quantity:      tt.quantity,
			})
			require.ErrorIs(t, err, services.ErrInvalidRefundAmount)
			assert.Zero(t, provider.calls.Load(), "a refused refund must not reach the gateway")
		})
	}
}

func TestBillingService_Refund_AllowsCapturedMoney(t *testing.T) {
	t.Parallel()
	f := setupCommittedTest(t)

	t.Run("up to what remains", func(t *testing.T) {
		provider := newRendezvousProvider(billing.Octo, 0)
		transaction := seedTransaction(t, f, billing.PartiallyRefunded, billing.Octo,
			details.NewOctoDetails("remaining", details.OctoWithRefundedSum(600)))

		refunded, err := newServiceWithProvider(f, provider).Refund(poolContext(f), &services.RefundTransactionCommand{
			TransactionID: transaction.ID(),
			Quantity:      400,
		})
		require.NoError(t, err)
		assert.Equal(t, billing.Refunded, refunded.Status())
		assert.EqualValues(t, 1, provider.calls.Load())
	})

	// Cash records no refunded sum, so the ceiling stays the transaction total and
	// the requested amount alone decides the status.
	t.Run("details-only gateway partially then in full", func(t *testing.T) {
		billingService := getBillingService(f)
		transaction := seedTransaction(t, f, billing.Completed, billing.Cash, details.NewCashDetails())

		partial, err := billingService.Refund(poolContext(f), &services.RefundTransactionCommand{
			TransactionID: transaction.ID(),
			Quantity:      400,
		})
		require.NoError(t, err)
		assert.Equal(t, billing.PartiallyRefunded, partial.Status())

		full, err := billingService.Refund(poolContext(f), &services.RefundTransactionCommand{
			TransactionID: transaction.ID(),
			Quantity:      1000,
		})
		require.NoError(t, err)
		assert.Equal(t, billing.Refunded, full.Status())
	})
}

// TestBillingService_Refund_ConcurrentCallsSendMoneyOnce is the double-refund
// race: without the row lock both callers pass the status check on the same
// snapshot and the rendezvous holds them inside the gateway together, so the
// provider counts two refunds.
func TestBillingService_Refund_ConcurrentCallsSendMoneyOnce(t *testing.T) {
	t.Parallel()
	f := setupCommittedTest(t)

	provider := newRendezvousProvider(billing.Click, 500*time.Millisecond)
	billingService := newServiceWithProvider(f, provider)
	transaction := seedTransaction(t, f, billing.Completed, billing.Click, details.NewClickDetails("double_refund"))

	errs := raceTwice(f, func(ctx context.Context) error {
		_, err := billingService.Refund(ctx, &services.RefundTransactionCommand{
			TransactionID: transaction.ID(),
			Quantity:      1000,
		})
		return err
	})

	assertOneWinner(t, errs, services.ErrTransactionNotRefundable)
	assert.EqualValues(t, 1, provider.calls.Load(), "the gateway must be asked for the refund once")
}

func TestBillingService_Cancel_ConcurrentCallsReachGatewayOnce(t *testing.T) {
	t.Parallel()
	f := setupCommittedTest(t)

	provider := newRendezvousProvider(billing.Click, 500*time.Millisecond)
	billingService := newServiceWithProvider(f, provider)
	transaction := seedTransaction(t, f, billing.Created, billing.Click, details.NewClickDetails("double_cancel"))

	errs := raceTwice(f, func(ctx context.Context) error {
		_, err := billingService.Cancel(ctx, &services.CancelTransactionCommand{TransactionID: transaction.ID()})
		return err
	})

	assertOneWinner(t, errs, services.ErrTransactionNotCancellable)
	assert.EqualValues(t, 1, provider.calls.Load(), "the gateway must be asked for the cancellation once")
}

// TestBillingService_Delete_ReadsUnderTheRowLock holds the row while a status
// change is in flight. Reading before the lock would make Delete report the
// status from before that change; it can only be falsely green if Delete has
// not started by the time the change commits, never falsely red.
func TestBillingService_Delete_ReadsUnderTheRowLock(t *testing.T) {
	t.Parallel()
	f := setupCommittedTest(t)

	billingService := getBillingService(f)
	repo := persistence.NewBillingRepository()
	transaction := seedTransaction(t, f, billing.Completed, billing.Click, details.NewClickDetails("delete_lock"))
	poolCtx := poolContext(f)

	deleted := make(chan billing.Transaction, 1)
	err := composables.InTx(poolCtx, func(txCtx context.Context) error {
		locked, err := repo.GetByIDForUpdate(txCtx, transaction.ID())
		if err != nil {
			return err
		}
		if _, err := repo.Save(txCtx, locked.SetStatus(billing.Refunded)); err != nil {
			return err
		}

		go func() {
			result, deleteErr := billingService.Delete(poolCtx, transaction.ID())
			if deleteErr != nil {
				t.Errorf("Delete: %v", deleteErr)
			}
			deleted <- result
		}()
		time.Sleep(300 * time.Millisecond)
		return nil
	})
	require.NoError(t, err)

	result := <-deleted
	require.NotNil(t, result)
	assert.Equal(t, billing.Refunded, result.Status(), "Delete must report the row as it was when it took the lock")
}

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

	// Test passes if we reach here without deadlock or panic
}

func TestBillingService_InvokeCallback_WithPanic(t *testing.T) {
	t.Helper()
	t.Parallel()

	tests := []struct {
		name        string
		callback    billing.TransactionCallback
		wantErrText string
	}{
		{
			name: "PanicWithString",
			callback: func(ctx context.Context, tx billing.Transaction) error {
				panic("something went wrong")
			},
			wantErrText: "callback panic: something went wrong",
		},
		{
			name: "PanicWithError",
			callback: func(ctx context.Context, tx billing.Transaction) error {
				panic(errors.New("custom error occurred"))
			},
			wantErrText: "callback panic: custom error occurred",
		},
		{
			name: "PanicWithInt",
			callback: func(ctx context.Context, tx billing.Transaction) error {
				panic(42)
			},
			wantErrText: "callback panic: 42",
		},
		{
			name: "PanicWithNil",
			callback: func(ctx context.Context, tx billing.Transaction) error {
				panic(nil)
			},
			wantErrText: "callback panic: panic called with nil argument",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			f := setupTest(t)
			billingService := getBillingService(f)

			billingService.RegisterCallback(tt.callback)

			transaction := billing.New(
				100.0,
				billing.UZS,
				billing.Click,
				details.NewClickDetails("test-123"),
			)

			err := billingService.InvokeCallback(f.Ctx, transaction)
			require.Error(t, err, "Expected error from panicking callback")
			assert.Contains(t, err.Error(), "callback panic:", "Error should contain 'callback panic:' prefix")
			assert.Equal(t, tt.wantErrText, err.Error(), "Error message should match expected format")
		})
	}
}
