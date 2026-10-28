package services_test

import (
	"context"
	"sort"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/currency"
	corepersistence "github.com/iota-uz/iota-sdk/modules/core/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/modules/finance/domain/aggregates/debt"
	expensecategory "github.com/iota-uz/iota-sdk/modules/finance/domain/aggregates/expense_category"
	moneyaccount "github.com/iota-uz/iota-sdk/modules/finance/domain/aggregates/money_account"
	paymentcategory "github.com/iota-uz/iota-sdk/modules/finance/domain/aggregates/payment_category"
	"github.com/iota-uz/iota-sdk/modules/finance/domain/entities/counterparty"
	"github.com/iota-uz/iota-sdk/modules/finance/domain/entities/inventory"
	"github.com/iota-uz/iota-sdk/modules/finance/domain/entities/transaction"
	"github.com/iota-uz/iota-sdk/modules/finance/permissions"
	"github.com/iota-uz/iota-sdk/modules/finance/services"
	"github.com/iota-uz/iota-sdk/modules/logging/domain/aggregates/businessevent"
	"github.com/iota-uz/iota-sdk/modules/logging/history"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/composition"
	"github.com/iota-uz/iota-sdk/pkg/itf"
	"github.com/iota-uz/iota-sdk/pkg/money"
	"github.com/stretchr/testify/require"
)

type financeHistoryFixture struct {
	env               *itf.TestEnvironment
	accounts          *services.MoneyAccountService
	transactions      *services.TransactionService
	counterparties    *services.CounterpartyService
	debts             *services.DebtService
	inventory         *services.InventoryService
	expenseCategories *services.ExpenseCategoryService
	paymentCategories *services.PaymentCategoryService
	events            businessevent.Repository
}

func setupFinanceHistory(t *testing.T) financeHistoryFixture {
	t.Helper()
	env := setupTest(t, permissions.DebtCreate, permissions.DebtRead, permissions.DebtUpdate, permissions.DebtDelete)
	require.NoError(t, corepersistence.NewCurrencyRepository().Create(env.Ctx, currency.USD))
	events, err := composition.Resolve[businessevent.Repository](env.Container)
	require.NoError(t, err)
	return financeHistoryFixture{
		env:               env,
		accounts:          getAccountService(env),
		transactions:      itf.GetService[services.TransactionService](env),
		counterparties:    itf.GetService[services.CounterpartyService](env),
		debts:             getDebtService(env),
		inventory:         itf.GetService[services.InventoryService](env),
		expenseCategories: itf.GetService[services.ExpenseCategoryService](env),
		paymentCategories: getPaymentCategoryService(env),
		events:            events,
	}
}

func (f financeHistoryFixture) newAccount(t *testing.T) moneyaccount.Account {
	t.Helper()
	account, err := f.accounts.Create(f.env.Ctx, moneyaccount.New(
		"Cash "+uuid.NewString()[:8], money.New(100000, "USD"),
		moneyaccount.WithTenantID(f.env.TenantID()), moneyaccount.WithAccountNumber(uuid.NewString()),
	))
	require.NoError(t, err)
	return account
}

func (f financeHistoryFixture) newTransfer(t *testing.T) transaction.Transaction {
	t.Helper()
	created, err := f.transactions.Create(f.env.Ctx, f.transfer(f.newAccount(t).ID(), f.newAccount(t).ID()))
	require.NoError(t, err)
	return created
}

func (f financeHistoryFixture) transfer(origin, destination uuid.UUID) transaction.Transaction {
	return transaction.New(
		money.New(25000, "USD"), transaction.Transfer,
		transaction.WithTenantID(f.env.TenantID()),
		transaction.WithOriginAccountID(origin),
		transaction.WithDestinationAccountID(destination),
		transaction.WithComment("Cash to bank"),
	)
}

func (f financeHistoryFixture) newCounterparty(t *testing.T) counterparty.Counterparty {
	t.Helper()
	created, err := f.counterparties.Create(f.env.Ctx, counterparty.New(
		"Supplier "+uuid.NewString()[:8], counterparty.Supplier, counterparty.LLC,
		counterparty.WithTenantID(f.env.TenantID()), counterparty.WithLegalAddress("Tashkent"),
	))
	require.NoError(t, err)
	return created
}

func (f financeHistoryFixture) debt() debt.Debt {
	return debt.New(debt.DebtTypePayable, money.New(50000, "USD"),
		debt.WithTenantID(f.env.TenantID()), debt.WithDescription("Office rent"),
	)
}

func (f financeHistoryFixture) newDebt(t *testing.T) debt.Debt {
	t.Helper()
	created, err := f.debts.Create(f.env.Ctx, f.debt().UpdateCounterpartyID(f.newCounterparty(t).ID()))
	require.NoError(t, err)
	return created
}

func (f financeHistoryFixture) newInventory(t *testing.T) inventory.Inventory {
	t.Helper()
	created, err := f.inventory.Create(f.env.Ctx, inventory.New("Chair "+uuid.NewString()[:8], money.New(4500, "USD"), 10,
		inventory.WithTenantID(f.env.TenantID()), inventory.WithDescription("Office chair"),
	))
	require.NoError(t, err)
	return created
}

func (f financeHistoryFixture) newExpenseCategory(t *testing.T) expensecategory.ExpenseCategory {
	t.Helper()
	created, err := f.expenseCategories.Create(f.env.Ctx, expensecategory.New("Rent "+uuid.NewString()[:8],
		expensecategory.WithTenantID(f.env.TenantID()), expensecategory.WithDescription("Office rent"),
	))
	require.NoError(t, err)
	return created
}

func (f financeHistoryFixture) newPaymentCategory(t *testing.T) paymentcategory.PaymentCategory {
	t.Helper()
	created, err := f.paymentCategories.Create(f.env.Ctx, paymentcategory.New("Sales "+uuid.NewString()[:8],
		paymentcategory.WithTenantID(f.env.TenantID()), paymentcategory.WithDescription("Product sales"),
	))
	require.NoError(t, err)
	return created
}

func (f financeHistoryFixture) operationEvents(t *testing.T, ctx context.Context) []businessevent.Event {
	t.Helper()
	exec, ok := history.UseExecution(ctx)
	require.True(t, ok)
	events, err := f.events.ListByOperation(f.env.Ctx, exec.OperationID)
	require.NoError(t, err)
	return events
}

func TestFinanceHistory_RecordsServiceMutations(t *testing.T) {
	f := setupFinanceHistory(t)
	accountFields := []string{"account_number", "balance", "description", "name"}
	transferFields := []string{"accounting_period", "amount", "comment", "destination_account_id", "destination_amount", "origin_account_id", "transaction_date", "type"}
	counterpartyFields := []string{"legal_address", "legal_type", "name", "tin", "type"}
	debtFields := []string{"counterparty_id", "description", "due_date", "money_account_id", "original_amount", "outstanding_amount", "project_id", "settlement_transaction_id", "status", "type"}
	inventoryFields := []string{"description", "name", "price", "quantity"}
	expenseCategoryFields := []string{"description", "is_cogs", "name"}
	paymentCategoryFields := []string{"description", "name"}

	tests := []struct {
		name      string
		act       func(t *testing.T, ctx context.Context) string
		eventType string
		resource  string
		fields    []string
	}{
		{
			name: "money account created",
			act: func(t *testing.T, ctx context.Context) string {
				t.Helper()
				account, err := f.accounts.Create(ctx, moneyaccount.New("Bank", money.New(70000, "USD"),
					moneyaccount.WithTenantID(f.env.TenantID()), moneyaccount.WithAccountNumber(uuid.NewString()),
				))
				require.NoError(t, err)
				return account.ID().String()
			},
			eventType: services.MoneyAccountCreated, resource: services.MoneyAccountResource, fields: accountFields,
		},
		{
			name: "money account renamed and balance corrected",
			act: func(t *testing.T, ctx context.Context) string {
				t.Helper()
				account := f.newAccount(t)
				_, err := f.accounts.Update(ctx, account.UpdateName("Main cash").UpdateBalance(money.New(90000, "USD")))
				require.NoError(t, err)
				return account.ID().String()
			},
			eventType: services.MoneyAccountUpdated, resource: services.MoneyAccountResource, fields: []string{"balance", "name"},
		},
		{
			name: "money account deleted",
			act: func(t *testing.T, ctx context.Context) string {
				t.Helper()
				account := f.newAccount(t)
				_, err := f.accounts.Delete(ctx, account.ID())
				require.NoError(t, err)
				return account.ID().String()
			},
			eventType: services.MoneyAccountDeleted, resource: services.MoneyAccountResource, fields: accountFields,
		},
		{
			name: "transfer between accounts created",
			act: func(t *testing.T, ctx context.Context) string {
				t.Helper()
				created, err := f.transactions.Create(ctx, f.transfer(f.newAccount(t).ID(), f.newAccount(t).ID()))
				require.NoError(t, err)
				return created.ID().String()
			},
			eventType: services.TransactionCreated, resource: services.TransactionResource, fields: transferFields,
		},
		{
			name: "transfer updated",
			act: func(t *testing.T, ctx context.Context) string {
				t.Helper()
				created := f.newTransfer(t)
				_, err := f.transactions.Update(ctx, created.UpdateComment("Corrected").UpdateAmount(money.New(30000, "USD")))
				require.NoError(t, err)
				return created.ID().String()
			},
			eventType: services.TransactionUpdated, resource: services.TransactionResource, fields: []string{"amount", "comment"},
		},
		{
			name: "transfer deleted",
			act: func(t *testing.T, ctx context.Context) string {
				t.Helper()
				created := f.newTransfer(t)
				require.NoError(t, f.transactions.Delete(ctx, created.ID()))
				return created.ID().String()
			},
			eventType: services.TransactionDeleted, resource: services.TransactionResource, fields: transferFields,
		},
		{
			name: "counterparty created",
			act: func(t *testing.T, ctx context.Context) string {
				t.Helper()
				created, err := f.counterparties.Create(ctx, counterparty.New("Client", counterparty.Customer, counterparty.Individual,
					counterparty.WithTenantID(f.env.TenantID()),
				))
				require.NoError(t, err)
				return created.ID().String()
			},
			eventType: services.CounterpartyCreated, resource: services.CounterpartyResource, fields: counterpartyFields,
		},
		{
			name: "counterparty updated",
			act: func(t *testing.T, ctx context.Context) string {
				t.Helper()
				created := f.newCounterparty(t)
				created.SetLegalAddress("Samarkand")
				created.SetType(counterparty.Both)
				_, err := f.counterparties.Update(ctx, created)
				require.NoError(t, err)
				return created.ID().String()
			},
			eventType: services.CounterpartyUpdated, resource: services.CounterpartyResource, fields: []string{"legal_address", "type"},
		},
		{
			name: "counterparty deleted",
			act: func(t *testing.T, ctx context.Context) string {
				t.Helper()
				created := f.newCounterparty(t)
				_, err := f.counterparties.Delete(ctx, created.ID())
				require.NoError(t, err)
				return created.ID().String()
			},
			eventType: services.CounterpartyDeleted, resource: services.CounterpartyResource, fields: counterpartyFields,
		},
		{
			name: "debt created",
			act: func(t *testing.T, ctx context.Context) string {
				t.Helper()
				created, err := f.debts.Create(ctx, f.debt().UpdateCounterpartyID(f.newCounterparty(t).ID()))
				require.NoError(t, err)
				return created.ID().String()
			},
			eventType: services.DebtCreated, resource: services.DebtResource, fields: debtFields,
		},
		{
			name: "debt updated",
			act: func(t *testing.T, ctx context.Context) string {
				t.Helper()
				created := f.newDebt(t)
				due := time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC)
				_, err := f.debts.Update(ctx, created.UpdateDescription("Office rent, December").UpdateDueDate(&due))
				require.NoError(t, err)
				return created.ID().String()
			},
			eventType: services.DebtUpdated, resource: services.DebtResource, fields: []string{"description", "due_date"},
		},
		{
			name: "debt partially settled",
			act: func(t *testing.T, ctx context.Context) string {
				t.Helper()
				created := f.newDebt(t)
				_, err := f.debts.Settle(ctx, created.ID(), 200, nil)
				require.NoError(t, err)
				return created.ID().String()
			},
			eventType: services.DebtSettled, resource: services.DebtResource, fields: []string{"outstanding_amount", "status"},
		},
		{
			name: "debt written off",
			act: func(t *testing.T, ctx context.Context) string {
				t.Helper()
				created := f.newDebt(t)
				_, err := f.debts.WriteOff(ctx, created.ID())
				require.NoError(t, err)
				return created.ID().String()
			},
			eventType: services.DebtWrittenOff, resource: services.DebtResource, fields: []string{"outstanding_amount", "status"},
		},
		{
			name: "debt cancelled",
			act: func(t *testing.T, ctx context.Context) string {
				t.Helper()
				created := f.newDebt(t)
				_, err := f.debts.Cancel(ctx, created.ID())
				require.NoError(t, err)
				return created.ID().String()
			},
			eventType: services.DebtCancelled, resource: services.DebtResource, fields: []string{"outstanding_amount", "status"},
		},
		{
			name: "debt deleted",
			act: func(t *testing.T, ctx context.Context) string {
				t.Helper()
				created := f.newDebt(t)
				_, err := f.debts.Delete(ctx, created.ID())
				require.NoError(t, err)
				return created.ID().String()
			},
			eventType: services.DebtDeleted, resource: services.DebtResource, fields: debtFields,
		},
		{
			name: "inventory item created",
			act: func(t *testing.T, ctx context.Context) string {
				t.Helper()
				created, err := f.inventory.Create(ctx, inventory.New("Desk", money.New(12000, "USD"), 3,
					inventory.WithTenantID(f.env.TenantID()),
				))
				require.NoError(t, err)
				return created.ID().String()
			},
			eventType: services.InventoryCreated, resource: services.InventoryResource, fields: inventoryFields,
		},
		{
			name: "inventory item updated",
			act: func(t *testing.T, ctx context.Context) string {
				t.Helper()
				created := f.newInventory(t)
				_, err := f.inventory.Update(ctx, created.UpdateQuantity(7).UpdatePrice(money.New(5000, "USD")))
				require.NoError(t, err)
				return created.ID().String()
			},
			eventType: services.InventoryUpdated, resource: services.InventoryResource, fields: []string{"price", "quantity"},
		},
		{
			name: "inventory item deleted",
			act: func(t *testing.T, ctx context.Context) string {
				t.Helper()
				created := f.newInventory(t)
				require.NoError(t, f.inventory.Delete(ctx, created.ID()))
				return created.ID().String()
			},
			eventType: services.InventoryDeleted, resource: services.InventoryResource, fields: inventoryFields,
		},
		{
			name: "expense category created",
			act: func(t *testing.T, ctx context.Context) string {
				t.Helper()
				created, err := f.expenseCategories.Create(ctx, expensecategory.New("Materials",
					expensecategory.WithTenantID(f.env.TenantID()), expensecategory.WithIsCOGS(true),
				))
				require.NoError(t, err)
				return created.ID().String()
			},
			eventType: services.ExpenseCategoryCreated, resource: services.ExpenseCategoryResource, fields: expenseCategoryFields,
		},
		{
			name: "expense category updated",
			act: func(t *testing.T, ctx context.Context) string {
				t.Helper()
				created := f.newExpenseCategory(t)
				_, err := f.expenseCategories.Update(ctx, created.UpdateDescription("Office and warehouse rent"))
				require.NoError(t, err)
				return created.ID().String()
			},
			eventType: services.ExpenseCategoryUpdated, resource: services.ExpenseCategoryResource, fields: []string{"description"},
		},
		{
			name: "expense category deleted",
			act: func(t *testing.T, ctx context.Context) string {
				t.Helper()
				created := f.newExpenseCategory(t)
				_, err := f.expenseCategories.Delete(ctx, created.ID())
				require.NoError(t, err)
				return created.ID().String()
			},
			eventType: services.ExpenseCategoryDeleted, resource: services.ExpenseCategoryResource, fields: expenseCategoryFields,
		},
		{
			name: "payment category created",
			act: func(t *testing.T, ctx context.Context) string {
				t.Helper()
				created, err := f.paymentCategories.Create(ctx, paymentcategory.New("Services",
					paymentcategory.WithTenantID(f.env.TenantID()),
				))
				require.NoError(t, err)
				return created.ID().String()
			},
			eventType: services.PaymentCategoryCreated, resource: services.PaymentCategoryResource, fields: paymentCategoryFields,
		},
		{
			name: "payment category updated",
			act: func(t *testing.T, ctx context.Context) string {
				t.Helper()
				created := f.newPaymentCategory(t)
				_, err := f.paymentCategories.Update(ctx, created.UpdateName("Services "+uuid.NewString()[:8]))
				require.NoError(t, err)
				return created.ID().String()
			},
			eventType: services.PaymentCategoryUpdated, resource: services.PaymentCategoryResource, fields: []string{"name"},
		},
		{
			name: "payment category deleted",
			act: func(t *testing.T, ctx context.Context) string {
				t.Helper()
				created := f.newPaymentCategory(t)
				_, err := f.paymentCategories.Delete(ctx, created.ID())
				require.NoError(t, err)
				return created.ID().String()
			},
			eventType: services.PaymentCategoryDeleted, resource: services.PaymentCategoryResource, fields: paymentCategoryFields,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := itf.TestExecution(f.env.Ctx)
			id := tt.act(t, ctx)

			events := f.operationEvents(t, ctx)
			require.Len(t, events, 1)
			e := events[0]
			require.Equal(t, tt.eventType, e.Type)
			require.Equal(t, []businessevent.Subject{{Resource: tt.resource, ID: id}}, e.Subjects)
			require.Len(t, e.Changes, 1)
			fields := make([]string, 0, len(e.Changes[0].Fields))
			for _, d := range e.Changes[0].Fields {
				fields = append(fields, d.Field)
			}
			sort.Strings(fields)
			require.Equal(t, tt.fields, fields)
		})
	}
}

func TestFinanceHistory_UnchangedUpdateRecordsNothing(t *testing.T) {
	f := setupFinanceHistory(t)
	created := f.newCounterparty(t)
	ctx := itf.TestExecution(f.env.Ctx)

	_, err := f.counterparties.Update(ctx, created)
	require.NoError(t, err)

	require.Empty(t, f.operationEvents(t, ctx))
}

func TestFinanceHistory_RollbackLeavesNoEvents(t *testing.T) {
	f := setupFinanceHistory(t)
	created := f.newDebt(t)

	savepoint, err := f.env.Tx.Begin(f.env.Ctx)
	require.NoError(t, err)
	operation := itf.TestExecution(composables.WithTx(f.env.Ctx, savepoint))

	_, err = f.debts.Settle(operation, created.ID(), 500, nil)
	require.NoError(t, err)
	require.Len(t, f.operationEvents(t, operation), 1)
	require.NoError(t, savepoint.Rollback(f.env.Ctx))

	require.Empty(t, f.operationEvents(t, operation))
	reloaded, err := f.debts.GetByID(f.env.Ctx, created.ID())
	require.NoError(t, err)
	require.Equal(t, debt.DebtStatusPending, reloaded.Status())
}

func TestFinanceHistory_MissingExecutionFailsClosed(t *testing.T) {
	f := setupFinanceHistory(t)
	ctx := composables.WithPool(context.Background(), f.env.Pool)
	ctx = composables.WithTenantID(ctx, f.env.TenantID())
	ctx = composables.WithParams(ctx, itf.DefaultParams())
	ctx = composables.WithSession(ctx, itf.MockSession())
	ctx = composables.WithUser(ctx, f.env.User)
	ctx = composables.WithTx(ctx, f.env.Tx)

	_, err := f.inventory.Create(ctx, inventory.New("Lamp", money.New(900, "USD"), 1, inventory.WithTenantID(f.env.TenantID())))

	require.ErrorIs(t, err, history.ErrExecutionMissing)
}
