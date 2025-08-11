package persistence_test

import (
	"testing"

	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/currency"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/country"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/tax"
	corepersistence "github.com/iota-uz/iota-sdk/modules/core/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/modules/finance/domain/aggregates/debt"
	"github.com/iota-uz/iota-sdk/modules/finance/domain/entities/counterparty"
	"github.com/iota-uz/iota-sdk/modules/finance/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/money"
	"github.com/stretchr/testify/require"
)

func TestGormDebtRepository_CRUD(t *testing.T) {
	t.Parallel()
	f := setupTest(t)

	// Create currency
	currencyRepo := corepersistence.NewCurrencyRepository()
	if err := currencyRepo.Create(f.Ctx, &currency.USD); err != nil {
		t.Fatal(err)
	}

	// Create counterparty first
	counterpartyRepo := persistence.NewCounterpartyRepository()
	tin, err := tax.NewTin("123456789", country.Uzbekistan)
	require.NoError(t, err)

	tenantID, err := composables.UseTenantID(f.Ctx)
	require.NoError(t, err)

	counterpartyEntity := counterparty.New(
		"Test Counterparty",
		counterparty.Customer,
		counterparty.LLC,
		counterparty.WithTenantID(tenantID),
		counterparty.WithTin(tin),
		counterparty.WithLegalAddress("Test Address"),
	)

	createdCounterparty, err := counterpartyRepo.Create(f.Ctx, counterpartyEntity)
	require.NoError(t, err)

	// Test debt repository
	debtRepo := persistence.NewDebtRepository()

	t.Run("Count", func(t *testing.T) {
		count, err := debtRepo.Count(f.Ctx)
		require.NoError(t, err)
		require.GreaterOrEqual(t, count, int64(0))
	})

	t.Run("GetPaginated", func(t *testing.T) {
		params := &debt.FindParams{
			Limit:  10,
			Offset: 0,
		}
		debts, err := debtRepo.GetPaginated(f.Ctx, params)
		require.NoError(t, err)
		require.NotNil(t, debts)
	})

	t.Run("GetAll", func(t *testing.T) {
		debts, err := debtRepo.GetAll(f.Ctx)
		require.NoError(t, err)
		require.NotNil(t, debts)
	})

	t.Run("GetByID", func(t *testing.T) {
		// Create a debt first
		originalAmount := money.New(100000, "USD")
		debtEntity := debt.New(
			debt.DebtTypeReceivable,
			originalAmount,
			debt.WithTenantID(tenantID),
			debt.WithCounterpartyID(createdCounterparty.ID()),
			debt.WithDescription("Test debt"),
		)

		createdDebt, err := debtRepo.Create(f.Ctx, debtEntity)
		require.NoError(t, err)
		require.NotNil(t, createdDebt)

		// Test GetByID
		retrievedDebt, err := debtRepo.GetByID(f.Ctx, createdDebt.ID())
		require.NoError(t, err)
		require.Equal(t, createdDebt.ID(), retrievedDebt.ID())
		require.Equal(t, originalAmount.Amount(), retrievedDebt.OriginalAmount().Amount())
		require.Equal(t, "USD", retrievedDebt.OriginalAmount().Currency().Code)
	})

	t.Run("Update", func(t *testing.T) {
		// Create a debt first
		originalAmount := money.New(100000, "USD")
		debtEntity := debt.New(
			debt.DebtTypeReceivable,
			originalAmount,
			debt.WithTenantID(tenantID),
			debt.WithCounterpartyID(createdCounterparty.ID()),
			debt.WithDescription("Test debt for update"),
		)

		createdDebt, err := debtRepo.Create(f.Ctx, debtEntity)
		require.NoError(t, err)

		// Update the debt
		updatedDescription := "Updated description"
		updatedDebt := createdDebt.UpdateDescription(updatedDescription)

		finalDebt, err := debtRepo.Update(f.Ctx, updatedDebt)
		require.NoError(t, err)
		require.Equal(t, updatedDescription, finalDebt.Description())
	})
}
