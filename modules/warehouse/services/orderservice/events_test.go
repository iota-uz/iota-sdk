package orderservice_test

import (
	"context"
	"encoding/json"
	"strconv"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/modules"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/warehouse/domain/aggregates/order"
	"github.com/iota-uz/iota-sdk/modules/warehouse/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/modules/warehouse/permissions"
	"github.com/iota-uz/iota-sdk/modules/warehouse/services/orderservice"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/composition"
	"github.com/iota-uz/iota-sdk/pkg/eventbus"
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

func TestOrderEvents_CommitAndRollBackWithTheOrder(t *testing.T) {
	env := itf.Setup(t,
		itf.WithComponents(modules.Components()...),
		itf.WithUser(itf.User(permissions.OrderCreate, permissions.OrderRead, permissions.OrderUpdate)),
	)
	// The acting user belongs to the tenant, as a signed-in user does.
	env.User = user.New("", "", env.User.Email(), "", user.WithID(env.User.ID()), user.WithRoles(env.User.Roles()), user.WithTenantID(env.TenantID()))
	env.Ctx = composables.WithUser(env.Ctx, env.User)
	service := itf.GetService[orderservice.OrderService](env)
	bus, err := composition.Resolve[eventbus.EventBus](env.Container)
	require.NoError(t, err)
	var created []order.OrderCreatedEvent
	var completed []order.OrderCompletedEvent
	defer bus.Subscribe(func(e order.OrderCreatedEvent) { created = append(created, e) })()
	defer bus.Subscribe(func(e order.OrderCompletedEvent) { completed = append(completed, e) })()
	ctx := itf.TestExecution(env.Ctx)
	tenant := env.TenantID()

	row := commitsAndRollsBack(t, ctx, tenant, orderservice.OrderCreatedEventID, func(ctx context.Context) {
		require.NoError(t, service.Create(ctx, order.CreateDTO{Type: "in", Status: "pending"}))
	})
	orders, err := service.GetAll(ctx)
	require.NoError(t, err)
	require.NotEmpty(t, orders)
	id := orders[len(orders)-1].ID()
	require.Equal(t, strconv.FormatUint(uint64(id), 10), row.Subject)
	require.JSONEq(t, `{"orderId":"`+row.Subject+`","type":"in","status":"pending"}`, string(row.Payload))
	require.NotNil(t, row.Initiator)
	require.Len(t, created, 2, "the eventbus event of the aggregate is published after each committed creation, not dropped")
	require.Equal(t, order.TypeIn, created[1].Type)

	// Complete publishes the native event in the transaction and the events
	// the aggregate accumulated on the eventbus. Only the completion: the
	// order read back from storage reports no creation.
	row = commitsAndRollsBack(t, ctx, tenant, orderservice.OrderCompletedEventID, func(ctx context.Context) {
		_, err := service.Complete(ctx, id)
		require.NoError(t, err)
	})
	require.Equal(t, strconv.FormatUint(uint64(id), 10), row.Subject)
	require.JSONEq(t, `{"orderId":"`+row.Subject+`","type":"in"}`, string(row.Payload))
	require.Len(t, completed, 2)
	require.Equal(t, id, completed[1].OrderID)
	require.Len(t, created, 2, "completing an order does not announce its creation again")

	stored, err := persistence.NewOrderRepository(persistence.NewProductRepository()).GetByID(ctx, id)
	require.NoError(t, err)
	require.Equal(t, order.Complete, stored.Status())
	require.Empty(t, stored.Events(), "an order read back from storage carries no events")
}
