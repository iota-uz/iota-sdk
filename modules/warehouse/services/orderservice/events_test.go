package orderservice_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/modules"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/warehouse/domain/aggregates/order"
	"github.com/iota-uz/iota-sdk/modules/warehouse/domain/aggregates/position"
	"github.com/iota-uz/iota-sdk/modules/warehouse/domain/aggregates/product"
	"github.com/iota-uz/iota-sdk/modules/warehouse/domain/entities/unit"
	"github.com/iota-uz/iota-sdk/modules/warehouse/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/modules/warehouse/permissions"
	"github.com/iota-uz/iota-sdk/modules/warehouse/services/orderservice"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/composition"
	"github.com/iota-uz/iota-sdk/pkg/constants"
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
	// The test owns the savepoint, so it owns its commit hooks too.
	spCtx, _ := composables.WithCommitHooks(composables.WithTx(ctx, savepoint))
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
		if err := service.Create(txCtx, order.CreateDTO{Type: "in", Status: "pending"}); err != nil {
			return err
		}
		if c, _ := count(); c != 0 {
			return errors.New("announced before the caller's commit")
		}
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
		if _, err := service.Complete(txCtx, id); err != nil {
			return err
		}
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

// A service that promises an eventbus announcement after commit refuses to run
// in a transaction whose owner offers no commit hooks (opened with the pool and
// handed over with WithTx, or a savepoint begun by hand): it would save the
// change and never announce it. It refuses before it writes anything. The owner
// can attach hooks (WithCommitHooks), and InSavepoint nests with announcements:
// they move to the enclosing transaction on release and are dropped on rollback.
func TestOrderEvents_EventbusNeedsAnOwnerThatOffersCommitHooks(t *testing.T) {
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
	announced := 0
	defer bus.Subscribe(func(order.OrderCreatedEvent) { mu.Lock(); announced++; mu.Unlock() })()
	count := func() int { mu.Lock(); defer mu.Unlock(); return announced }
	env.CommitTx(t)
	ctx := itf.TestExecution(composables.WithUser(composables.WithTenantID(composables.WithPool(context.Background(), env.Pool), env.TenantID()), env.User))
	create := func(ctx context.Context) error {
		return service.Create(ctx, order.CreateDTO{Type: "in", Status: "pending"})
	}

	// pool.Begin + WithTx: refused, nothing written, whatever the owner does next.
	tx, err := env.Pool.Begin(ctx)
	require.NoError(t, err)
	defer func() { _ = tx.Rollback(context.WithoutCancel(ctx)) }() // frees the connection if an assertion fails
	require.ErrorIs(t, create(composables.WithTx(ctx, tx)), composables.ErrNoCommitHooks)
	require.NoError(t, tx.Commit(ctx))
	require.Zero(t, count())
	require.Zero(t, committedOutbox(t, env, orderservice.OrderCreatedEventID))
	orders, err := service.GetAll(ctx)
	require.NoError(t, err)
	require.Empty(t, orders, "the refused call changed nothing")

	// The same owner, once it attaches hooks and runs them after its commit.
	tx, err = env.Pool.Begin(ctx)
	require.NoError(t, err)
	defer func() { _ = tx.Rollback(context.WithoutCancel(ctx)) }()
	hookCtx, hooks := composables.WithCommitHooks(composables.WithTx(ctx, tx))
	require.NoError(t, create(hookCtx))
	require.Zero(t, count())
	require.NoError(t, tx.Commit(ctx))
	hooks.Run(ctx)
	require.Equal(t, 1, count(), "an owner that offers hooks gets its announcement after its commit")
	require.Equal(t, 1, committedOutbox(t, env, orderservice.OrderCreatedEventID))

	// A savepoint begun by hand inside InTx has no hooks of its own: refused.
	require.NoError(t, composables.InTx(ctx, func(txCtx context.Context) error {
		raw, err := txCtx.Value(constants.TxKey).(pgx.Tx).Begin(txCtx)
		if err != nil {
			return err
		}
		defer func() { _ = raw.Rollback(context.WithoutCancel(txCtx)) }()
		if err := create(composables.WithTx(txCtx, raw)); !errors.Is(err, composables.ErrNoCommitHooks) {
			return fmt.Errorf("a savepoint without hooks must be refused, got %v", err)
		}
		return nil
	}))
	require.Equal(t, 1, count())
	require.Equal(t, 1, committedOutbox(t, env, orderservice.OrderCreatedEventID))

	// InSavepoint: rollback drops, release hands over to the caller.
	errLater := errors.New("rolled back by the caller")
	require.NoError(t, composables.InTx(ctx, func(txCtx context.Context) error {
		err := composables.InSavepoint(txCtx, func(spCtx context.Context) error {
			if err := create(spCtx); err != nil {
				return err
			}
			return errLater
		})
		if !errors.Is(err, errLater) {
			return fmt.Errorf("the savepoint must roll back with the caller's error, got %v", err)
		}
		return nil
	}))
	require.Equal(t, 1, count())
	require.Equal(t, 1, committedOutbox(t, env, orderservice.OrderCreatedEventID))

	err = composables.InTx(ctx, func(txCtx context.Context) error {
		if err := composables.InSavepoint(txCtx, create); err != nil {
			return err
		}
		if count() != 1 {
			return errors.New("announced before the caller's commit")
		}
		return errLater
	})
	require.ErrorIs(t, err, errLater)
	require.Equal(t, 1, count(), "the caller rolled back after the release")

	require.NoError(t, composables.InTx(ctx, func(txCtx context.Context) error {
		return composables.InSavepoint(txCtx, create)
	}))
	require.Equal(t, 2, count())
	require.Equal(t, 2, committedOutbox(t, env, orderservice.OrderCreatedEventID))
}

// An order read from storage reports no item as added: completing an order that
// has items announces its completion and nothing else.
func TestOrderEvents_CompletingAnOrderWithItemsAnnouncesNoItemAdded(t *testing.T) {
	env := itf.Setup(t,
		itf.WithComponents(modules.Components()...),
		itf.WithUser(itf.User(permissions.OrderCreate, permissions.OrderRead, permissions.OrderUpdate)),
	)
	env.User = user.New("", "", env.User.Email(), "", user.WithID(env.User.ID()), user.WithRoles(env.User.Roles()), user.WithTenantID(env.TenantID()))
	env.Ctx = composables.WithUser(env.Ctx, env.User)
	service := itf.GetService[orderservice.OrderService](env)
	bus, err := composition.Resolve[eventbus.EventBus](env.Container)
	require.NoError(t, err)

	productRepo := persistence.NewProductRepository()
	orderRepo := persistence.NewOrderRepository(productRepo)
	require.NoError(t, persistence.NewUnitRepository().Create(env.Ctx, &unit.Unit{ID: 1, Title: "Unit", ShortTitle: "U", CreatedAt: time.Now(), UpdatedAt: time.Now()}))
	pos := position.New("Position", "1234567890", position.WithID(1), position.WithUnitID(1), position.WithCreatedAt(time.Now()), position.WithUpdatedAt(time.Now()))
	_, err = persistence.NewPositionRepository().Create(env.Ctx, pos)
	require.NoError(t, err)
	draft, err := order.New(order.TypeIn, order.WithStatus(order.Pending)).AddItem(pos, product.New("EPS:1234567890", product.InStock, product.WithPosition(pos)))
	require.NoError(t, err)
	created, err := orderRepo.Create(env.Ctx, draft)
	require.NoError(t, err)

	var mu sync.Mutex
	var seen []string
	defer bus.Subscribe(func(order.ItemAddedEvent) { mu.Lock(); seen = append(seen, "itemAdded"); mu.Unlock() })()
	defer bus.Subscribe(func(order.OrderCompletedEvent) { mu.Lock(); seen = append(seen, "completed"); mu.Unlock() })()
	defer bus.Subscribe(func(order.OrderCreatedEvent) { mu.Lock(); seen = append(seen, "created"); mu.Unlock() })()
	env.CommitTx(t)

	ctx := itf.TestExecution(composables.WithUser(composables.WithTenantID(composables.WithPool(context.Background(), env.Pool), env.TenantID()), env.User))
	stored, err := orderRepo.GetByID(ctx, created.ID())
	require.NoError(t, err)
	require.Len(t, stored.Items(), 1)
	require.Empty(t, stored.Events(), "hydration records no events")

	_, err = service.Complete(ctx, created.ID())
	require.NoError(t, err)
	mu.Lock()
	defer mu.Unlock()
	require.Equal(t, []string{"completed"}, seen)
}
