package controllers

import (
	"context"
	"strings"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/billing/domain/aggregates/billing"
	"github.com/iota-uz/iota-sdk/modules/billing/domain/aggregates/details"
	"github.com/iota-uz/iota-sdk/modules/billing/services"
	"github.com/iota-uz/iota-sdk/pkg/constants"
	"github.com/iota-uz/iota-sdk/pkg/repo"
	paymeapi "github.com/iota-uz/payme"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPaymeController_Create_UsesActiveTransactionWhenTerminalRowsMatchAccount(t *testing.T) {
	t.Parallel()

	account := map[string]any{"order_id": "order-123"}
	terminal := newPaymeControllerTestTransaction(
		billing.Failed,
		"failed-row",
		details.PaymeWithAccount(account),
		details.PaymeWithState(paymeapi.TransactionStateCancelledBeforeCompletion),
	)
	active := newPaymeControllerTestTransaction(
		billing.Created,
		"created-row",
		details.PaymeWithAccount(account),
		details.PaymeWithCreatedTime(1710000000000),
	)
	var saved billing.Transaction

	controller := newTestPaymeController(t, account, []billing.Transaction{terminal, active}, func(tx billing.Transaction) {
		saved = tx
	})

	ctx, outbox := newOutboxCtx()
	resp, errRPC := controller.create(ctx, &paymeapi.CreateTransactionRequest{
		Id:      "payme-tx-1",
		Time:    1710000001000,
		Amount:  12500,
		Account: account,
	}, logrus.New().WithField("test", true))

	require.Nil(t, errRPC)
	require.NotNil(t, resp)
	assert.Equal(t, "created-row", resp.Transaction)
	assert.EqualValues(t, paymeapi.TransactionStateCreated, resp.State)
	assert.Equal(t, int64(1710000000000), resp.CreateTime)

	require.NotNil(t, saved)
	paymeDetails, ok := saved.Details().(details.PaymeDetails)
	require.True(t, ok)
	assert.Equal(t, "payme-tx-1", paymeDetails.ID())
	assert.Equal(t, billing.Created, saved.Status())
	assert.Zero(t, outbox.events(), "a save that keeps the status announces nothing")
}

func TestPaymeController_CheckPerform_UsesActiveTransactionWhenTerminalRowsMatchAccount(t *testing.T) {
	t.Parallel()

	account := map[string]any{"order_id": "order-123"}
	terminal := newPaymeControllerTestTransaction(
		billing.Canceled,
		"canceled-row",
		details.PaymeWithAccount(account),
		details.PaymeWithState(paymeapi.TransactionStateCancelledBeforeCompletion),
	)
	active := newPaymeControllerTestTransaction(
		billing.Created,
		"created-row",
		details.PaymeWithAccount(account),
	)
	var saved billing.Transaction

	controller := newTestPaymeController(t, account, []billing.Transaction{terminal, active}, func(tx billing.Transaction) {
		saved = tx
	})

	ctx, outbox := newOutboxCtx()
	resp, errRPC := controller.checkPerform(ctx, &paymeapi.CheckPerformTransactionRequest{
		Amount:  12500,
		Account: account,
	}, logrus.New().WithField("test", true))

	require.Nil(t, errRPC)
	require.NotNil(t, resp)
	assert.True(t, resp.Allow)

	require.NotNil(t, saved)
	assert.Equal(t, billing.Pending, saved.Status())
	assert.Equal(t, 1, outbox.events(), "the created -> pending change is announced in the saving transaction")

	paymeDetails, ok := saved.Details().(details.PaymeDetails)
	require.True(t, ok)
	assert.Equal(t, "created-row", paymeDetails.Transaction())
	assert.Equal(t, account, paymeDetails.Account())
}

func TestPaymeController_Create_RejectsDuplicateActiveTransactions(t *testing.T) {
	t.Parallel()

	account := map[string]any{"order_id": "order-123"}
	controller := newTestPaymeController(t, account, []billing.Transaction{
		newPaymeControllerTestTransaction(
			billing.Created,
			"created-row",
			details.PaymeWithAccount(account),
		),
		newPaymeControllerTestTransaction(
			billing.Pending,
			"pending-row",
			details.PaymeWithAccount(account),
		),
	}, nil)

	ctx, _ := newOutboxCtx()
	resp, errRPC := controller.create(ctx, &paymeapi.CreateTransactionRequest{
		Id:      "payme-tx-1",
		Time:    1710000001000,
		Amount:  12500,
		Account: account,
	}, logrus.New().WithField("test", true))

	require.Nil(t, resp)
	require.NotNil(t, errRPC)
	assert.Equal(t, paymeapi.InvalidAccountError().Code, errRPC.Code)
}

func TestPaymeController_CheckPerform_RejectsDuplicateActiveTransactions(t *testing.T) {
	t.Parallel()

	account := map[string]any{"order_id": "order-123"}
	controller := newTestPaymeController(t, account, []billing.Transaction{
		newPaymeControllerTestTransaction(
			billing.Created,
			"created-row",
			details.PaymeWithAccount(account),
		),
		newPaymeControllerTestTransaction(
			billing.Pending,
			"pending-row",
			details.PaymeWithAccount(account),
		),
	}, nil)

	ctx, _ := newOutboxCtx()
	resp, errRPC := controller.checkPerform(ctx, &paymeapi.CheckPerformTransactionRequest{
		Amount:  12500,
		Account: account,
	}, logrus.New().WithField("test", true))

	require.Nil(t, resp)
	require.NotNil(t, errRPC)
	assert.Equal(t, paymeapi.CheckPerformTransactionInvalidAccountError().Code, errRPC.Code)
}

// A customer can hold a checkout open across a cancellation: they opened their
// transaction while the link was live and only pressed pay afterwards. Payme
// resolves a perform by transaction id and never consults its state, so the
// call arrives regardless — refusing it is the merchant's job, and answering
// anything but an error means money accepted for an order that has moved on.
func TestPaymeController_Perform_RefusesACancelledTransaction(t *testing.T) {
	t.Parallel()

	cancelled := newPaymeControllerTestTransaction(
		billing.Canceled,
		"cancelled-row",
		details.PaymeWithID("payme-tx-1"),
		details.PaymeWithState(paymeapi.TransactionStateCancelledBeforeCompletion),
		details.PaymeWithReason(paymeapi.CancelReasonDebitError),
	)
	controller := newTestPaymePerformController(t, "payme-tx-1", cancelled, func(context.Context, billing.Transaction) error {
		t.Fatal("a cancelled transaction must never reach the host callback")
		return nil
	}, func(billing.Transaction) {
		t.Fatal("refusing a perform must not rewrite the transaction")
	})

	ctx, _ := newOutboxCtx()
	resp, errRPC := controller.perform(ctx, &paymeapi.PerformTransactionRequest{
		Id: "payme-tx-1",
	}, logrus.New().WithField("test", true))

	require.Nil(t, resp)
	require.NotNil(t, errRPC)
	assert.Equal(t, paymeapi.PerformTransactionOperationNotAllowedError().Code, errRPC.Code)
}

// The other half of the same rule: a perform Payme repeats — because our answer
// was lost, not because anything changed — must still be honoured, or Payme
// keeps retrying a payment it has already taken.
func TestPaymeController_Perform_StaysIdempotentOnACompletedTransaction(t *testing.T) {
	t.Parallel()

	completed := newPaymeControllerTestTransaction(
		billing.Completed,
		"completed-row",
		details.PaymeWithID("payme-tx-1"),
		details.PaymeWithState(paymeapi.TransactionStateCompleted),
		details.PaymeWithPerformTime(1710000002000),
	)
	var invoked bool
	controller := newTestPaymePerformController(t, "payme-tx-1", completed, func(context.Context, billing.Transaction) error {
		invoked = true
		return nil
	}, nil)

	ctx, outbox := newOutboxCtx()
	resp, errRPC := controller.perform(ctx, &paymeapi.PerformTransactionRequest{
		Id: "payme-tx-1",
	}, logrus.New().WithField("test", true))

	require.Nil(t, errRPC)
	require.NotNil(t, resp)
	assert.True(t, invoked)
	assert.Equal(t, "completed-row", resp.Transaction)
	assert.EqualValues(t, paymeapi.TransactionStateCompleted, resp.State)
	assert.Equal(t, int64(1710000002000), resp.PerformTime)
	assert.Zero(t, outbox.events(), "a repeated callback with the same status announces nothing")
}

func newTestPaymePerformController(
	t *testing.T,
	transactionID string,
	transaction billing.Transaction,
	callback billing.TransactionCallback,
	onSave func(billing.Transaction),
) *PaymeController {
	t.Helper()

	repo := &testBillingRepo{
		stored: []billing.Transaction{transaction},
		getByDetailsFields: func(_ context.Context, gateway billing.Gateway, filters []billing.DetailsFieldFilter) ([]billing.Transaction, error) {
			require.Equal(t, billing.Payme, gateway)
			require.Equal(t, []billing.DetailsFieldFilter{{
				Path:     []string{"id"},
				Operator: billing.OpEqual,
				Value:    transactionID,
			}}, filters)

			return []billing.Transaction{transaction}, nil
		},
		save: func(_ context.Context, tx billing.Transaction) (billing.Transaction, error) {
			if onSave != nil {
				onSave(tx)
			}
			return tx, nil
		},
	}

	billingService := services.NewBillingService(repo, nil, noopEventBus{})
	billingService.RegisterCallback(callback)

	return &PaymeController{billingService: billingService}
}

func newTestPaymeController(
	t *testing.T,
	account map[string]any,
	transactions []billing.Transaction,
	onSave func(billing.Transaction),
) *PaymeController {
	t.Helper()

	repo := &testBillingRepo{
		stored: transactions,
		getByDetailsFields: func(_ context.Context, gateway billing.Gateway, filters []billing.DetailsFieldFilter) ([]billing.Transaction, error) {
			require.Equal(t, billing.Payme, gateway)
			require.ElementsMatch(t, paymeAccountFilters(account), filters)

			return transactions, nil
		},
		save: func(_ context.Context, tx billing.Transaction) (billing.Transaction, error) {
			if onSave != nil {
				onSave(tx)
			}
			return tx, nil
		},
	}

	return &PaymeController{
		billingService: services.NewBillingService(repo, nil, noopEventBus{}),
	}
}

type noopEventBus struct{}

func (noopEventBus) Publish(...interface{}) {}

func (noopEventBus) Subscribe(interface{}) func() {
	return func() {}
}

func (noopEventBus) Clear() {}

func (noopEventBus) SubscribersCount() int {
	return 0
}

func paymeAccountFilters(account map[string]any) []billing.DetailsFieldFilter {
	filters := make([]billing.DetailsFieldFilter, 0, len(account))
	for k, v := range account {
		filters = append(filters, billing.DetailsFieldFilter{
			Path:     []string{"account", k},
			Operator: billing.OpEqual,
			Value:    v,
		})
	}
	return filters
}

func newPaymeControllerTestTransaction(
	status billing.Status,
	transactionID string,
	options ...details.PaymeOption,
) billing.Transaction {
	options = append([]details.PaymeOption{
		details.PaymeWithState(paymeapi.TransactionStateCreated),
	}, options...)

	return billing.New(
		125,
		billing.UZS,
		billing.Payme,
		details.NewPaymeDetails(transactionID, options...),
		billing.WithID(uuid.New()),
		billing.WithTenantID(uuid.New()),
		billing.WithStatus(status),
	)
}

// outboxTx is a transaction that only records what is written to the outbox.
type outboxTx struct {
	repo.Tx
	mu    sync.Mutex
	execs []string
}

func (tx *outboxTx) Exec(_ context.Context, sql string, _ ...any) (pgconn.CommandTag, error) {
	tx.mu.Lock()
	defer tx.mu.Unlock()
	tx.execs = append(tx.execs, sql)
	return pgconn.NewCommandTag("INSERT 0 1"), nil
}

// events is the number of rows written to extruntime.outbox_events.
func (tx *outboxTx) events() int {
	tx.mu.Lock()
	defer tx.mu.Unlock()
	n := 0
	for _, sql := range tx.execs {
		if strings.Contains(sql, "extruntime.outbox_events") {
			n++
		}
	}
	return n
}

func newOutboxCtx() (context.Context, *outboxTx) {
	tx := &outboxTx{}
	return context.WithValue(context.Background(), constants.TxKey, tx), tx
}
