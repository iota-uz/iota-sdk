package services_test

import (
	"context"
	"testing"
	"time"

	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/currency"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/country"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/tax"
	corepersistence "github.com/iota-uz/iota-sdk/modules/core/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/modules/finance/domain/aggregates/debt"
	"github.com/iota-uz/iota-sdk/modules/finance/domain/entities/counterparty"
	"github.com/iota-uz/iota-sdk/modules/finance/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/modules/finance/permissions"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/money"
	"github.com/stretchr/testify/require"
)

// setupDebtTestData creates necessary test data for debt tests
func setupDebtTestData(ctx context.Context, t *testing.T) counterparty.Counterparty {
	t.Helper()

	// Create currency
	currencyRepo := corepersistence.NewCurrencyRepository()
	if err := currencyRepo.Create(ctx, &currency.USD); err != nil {
		t.Fatal(err)
	}

	// Create counterparty
	counterpartyRepo := persistence.NewCounterpartyRepository()
	tin, err := tax.NewTin("123456789", country.Uzbekistan)
	require.NoError(t, err)

	// Get tenant ID from context
	testTenantID, err := composables.UseTenantID(ctx)
	require.NoError(t, err)

	createdCounterparty, err := counterpartyRepo.Create(ctx, counterparty.New(
		"Test Counterparty",
		counterparty.Customer,
		counterparty.LLC,
		counterparty.WithTenantID(testTenantID),
		counterparty.WithTin(tin),
		counterparty.WithLegalAddress("Test Address"),
	))
	require.NoError(t, err)

	return createdCounterparty
}

func TestDebtService_CRUD(t *testing.T) {
	t.Parallel()
	f := setupTest(t,
		permissions.DebtCreate,
		permissions.DebtRead,
		permissions.DebtUpdate,
		permissions.DebtDelete,
	)

	counterparty := setupDebtTestData(f.Ctx, t)
	debtService := getDebtService(f)

	// Get tenant ID from context
	tenantID, err := composables.UseTenantID(f.Ctx)
	require.NoError(t, err)

	// Test Create
	originalAmount := money.New(100000, "USD") // $1000.00
	dueDate := time.Now().AddDate(0, 1, 0)     // 1 month from now
	debtEntity := debt.New(
		debt.DebtTypeReceivable,
		originalAmount,
		debt.WithTenantID(tenantID),
		debt.WithCounterpartyID(counterparty.ID()),
		debt.WithDescription("Test debt for services"),
		debt.WithDueDate(&dueDate),
	)

	createdDebt, err := debtService.Create(f.Ctx, debtEntity)
	require.NoError(t, err)
	require.NotNil(t, createdDebt)
	require.Equal(t, debt.DebtTypeReceivable, createdDebt.Type())
	require.Equal(t, originalAmount.Amount(), createdDebt.OriginalAmount().Amount())
	require.Equal(t, originalAmount.Amount(), createdDebt.OutstandingAmount().Amount())
	require.Equal(t, debt.DebtStatusPending, createdDebt.Status())

	// Test GetByID
	retrievedDebt, err := debtService.GetByID(f.Ctx, createdDebt.ID())
	require.NoError(t, err)
	require.Equal(t, createdDebt.ID(), retrievedDebt.ID())
	require.Equal(t, createdDebt.OriginalAmount().Amount(), retrievedDebt.OriginalAmount().Amount())

	// Test GetAll
	allDebts, err := debtService.GetAll(f.Ctx)
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(allDebts), 1)

	// Test GetByCounterpartyID
	counterpartyDebts, err := debtService.GetByCounterpartyID(f.Ctx, counterparty.ID())
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(counterpartyDebts), 1)

	// Test Update
	updatedDescription := "Updated debt description"
	updatedDebt := createdDebt.UpdateDescription(updatedDescription)
	finalUpdatedDebt, err := debtService.Update(f.Ctx, updatedDebt)
	require.NoError(t, err)
	require.Equal(t, updatedDescription, finalUpdatedDebt.Description())

	// Test Count
	count, err := debtService.Count(f.Ctx)
	require.NoError(t, err)
	require.GreaterOrEqual(t, count, int64(1))

	// Test Delete
	deletedDebt, err := debtService.Delete(f.Ctx, createdDebt.ID())
	require.NoError(t, err)
	require.Equal(t, createdDebt.ID(), deletedDebt.ID())

	// Verify deletion
	_, err = debtService.GetByID(f.Ctx, createdDebt.ID())
	require.Error(t, err)
}

func TestDebtService_Settle(t *testing.T) {
	t.Parallel()
	f := setupTest(t,
		permissions.DebtCreate,
		permissions.DebtRead,
		permissions.DebtUpdate,
	)

	counterparty := setupDebtTestData(f.Ctx, t)
	debtService := getDebtService(f)

	// Get tenant ID from context
	tenantID, err := composables.UseTenantID(f.Ctx)
	require.NoError(t, err)

	// Create a debt
	originalAmount := money.New(100000, "USD") // $1000.00
	debtEntity := debt.New(
		debt.DebtTypeReceivable,
		originalAmount,
		debt.WithTenantID(tenantID),
		debt.WithCounterpartyID(counterparty.ID()),
		debt.WithDescription("Test debt for settlement"),
	)

	createdDebt, err := debtService.Create(f.Ctx, debtEntity)
	require.NoError(t, err)

	// Test partial settlement
	settlementAmount := 300.0 // $300.00
	settledDebt, err := debtService.Settle(f.Ctx, createdDebt.ID(), settlementAmount, nil)
	require.NoError(t, err)
	require.Equal(t, debt.DebtStatusPartial, settledDebt.Status())

	expectedOutstanding := money.New(70000, "USD") // $1000 - $300 = $700
	require.Equal(t, expectedOutstanding.Amount(), settledDebt.OutstandingAmount().Amount())

	// Test full settlement
	remainingAmount := settledDebt.OutstandingAmount().AsMajorUnits()
	fullySettledDebt, err := debtService.Settle(f.Ctx, settledDebt.ID(), remainingAmount, nil)
	require.NoError(t, err)
	require.Equal(t, debt.DebtStatusSettled, fullySettledDebt.Status())
	require.Equal(t, int64(0), fullySettledDebt.OutstandingAmount().Amount())
}

func TestDebtService_WriteOff(t *testing.T) {
	t.Parallel()
	f := setupTest(t,
		permissions.DebtCreate,
		permissions.DebtRead,
		permissions.DebtUpdate,
	)

	counterparty := setupDebtTestData(f.Ctx, t)
	debtService := getDebtService(f)

	// Get tenant ID from context
	tenantID, err := composables.UseTenantID(f.Ctx)
	require.NoError(t, err)

	// Create a debt
	originalAmount := money.New(100000, "USD") // $1000.00
	debtEntity := debt.New(
		debt.DebtTypeReceivable,
		originalAmount,
		debt.WithTenantID(tenantID),
		debt.WithCounterpartyID(counterparty.ID()),
		debt.WithDescription("Test debt for write-off"),
	)

	createdDebt, err := debtService.Create(f.Ctx, debtEntity)
	require.NoError(t, err)

	// Test write-off
	writtenOffDebt, err := debtService.WriteOff(f.Ctx, createdDebt.ID())
	require.NoError(t, err)
	require.Equal(t, debt.DebtStatusWrittenOff, writtenOffDebt.Status())
	require.Equal(t, int64(0), writtenOffDebt.OutstandingAmount().Amount())
}
