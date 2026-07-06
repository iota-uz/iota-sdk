package controllers

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/billing/domain/aggregates/billing"
	"github.com/iota-uz/iota-sdk/modules/billing/domain/aggregates/details"
	"github.com/iota-uz/iota-sdk/modules/billing/services"
	"github.com/iota-uz/iota-sdk/pkg/constants"
	paymeapi "github.com/iota-uz/payme"
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

	ctx := context.WithValue(context.Background(), constants.TxKey, struct{}{})
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

	ctx := context.WithValue(context.Background(), constants.TxKey, struct{}{})
	resp, errRPC := controller.checkPerform(ctx, &paymeapi.CheckPerformTransactionRequest{
		Amount:  12500,
		Account: account,
	}, logrus.New().WithField("test", true))

	require.Nil(t, errRPC)
	require.NotNil(t, resp)
	assert.True(t, resp.Allow)

	require.NotNil(t, saved)
	assert.Equal(t, billing.Pending, saved.Status())

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

	ctx := context.WithValue(context.Background(), constants.TxKey, struct{}{})
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

	ctx := context.WithValue(context.Background(), constants.TxKey, struct{}{})
	resp, errRPC := controller.checkPerform(ctx, &paymeapi.CheckPerformTransactionRequest{
		Amount:  12500,
		Account: account,
	}, logrus.New().WithField("test", true))

	require.Nil(t, resp)
	require.NotNil(t, errRPC)
	assert.Equal(t, paymeapi.CheckPerformTransactionInvalidAccountError().Code, errRPC.Code)
}

func newTestPaymeController(
	t *testing.T,
	account map[string]any,
	transactions []billing.Transaction,
	onSave func(billing.Transaction),
) *PaymeController {
	t.Helper()

	repo := &testBillingRepo{
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
		billing.WithStatus(status),
	)
}
