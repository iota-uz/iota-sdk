package services_test

import (
	"context"
	"encoding/json"
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
	// ctx carries a user and no session.
	ctx := context.WithValue(f.ctx, constants.SessionKey, nil)
	_, err := composables.UseSession(ctx)
	require.Error(t, err, "the context has no session")
	actor, err := composables.UseUser(ctx)
	require.NoError(t, err)
	require.NotNil(t, actor)

	bus, err := composition.Resolve[eventbus.EventBus](f.env.Container)
	require.NoError(t, err)
	var announced []*debt.Settled
	unsubscribe := bus.Subscribe(func(e *debt.Settled) { announced = append(announced, e) })
	defer unsubscribe()

	settled, err := f.debts.Settle(ctx, created.ID(), 500.00, nil)
	require.NoError(t, err, "a committed settlement is not reported as failed")
	require.Equal(t, debt.DebtStatusSettled, settled.Status())

	stored, err := f.debts.GetByID(f.ctx, created.ID())
	require.NoError(t, err)
	require.Equal(t, debt.DebtStatusSettled, stored.Status())
	require.Len(t, outboxRows(t, f.ctx, f.env.TenantID(), services.DebtSettledEventID), 1)
	require.Len(t, announced, 1, "the eventbus event is still published, without a session")
	require.Nil(t, announced[0].Session)

	for name, call := range map[string]func(debt.Debt) error{
		"write off": func(d debt.Debt) error { _, err := f.debts.WriteOff(ctx, d.ID()); return err },
		"cancel":    func(d debt.Debt) error { _, err := f.debts.Cancel(ctx, d.ID()); return err },
	} {
		require.NoError(t, call(f.newDebt(t)), name)
	}
}
