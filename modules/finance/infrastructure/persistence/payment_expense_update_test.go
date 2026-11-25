package persistence_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/currency"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/country"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/tax"
	corepersistence "github.com/iota-uz/iota-sdk/modules/core/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/modules/finance/domain/aggregates/expense"
	expensecategory "github.com/iota-uz/iota-sdk/modules/finance/domain/aggregates/expense_category"
	moneyaccount "github.com/iota-uz/iota-sdk/modules/finance/domain/aggregates/money_account"
	"github.com/iota-uz/iota-sdk/modules/finance/domain/aggregates/payment"
	paymentcategory "github.com/iota-uz/iota-sdk/modules/finance/domain/aggregates/payment_category"
	"github.com/iota-uz/iota-sdk/modules/finance/domain/entities/counterparty"
	"github.com/iota-uz/iota-sdk/modules/finance/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/itf"
	"github.com/iota-uz/iota-sdk/pkg/money"
)

func newTestAccount(t *testing.T, ctx context.Context) moneyaccount.Account {
	t.Helper()
	tenantID, err := composables.UseTenantID(ctx)
	require.NoError(t, err)
	account, err := persistence.NewMoneyAccountRepository().Create(ctx, moneyaccount.New(
		"Cash",
		money.New(0, "USD"),
		moneyaccount.WithTenantID(tenantID),
		moneyaccount.WithAccountNumber(uuid.NewString()[:8]),
	))
	require.NoError(t, err)
	return account
}

func newTestPayment(t *testing.T, ctx context.Context, category paymentcategory.PaymentCategory) payment.Payment {
	t.Helper()
	tenantID, err := composables.UseTenantID(ctx)
	require.NoError(t, err)
	tin, err := tax.NewTin("123456789", country.Uzbekistan)
	require.NoError(t, err)
	party, err := persistence.NewCounterpartyRepository().Create(ctx, counterparty.New(
		"Counterparty",
		counterparty.Customer,
		counterparty.LLC,
		counterparty.WithTenantID(tenantID),
		counterparty.WithTin(tin),
		counterparty.WithLegalAddress("Address"),
	))
	require.NoError(t, err)
	created, err := persistence.NewPaymentRepository().Create(ctx, payment.New(
		money.New(10000, "USD"),
		category,
		payment.WithTenantID(tenantID),
		payment.WithCounterpartyID(party.ID()),
		payment.WithTransactionDate(time.Now()),
		payment.WithAccountingPeriod(time.Now()),
		payment.WithAccount(newTestAccount(t, ctx)),
	))
	require.NoError(t, err)
	return created
}

func newTestPaymentCategory(t *testing.T, ctx context.Context, name string) paymentcategory.PaymentCategory {
	t.Helper()
	tenantID, err := composables.UseTenantID(ctx)
	require.NoError(t, err)
	created, err := persistence.NewPaymentCategoryRepository().Create(ctx, paymentcategory.New(name, paymentcategory.WithTenantID(tenantID)))
	require.NoError(t, err)
	return created
}

func newTestExpense(t *testing.T, ctx context.Context, repo expense.Repository) expense.Expense {
	t.Helper()
	tenantID, err := composables.UseTenantID(ctx)
	require.NoError(t, err)
	category, err := persistence.NewExpenseCategoryRepository().Create(ctx, expensecategory.New("Rent", expensecategory.WithTenantID(tenantID)))
	require.NoError(t, err)
	created, err := repo.Create(ctx, expense.New(
		money.New(5000, "USD"),
		newTestAccount(t, ctx),
		category,
		time.Now(),
		expense.WithTenantID(tenantID),
		expense.WithAccountingPeriod(time.Now()),
		expense.WithComment("before"),
	))
	require.NoError(t, err)
	return created
}

func newExpenseRepository() expense.Repository {
	return persistence.NewExpenseRepository(persistence.NewExpenseCategoryRepository(), persistence.NewTransactionRepository())
}

// otherTenantCtx returns a context of a second tenant in the same database transaction.
func otherTenantCtx(t *testing.T, f *itf.TestEnvironment) context.Context {
	t.Helper()
	other, err := itf.CreateTestTenant(f.Ctx, f.Pool)
	require.NoError(t, err)
	return composables.WithTenantID(f.Ctx, other.ID)
}

func TestGormPaymentRepository_Update_WritesCategory(t *testing.T) {
	t.Parallel()
	f := setupTest(t)
	require.NoError(t, corepersistence.NewCurrencyRepository().Create(f.Ctx, currency.USD))
	repo := persistence.NewPaymentRepository()

	first := newTestPaymentCategory(t, f.Ctx, "First")
	second := newTestPaymentCategory(t, f.Ctx, "Second")
	created := newTestPayment(t, f.Ctx, first)
	require.Equal(t, first.ID(), created.Category().ID())

	updated, err := repo.Update(f.Ctx, created.UpdateCategory(second))
	require.NoError(t, err)
	require.Equal(t, second.ID(), updated.Category().ID())
	require.True(t, updated.UpdatedAt().After(created.UpdatedAt()))

	reread, err := repo.GetByID(f.Ctx, created.ID())
	require.NoError(t, err)
	require.Equal(t, second.ID(), reread.Category().ID())

	cleared, err := repo.Update(f.Ctx, reread.UpdateCategory(nil))
	require.NoError(t, err)
	require.Equal(t, uuid.Nil, cleared.Category().ID())

	var categoryID *uuid.UUID
	require.NoError(t, f.Tx.QueryRow(f.Ctx, "SELECT payment_category_id FROM payments WHERE id = $1", created.ID()).Scan(&categoryID))
	require.Nil(t, categoryID)
}

func TestGormPaymentRepository_UpdateAndDelete_OtherTenantUntouched(t *testing.T) {
	t.Parallel()
	f := setupTest(t)
	require.NoError(t, corepersistence.NewCurrencyRepository().Create(f.Ctx, currency.USD))
	repo := persistence.NewPaymentRepository()
	otherCtx := otherTenantCtx(t, f)

	otherCategory := newTestPaymentCategory(t, otherCtx, "Other")
	foreign := newTestPayment(t, otherCtx, otherCategory)

	_, err := repo.Update(f.Ctx, foreign.UpdateCategory(nil).UpdateComment("hijacked"))
	require.ErrorIs(t, err, persistence.ErrPaymentNotFound)
	require.Error(t, repo.Delete(f.Ctx, foreign.ID()))

	stored, err := repo.GetByID(otherCtx, foreign.ID())
	require.NoError(t, err)
	require.Equal(t, otherCategory.ID(), stored.Category().ID())
	require.Equal(t, foreign.Comment(), stored.Comment())
	require.Equal(t, foreign.UpdatedAt().UTC(), stored.UpdatedAt().UTC())
}

func TestGormExpenseRepository_Update_BumpsUpdatedAt(t *testing.T) {
	t.Parallel()
	f := setupTest(t)
	require.NoError(t, corepersistence.NewCurrencyRepository().Create(f.Ctx, currency.USD))
	repo := newExpenseRepository()

	created := newTestExpense(t, f.Ctx, repo)
	edited := created.SetComment("after")

	updated, err := repo.Update(f.Ctx, edited)
	require.NoError(t, err)
	require.Equal(t, "after", updated.Comment())
	require.True(t, updated.UpdatedAt().After(created.UpdatedAt()),
		"updated_at %s must move past %s", updated.UpdatedAt(), created.UpdatedAt())
	require.WithinDuration(t, edited.UpdatedAt(), updated.UpdatedAt(), time.Millisecond)

	reread, err := repo.GetByID(f.Ctx, created.ID())
	require.NoError(t, err)
	require.Equal(t, updated.UpdatedAt(), reread.UpdatedAt())
}

func TestGormExpenseRepository_UpdateAndDelete_OtherTenantUntouched(t *testing.T) {
	t.Parallel()
	f := setupTest(t)
	require.NoError(t, corepersistence.NewCurrencyRepository().Create(f.Ctx, currency.USD))
	repo := newExpenseRepository()
	otherCtx := otherTenantCtx(t, f)

	foreign := newTestExpense(t, otherCtx, repo)

	_, err := repo.Update(f.Ctx, foreign.SetComment("hijacked"))
	require.ErrorIs(t, err, persistence.ErrExpenseNotFound)
	require.NoError(t, repo.Delete(f.Ctx, foreign.ID()))

	stored, err := repo.GetByID(otherCtx, foreign.ID())
	require.NoError(t, err)
	require.Equal(t, "before", stored.Comment())
	require.Equal(t, foreign.UpdatedAt(), stored.UpdatedAt())
}
