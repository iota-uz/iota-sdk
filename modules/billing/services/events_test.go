package services_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/modules"
	"github.com/iota-uz/iota-sdk/modules/billing/domain/aggregates/billing"
	"github.com/iota-uz/iota-sdk/modules/billing/domain/aggregates/details"
	"github.com/iota-uz/iota-sdk/modules/billing/services"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/constants"
	"github.com/iota-uz/iota-sdk/pkg/itf"
)

type outboxRow struct {
	Subject   string
	Initiator *int64
	Payload   json.RawMessage
}

// outboxRows are the rows of the event in the transaction held by ctx, oldest
// first: the rows the transaction wrote are visible to it before they commit.
func outboxRows(t *testing.T, ctx context.Context, tenantID uuid.UUID, event string) []outboxRow {
	t.Helper()
	tx, err := composables.UseTx(ctx)
	require.NoError(t, err)
	rows, err := tx.Query(ctx, `SELECT subject_key, initiator_user_id, payload FROM extruntime.outbox_events WHERE tenant_id = $1 AND event = $2 ORDER BY seq`, tenantID, event)
	require.NoError(t, err)
	defer rows.Close()
	var out []outboxRow
	for rows.Next() {
		var r outboxRow
		require.NoError(t, rows.Scan(&r.Subject, &r.Initiator, &r.Payload))
		out = append(out, r)
	}
	require.NoError(t, rows.Err())
	return out
}

// commitsAndRollsBack runs act in a savepoint, sees the event in it, rolls the
// savepoint back and sees the event gone, then runs act in the test
// transaction itself and returns the event it published.
func commitsAndRollsBack(t *testing.T, ctx context.Context, tenantID uuid.UUID, event string, act func(ctx context.Context)) outboxRow {
	t.Helper()
	before := len(outboxRows(t, ctx, tenantID, event))
	tx, err := composables.UseTx(ctx)
	require.NoError(t, err)
	savepoint, err := tx.(pgx.Tx).Begin(ctx)
	require.NoError(t, err)
	spCtx := composables.WithTx(ctx, savepoint)
	act(spCtx)
	require.Len(t, outboxRows(t, spCtx, tenantID, event), before+1, "the event is in the transaction of the change")
	require.NoError(t, savepoint.Rollback(ctx))
	require.Len(t, outboxRows(t, ctx, tenantID, event), before, "rolling the change back takes its event with it")

	act(ctx)
	rows := outboxRows(t, ctx, tenantID, event)
	require.Len(t, rows, before+1, "committing the change keeps its event")
	return rows[len(rows)-1]
}

func statusPayload(t *testing.T, row outboxRow) services.TransactionStatusChangedPayload {
	t.Helper()
	var p services.TransactionStatusChangedPayload
	require.NoError(t, json.Unmarshal(row.Payload, &p))
	return p
}

func newTransaction(t *testing.T, env *itf.TestEnvironment, ctx context.Context, name string) billing.Transaction {
	t.Helper()
	created, err := getBillingService(env).Create(ctx, &services.CreateTransactionCommand{
		TenantID: env.TenantID(), Quantity: 1000, Currency: billing.UZS, Gateway: billing.Click,
		Details: details.NewClickDetails(name),
	})
	require.NoError(t, err)
	return created
}

// The status change event commits with the status the gateway callback saved
// and rolls back with it; a repeated callback with the same status publishes
// nothing, and a callback has no initiator.
func TestBillingEvents_StatusChangeCommitsAndRollsBackWithTheTransaction(t *testing.T) {
	env := itf.Setup(t, itf.WithComponents(modules.Components()...), itf.WithUser(itf.User()))
	// The acting user belongs to the tenant, as a signed-in user does.
	env.User = user.New("", "", env.User.Email(), "", user.WithID(env.User.ID()), user.WithRoles(env.User.Roles()), user.WithTenantID(env.TenantID()))
	env.Ctx = composables.WithUser(env.Ctx, env.User)
	service := getBillingService(env)
	ctx := itf.TestExecution(env.Ctx)
	tenant := env.TenantID()
	created := newTransaction(t, env, ctx, "status_change")
	require.Empty(t, outboxRows(t, ctx, tenant, services.TransactionStatusChangedEventID), "creating a transaction is no status change")

	// A gateway callback has no acting user.
	callback := context.WithValue(ctx, constants.UserKey, nil)
	row := commitsAndRollsBack(t, callback, tenant, services.TransactionStatusChangedEventID, func(ctx context.Context) {
		saved, err := service.Save(ctx, created.SetStatus(billing.Completed))
		require.NoError(t, err)
		require.Equal(t, billing.Completed, saved.Status())
	})
	require.Equal(t, created.ID().String(), row.Subject)
	require.Nil(t, row.Initiator, "a gateway callback has no initiator")
	require.JSONEq(t, `{"transactionId":"`+created.ID().String()+`","gateway":"click","from":"created","to":"completed","amount":{"amount":"100000","currency":"UZS"}}`, string(row.Payload))

	// The same callback again: the status does not change, nothing is published.
	_, err := service.Save(callback, created.SetStatus(billing.Completed))
	require.NoError(t, err)
	require.Len(t, outboxRows(t, ctx, tenant, services.TransactionStatusChangedEventID), 1)

	// Cancel and Refund save through the same path.
	other := newTransaction(t, env, ctx, "cancel_path")
	row = commitsAndRollsBack(t, ctx, tenant, services.TransactionStatusChangedEventID, func(ctx context.Context) {
		_, err := service.Cancel(ctx, &services.CancelTransactionCommand{TransactionID: other.ID()})
		require.NoError(t, err)
	})
	p := statusPayload(t, row)
	require.Equal(t, other.ID(), p.TransactionID)
	require.Equal(t, "created", p.From)
	require.Equal(t, "canceled", p.To)
	require.NotNil(t, row.Initiator, "the user who cancelled is the initiator")

	// Cash has no provider: its refund is a bare status write.
	refunded, err := service.Create(ctx, &services.CreateTransactionCommand{
		TenantID: tenant, Quantity: 1000, Currency: billing.UZS, Gateway: billing.Cash, Details: details.NewCashDetails(),
	})
	require.NoError(t, err)
	_, err = service.Save(callback, refunded.SetStatus(billing.Completed))
	require.NoError(t, err)
	row = commitsAndRollsBack(t, ctx, tenant, services.TransactionStatusChangedEventID, func(ctx context.Context) {
		_, err := service.Refund(ctx, &services.RefundTransactionCommand{TransactionID: refunded.ID(), Quantity: 400})
		require.NoError(t, err)
	})
	p = statusPayload(t, row)
	require.Equal(t, "completed", p.From)
	require.Equal(t, "partially-refunded", p.To)
}
