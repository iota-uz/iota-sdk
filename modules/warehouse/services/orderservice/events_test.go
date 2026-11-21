package orderservice_test

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"sync"
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

	// Complete publishes the native event in the transaction.
	row = commitsAndRollsBack(t, ctx, tenant, orderservice.OrderCompletedEventID, func(ctx context.Context) {
		_, err := service.Complete(ctx, id)
		require.NoError(t, err)
	})
	require.Equal(t, strconv.FormatUint(uint64(id), 10), row.Subject)
	require.JSONEq(t, `{"orderId":"`+row.Subject+`","type":"in"}`, string(row.Payload))

	stored, err := persistence.NewOrderRepository(persistence.NewProductRepository()).GetByID(ctx, id)
	require.NoError(t, err)
	require.Equal(t, order.Complete, stored.Status())
	require.Empty(t, stored.Events(), "an order read back from storage carries no events")
}

// committedOutbox counts the committed rows of the event.
func committedOutbox(t *testing.T, env *itf.TestEnvironment, event string) int {
	t.Helper()
	var n int
	require.NoError(t, env.Pool.QueryRow(context.Background(),
		`SELECT count(*) FROM extruntime.outbox_events WHERE tenant_id = $1 AND event = $2`, env.TenantID(), event).Scan(&n))
	return n
}

// The eventbus events of the order aggregate are published when the transaction
// that holds the change commits: a caller's transaction that rolls back after
// the service returned publishes none, and a concurrent second completion of
// one order is refused instead of announcing the transition twice.
func TestOrderEvents_EventbusFollowsTheOwnersCommit(t *testing.T) {
	env := itf.Setup(t,
		itf.WithComponents(modules.Components()...),
		itf.WithUser(itf.User(permissions.OrderCreate, permissions.OrderRead, permissions.OrderUpdate)),
	)
	env.User = user.New("", "", env.User.Email(), "", user.WithID(env.User.ID()), user.WithRoles(env.User.Roles()), user.WithTenantID(env.TenantID()))
	env.Ctx = composables.WithUser(env.Ctx, env.User)
	service := itf.GetService[orderservice.OrderService](env)
	bus, err := composition.Resolve[eventbus.EventBus](env.Container)
	require.NoError(t, err)
	var mu sync.Mutex
	var created []order.OrderCreatedEvent
	var completed []order.OrderCompletedEvent
	defer bus.Subscribe(func(e order.OrderCreatedEvent) { mu.Lock(); created = append(created, e); mu.Unlock() })()
	defer bus.Subscribe(func(e order.OrderCompletedEvent) { mu.Lock(); completed = append(completed, e); mu.Unlock() })()
	count := func() (int, int) { mu.Lock(); defer mu.Unlock(); return len(created), len(completed) }
	env.CommitTx(t)

	ctx := itf.TestExecution(composables.WithUser(composables.WithTenantID(composables.WithPool(context.Background(), env.Pool), env.TenantID()), env.User))
	errRollback := errors.New("the caller's later step failed")

	// The caller's transaction rolls back after Create returned: nothing is
	// announced, neither natively nor on the eventbus.
	err = composables.InTx(ctx, func(txCtx context.Context) error {
		require.NoError(t, service.Create(txCtx, order.CreateDTO{Type: "in", Status: "pending"}))
		c, _ := count()
		require.Zero(t, c, "the eventbus event waits for the caller's commit")
		return errRollback
	})
	require.ErrorIs(t, err, errRollback)
	c, _ := count()
	require.Zero(t, c)
	require.Zero(t, committedOutbox(t, env, orderservice.OrderCreatedEventID))

	// The caller's transaction commits: now the event is published, once.
	err = composables.InTx(ctx, func(txCtx context.Context) error {
		return service.Create(txCtx, order.CreateDTO{Type: "in", Status: "pending"})
	})
	require.NoError(t, err)
	c, _ = count()
	require.Equal(t, 1, c)
	require.Equal(t, 1, committedOutbox(t, env, orderservice.OrderCreatedEventID))
	orders, err := service.GetAll(ctx)
	require.NoError(t, err)
	require.Len(t, orders, 1)
	id := orders[0].ID()

	// A rolled back completion announces nothing.
	err = composables.InTx(ctx, func(txCtx context.Context) error {
		_, err := service.Complete(txCtx, id)
		require.NoError(t, err)
		return errRollback
	})
	require.ErrorIs(t, err, errRollback)
	_, done := count()
	require.Zero(t, done)
	require.Zero(t, committedOutbox(t, env, orderservice.OrderCompletedEventID))

	// Two concurrent completions of one order: one wins, the other is refused,
	// and the transition is announced once, natively and on the eventbus.
	var wg sync.WaitGroup
	errs := make([]error, 2)
	for i := range errs {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, errs[i] = service.Complete(ctx, id)
		}()
	}
	wg.Wait()
	failed := 0
	for _, e := range errs {
		if e != nil {
			failed++
		}
	}
	require.Equal(t, 1, failed, "exactly one of the concurrent completions is refused: %v", errs)
	_, done = count()
	require.Equal(t, 1, done)
	require.Equal(t, 1, committedOutbox(t, env, orderservice.OrderCompletedEventID))
}
