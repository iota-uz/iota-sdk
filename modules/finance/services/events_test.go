package services_test

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/currency"
	corepersistence "github.com/iota-uz/iota-sdk/modules/core/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/modules/finance/domain/aggregates/debt"
	"github.com/iota-uz/iota-sdk/modules/finance/domain/aggregates/expense"
	"github.com/iota-uz/iota-sdk/modules/finance/domain/aggregates/payment"
	paymentcategory "github.com/iota-uz/iota-sdk/modules/finance/domain/aggregates/payment_category"
	"github.com/iota-uz/iota-sdk/modules/finance/permissions"
	"github.com/iota-uz/iota-sdk/modules/finance/services"
	"github.com/iota-uz/iota-sdk/modules/logging/domain/aggregates/businessevent"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/composition"
	"github.com/iota-uz/iota-sdk/pkg/constants"
	"github.com/iota-uz/iota-sdk/pkg/eventbus"
	"github.com/iota-uz/iota-sdk/pkg/itf"
	"github.com/iota-uz/iota-sdk/pkg/money"
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
	spCtx, _ := composables.OwnTx(ctx, savepoint)
	act(spCtx)
	require.Len(t, outboxRows(t, spCtx, tenantID, event), before+1, "the event is in the transaction of the change")
	require.NoError(t, savepoint.Rollback(ctx))
	require.Len(t, outboxRows(t, ctx, tenantID, event), before, "rolling the change back takes its event with it")

	act(ctx)
	rows := outboxRows(t, ctx, tenantID, event)
	require.Len(t, rows, before+1, "committing the change keeps its event")
	return rows[len(rows)-1]
}

type financeEventsFixture struct {
	financeHistoryFixture
	payments *services.PaymentService
	expenses *services.ExpenseService
	ctx      context.Context
}

func newFinanceEventsFixture(t *testing.T) financeEventsFixture {
	t.Helper()
	env := setupTest(t, permissions.Permissions...)
	// The acting user belongs to the tenant, as a signed-in user does.
	env.User = user.New("", "", env.User.Email(), "", user.WithID(env.User.ID()), user.WithRoles(env.User.Roles()), user.WithTenantID(env.TenantID()))
	env.Ctx = composables.WithUser(env.Ctx, env.User)
	require.NoError(t, corepersistence.NewCurrencyRepository().Create(env.Ctx, currency.USD))
	events, err := composition.Resolve[businessevent.Repository](env.Container)
	require.NoError(t, err)
	return financeEventsFixture{
		financeHistoryFixture: financeHistoryFixture{
			env: env, accounts: getAccountService(env), counterparties: itf.GetService[services.CounterpartyService](env),
			debts: getDebtService(env), expenseCategories: itf.GetService[services.ExpenseCategoryService](env),
			paymentCategories: getPaymentCategoryService(env), events: events,
		},
		payments: getPaymentService(env),
		expenses: itf.GetService[services.ExpenseService](env),
		ctx:      itf.TestExecution(env.Ctx),
	}
}

func (f financeEventsFixture) payment(t *testing.T) payment.Payment {
	t.Helper()
	day := time.Date(2026, 10, 28, 0, 0, 0, 0, time.UTC)
	return payment.New(money.New(150050, "USD"), f.newPaymentCategory(t),
		payment.WithTenantID(f.env.TenantID()), payment.WithAccount(f.newAccount(t)),
		payment.WithCounterpartyID(f.newCounterparty(t).ID()),
		payment.WithTransactionDate(day), payment.WithAccountingPeriod(day), payment.WithComment("Invoice 7"),
	)
}

func TestPaymentEvents_CommitAndRollBackWithThePayment(t *testing.T) {
	f := newFinanceEventsFixture(t)
	tenant := f.env.TenantID()
	entity := f.payment(t)

	var created payment.Payment
	row := commitsAndRollsBack(t, f.ctx, tenant, services.PaymentCreatedEventID, func(ctx context.Context) {
		var err error
		created, err = f.payments.Create(ctx, entity)
		require.NoError(t, err)
	})
	require.Equal(t, created.ID().String(), row.Subject)
	require.NotNil(t, row.Initiator, "the acting user is the event initiator")
	require.JSONEq(t, `{"paymentId":"`+created.ID().String()+`","amount":{"amount":"150050","currency":"USD"},"accountId":"`+
		entity.Account().ID().String()+`","categoryId":"`+entity.Category().ID().String()+`","counterpartyId":"`+
		entity.CounterpartyID().String()+`","transactionDate":"2026-10-28","comment":"Invoice 7"}`, string(row.Payload))

	// An update that changes a field publishes the changed field and the amount.
	updated := created.UpdateComment("Invoice 7, paid").UpdateAmount(money.New(200000, "USD"))
	row = commitsAndRollsBack(t, f.ctx, tenant, services.PaymentUpdatedEventID, func(ctx context.Context) {
		_, err := f.payments.Update(ctx, updated)
		require.NoError(t, err)
	})
	require.JSONEq(t, `{"paymentId":"`+created.ID().String()+`","amount":{"amount":"200000","currency":"USD"},"changed":["amount","comment"]}`, string(row.Payload))

	// An update that changes nothing publishes nothing.
	before := len(outboxRows(t, f.ctx, tenant, services.PaymentUpdatedEventID))
	current, err := f.payments.GetByID(f.ctx, created.ID())
	require.NoError(t, err)
	_, err = f.payments.Update(f.ctx, current)
	require.NoError(t, err)
	require.Len(t, outboxRows(t, f.ctx, tenant, services.PaymentUpdatedEventID), before)
}

func TestExpenseEvents_CommitAndRollBackWithTheExpense(t *testing.T) {
	f := newFinanceEventsFixture(t)
	day := time.Date(2026, 10, 28, 0, 0, 0, 0, time.UTC)
	account, category := f.newAccount(t), f.newExpenseCategory(t)
	entity := expense.New(money.New(70000, "USD"), account, category, day,
		expense.WithTenantID(f.env.TenantID()), expense.WithAccountingPeriod(day))

	var created expense.Expense
	row := commitsAndRollsBack(t, f.ctx, f.env.TenantID(), services.ExpenseCreatedEventID, func(ctx context.Context) {
		var err error
		created, err = f.expenses.Create(ctx, entity)
		require.NoError(t, err)
	})
	require.Equal(t, created.ID().String(), row.Subject)
	require.JSONEq(t, `{"expenseId":"`+created.ID().String()+`","amount":{"amount":"70000","currency":"USD"},"accountId":"`+
		account.ID().String()+`","categoryId":"`+category.ID().String()+`","date":"2026-10-28"}`, string(row.Payload))
}

func TestDebtEvents_CommitAndRollBackWithTheDebt(t *testing.T) {
	f := newFinanceEventsFixture(t)
	tenant := f.env.TenantID()
	party := f.newCounterparty(t)
	due := time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC)
	entity := f.debt().UpdateCounterpartyID(party.ID()).UpdateDueDate(&due)

	var created debt.Debt
	row := commitsAndRollsBack(t, f.ctx, tenant, services.DebtCreatedEventID, func(ctx context.Context) {
		var err error
		created, err = f.debts.Create(ctx, entity)
		require.NoError(t, err)
	})
	require.Equal(t, created.ID().String(), row.Subject)
	require.JSONEq(t, `{"debtId":"`+created.ID().String()+`","type":"payable","counterpartyId":"`+party.ID().String()+
		`","originalAmount":{"amount":"50000","currency":"USD"},"dueDate":"2026-12-01"}`, string(row.Payload))

	// A partial settlement is the same event with status partial; the amounts
	// are minor units and match the balance of the debt.
	var partial debt.Debt
	row = commitsAndRollsBack(t, f.ctx, tenant, services.DebtSettledEventID, func(ctx context.Context) {
		var err error
		partial, err = f.debts.Settle(ctx, created.ID(), 100.00, nil)
		require.NoError(t, err)
	})
	require.JSONEq(t, `{"debtId":"`+created.ID().String()+`","type":"payable","status":"partial","counterpartyId":"`+party.ID().String()+
		`","originalAmount":{"amount":"50000","currency":"USD"},"outstandingAmount":{"amount":"40000","currency":"USD"},`+
		`"settledAmount":{"amount":"10000","currency":"USD"},"dueDate":"2026-12-01"}`, string(row.Payload))
	require.EqualValues(t, 40000, partial.OutstandingAmount().Amount())

	// The final settlement says settled; an overpayment settles only what was owed.
	row = commitsAndRollsBack(t, f.ctx, tenant, services.DebtSettledEventID, func(ctx context.Context) {
		_, err := f.debts.Settle(ctx, created.ID(), 450.00, nil)
		require.NoError(t, err)
	})
	var settled services.DebtSettledPayload
	require.NoError(t, json.Unmarshal(row.Payload, &settled))
	require.Equal(t, "settled", settled.Status)
	require.EqualValues(t, 0, settled.OutstandingAmount.Amount)
	require.EqualValues(t, 40000, settled.SettledAmount.Amount)

	writeOff := f.newDebt(t)
	row = commitsAndRollsBack(t, f.ctx, tenant, services.DebtWrittenOffEventID, func(ctx context.Context) {
		_, err := f.debts.WriteOff(ctx, writeOff.ID())
		require.NoError(t, err)
	})
	require.JSONEq(t, `{"debtId":"`+writeOff.ID().String()+`","type":"payable","counterpartyId":"`+writeOff.CounterpartyID().String()+
		`","originalAmount":{"amount":"50000","currency":"USD"}}`, string(row.Payload))

	cancel := f.newDebt(t)
	row = commitsAndRollsBack(t, f.ctx, tenant, services.DebtCancelledEventID, func(ctx context.Context) {
		_, err := f.debts.Cancel(ctx, cancel.ID())
		require.NoError(t, err)
	})
	require.JSONEq(t, `{"debtId":"`+cancel.ID().String()+`","type":"payable","counterpartyId":"`+cancel.CounterpartyID().String()+
		`","originalAmount":{"amount":"50000","currency":"USD"}}`, string(row.Payload))
}

// A call from MCP, an agent or a job has a user but no session. The settlement
// is saved and announced; building the eventbus event after the commit does not
// turn the call into a failure.
func TestDebtService_Settle_WithoutSession(t *testing.T) {
	f := newFinanceEventsFixture(t)
	created := f.newDebt(t)
	f.env.CommitTx(t)
	// ctx carries a user and no session.
	ctx := context.WithValue(committedCtx(f.env), constants.SessionKey, nil)
	_, err := composables.UseSession(ctx)
	require.Error(t, err, "the context has no session")
	actor, err := composables.UseUser(ctx)
	require.NoError(t, err)
	require.NotNil(t, actor)

	bus, err := composition.Resolve[eventbus.EventBus](f.env.Container)
	require.NoError(t, err)
	var mu sync.Mutex
	var announced []*debt.Settled
	unsubscribe := bus.Subscribe(func(e *debt.Settled) { mu.Lock(); announced = append(announced, e); mu.Unlock() })
	defer unsubscribe()

	settled, err := f.debts.Settle(ctx, created.ID(), 500.00, nil)
	require.NoError(t, err, "a committed settlement is not reported as failed")
	require.Equal(t, debt.DebtStatusSettled, settled.Status())

	stored, err := f.debts.GetByID(ctx, created.ID())
	require.NoError(t, err)
	require.Equal(t, debt.DebtStatusSettled, stored.Status())
	require.Len(t, committedPayloads(t, f, services.DebtSettledEventID), 1)
	mu.Lock()
	require.Len(t, announced, 1, "the eventbus event is still published, without a session")
	require.Nil(t, announced[0].Session)
	mu.Unlock()

	for name, call := range map[string]func(debt.Debt) error{
		"write off": func(d debt.Debt) error { _, err := f.debts.WriteOff(ctx, d.ID()); return err },
		"cancel":    func(d debt.Debt) error { _, err := f.debts.Cancel(ctx, d.ID()); return err },
	} {
		other, err := f.debts.Create(committedCtx(f.env), f.debt().UpdateCounterpartyID(created.CounterpartyID()))
		require.NoError(t, err)
		require.NoError(t, call(other), name)
	}
}

// committedPayloads are the payloads of the committed rows of the event, oldest first.
func committedPayloads(t *testing.T, f financeEventsFixture, event string) []json.RawMessage {
	t.Helper()
	rows, err := f.env.Pool.Query(context.Background(),
		`SELECT payload FROM extruntime.outbox_events WHERE tenant_id = $1 AND event = $2 ORDER BY seq`, f.env.TenantID(), event)
	require.NoError(t, err)
	defer rows.Close()
	var out []json.RawMessage
	for rows.Next() {
		var p json.RawMessage
		require.NoError(t, rows.Scan(&p))
		out = append(out, p)
	}
	require.NoError(t, rows.Err())
	return out
}

func runConcurrently(n int, call func(i int) error) []error {
	var wg sync.WaitGroup
	errs := make([]error, n)
	for i := range errs {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs[i] = call(i)
		}()
	}
	wg.Wait()
	return errs
}

// Two settlements racing on one debt are applied one after the other, and each
// event reports the transition it actually made: the outstanding amounts of the
// two events chain from the debt's balance instead of both claiming the same
// starting point.
func TestDebtEvents_ConcurrentSettlementsAnnounceTheirOwnTransitions(t *testing.T) {
	f := newFinanceEventsFixture(t)
	created := f.newDebt(t) // 500.00 USD = 50000 minor units
	f.env.CommitTx(t)
	ctx := committedCtx(f.env)

	errs := runConcurrently(2, func(int) error {
		_, err := f.debts.Settle(ctx, created.ID(), 100.00, nil)
		return err
	})
	require.NoError(t, errs[0])
	require.NoError(t, errs[1])

	stored, err := f.debts.GetByID(ctx, created.ID())
	require.NoError(t, err)
	require.EqualValues(t, 30000, stored.OutstandingAmount().Amount(), "both settlements are in the balance")

	payloads := committedPayloads(t, f, services.DebtSettledEventID)
	outstanding := make([]int64, 0, len(payloads))
	for _, raw := range payloads {
		var p services.DebtSettledPayload
		require.NoError(t, json.Unmarshal(raw, &p))
		require.EqualValues(t, 10000, p.SettledAmount.Amount)
		outstanding = append(outstanding, p.OutstandingAmount.Amount)
	}
	require.Equal(t, []int64{40000, 30000}, outstanding, "each event is consistent with the balance transition it made")
}

// Identical updates racing on one payment change it once: the second finds the
// stored state already equal and announces nothing.
func TestPaymentEvents_ConcurrentIdenticalUpdatesAnnounceOnce(t *testing.T) {
	f := newFinanceEventsFixture(t)
	created, err := f.payments.Create(f.ctx, f.payment(t))
	require.NoError(t, err)
	f.env.CommitTx(t)
	ctx := committedCtx(f.env)

	updated := created.UpdateComment("Invoice 7, paid")
	errs := runConcurrently(2, func(int) error {
		_, err := f.payments.Update(ctx, updated)
		return err
	})
	require.NoError(t, errs[0])
	require.NoError(t, errs[1])
	rows := committedPayloads(t, f, services.PaymentUpdatedEventID)
	require.Len(t, rows, 1)
	require.JSONEq(t, `{"paymentId":"`+created.ID().String()+`","amount":{"amount":"150050","currency":"USD"},"changed":["comment"]}`, string(rows[0]))
}

// The eventbus events of a debt change are published when the transaction that
// holds it commits: a caller's transaction that rolls back after Settle returned
// announces nothing.
func TestDebtService_EventbusFollowsTheOwnersCommit(t *testing.T) {
	f := newFinanceEventsFixture(t)
	created := f.newDebt(t)
	f.env.CommitTx(t)
	ctx := committedCtx(f.env)
	bus, err := composition.Resolve[eventbus.EventBus](f.env.Container)
	require.NoError(t, err)
	var mu sync.Mutex
	announced := 0
	defer bus.Subscribe(func(*debt.Settled) { mu.Lock(); announced++; mu.Unlock() })()
	count := func() int { mu.Lock(); defer mu.Unlock(); return announced }
	errLater := errors.New("the caller's later step failed")

	err = composables.InTx(ctx, func(txCtx context.Context) error {
		_, err := f.debts.Settle(txCtx, created.ID(), 100.00, nil)
		require.NoError(t, err)
		require.Zero(t, count(), "the eventbus event waits for the caller's commit")
		return errLater
	})
	require.ErrorIs(t, err, errLater)
	require.Zero(t, count(), "a rolled back settlement is not announced")
	require.Empty(t, committedPayloads(t, f, services.DebtSettledEventID))
	stored, err := f.debts.GetByID(ctx, created.ID())
	require.NoError(t, err)
	require.EqualValues(t, 50000, stored.OutstandingAmount().Amount())

	err = composables.InTx(ctx, func(txCtx context.Context) error {
		_, err := f.debts.Settle(txCtx, created.ID(), 100.00, nil)
		return err
	})
	require.NoError(t, err)
	require.Equal(t, 1, count())
}

// A payment without a category keeps no category on every read: the created
// payload carries no categoryId and an update that changes nothing publishes
// nothing.
func TestPaymentEvents_UncategorizedPaymentStaysUncategorized(t *testing.T) {
	f := newFinanceEventsFixture(t)
	tenant := f.env.TenantID()
	day := time.Date(2026, 10, 28, 0, 0, 0, 0, time.UTC)
	entity := payment.New(money.New(1500, "USD"), paymentcategory.New("none", paymentcategory.WithTenantID(tenant), paymentcategory.WithID(uuid.Nil)),
		payment.WithTenantID(tenant), payment.WithAccount(f.newAccount(t)),
		payment.WithCounterpartyID(f.newCounterparty(t).ID()),
		payment.WithTransactionDate(day), payment.WithAccountingPeriod(day),
	)
	created, err := f.payments.Create(f.ctx, entity)
	require.NoError(t, err)
	rows := outboxRows(t, f.ctx, tenant, services.PaymentCreatedEventID)
	require.Len(t, rows, 1)
	require.NotContains(t, string(rows[0].Payload), "categoryId")

	first, err := f.payments.GetByID(f.ctx, created.ID())
	require.NoError(t, err)
	second, err := f.payments.GetByID(f.ctx, created.ID())
	require.NoError(t, err)
	require.Equal(t, first.Category().ID(), second.Category().ID(), "the absent category reads back the same every time")

	_, err = f.payments.Update(f.ctx, first)
	require.NoError(t, err)
	require.Empty(t, outboxRows(t, f.ctx, tenant, services.PaymentUpdatedEventID), "an unchanged update publishes nothing")
}

// The eventbus events of a payment are published when the transaction that holds
// it commits, as the native events are.
func TestPaymentService_EventbusFollowsTheOwnersCommit(t *testing.T) {
	f := newFinanceEventsFixture(t)
	entity := f.payment(t)
	f.env.CommitTx(t)
	ctx := committedCtx(f.env)
	bus, err := composition.Resolve[eventbus.EventBus](f.env.Container)
	require.NoError(t, err)
	var mu sync.Mutex
	announced := 0
	defer bus.Subscribe(func(*payment.Created) { mu.Lock(); announced++; mu.Unlock() })()
	count := func() int { mu.Lock(); defer mu.Unlock(); return announced }
	errLater := errors.New("the caller's later step failed")

	err = composables.InTx(ctx, func(txCtx context.Context) error {
		if _, err := f.payments.Create(txCtx, entity); err != nil {
			return err
		}
		if count() != 0 {
			return errors.New("announced before the caller's commit")
		}
		return errLater
	})
	require.ErrorIs(t, err, errLater)
	require.Zero(t, count(), "a rolled back payment is not announced")

	require.NoError(t, composables.InTx(ctx, func(txCtx context.Context) error {
		_, err := f.payments.Create(txCtx, entity)
		return err
	}))
	require.Equal(t, 1, count())
}
