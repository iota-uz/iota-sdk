package persistence_test

import (
	"testing"

	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/currency"
	"github.com/iota-uz/iota-sdk/modules/core/infrastructure/persistence"
	moneyaccount "github.com/iota-uz/iota-sdk/modules/finance/domain/aggregates/money_account"
	financepersistence "github.com/iota-uz/iota-sdk/modules/finance/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/pkg/money"
)

func TestGormMoneyAccountRepository_CRUD(t *testing.T) {
	t.Parallel()
	f := setupTest(t)
	currencyRepository := persistence.NewCurrencyRepository()
	accountRepository := financepersistence.NewMoneyAccountRepository()

	if err := currencyRepository.Create(f.Ctx, &currency.USD); err != nil {
		t.Fatal(err)
	}
	createdAccount, err := accountRepository.Create(
		f.Ctx,
		moneyaccount.New(
			"test",
			money.New(10000, "USD"),
			moneyaccount.WithAccountNumber("123"),
		),
	)
	if err != nil {
		t.Fatal(err)
	}

	t.Run(
		"Count", func(t *testing.T) {
			count, err := accountRepository.Count(f.Ctx, &moneyaccount.FindParams{})
			if err != nil {
				t.Fatal(err)
			}
			if count != 1 {
				t.Errorf("expected 1, got %d", count)
			}
		},
	)

	t.Run(
		"GetPaginated", func(t *testing.T) {
			accounts, err := accountRepository.GetPaginated(f.Ctx, &moneyaccount.FindParams{Limit: 1})
			if err != nil {
				t.Fatal(err)
			}
			if len(accounts) != 1 {
				t.Errorf("expected 1, got %d", len(accounts))
			}
			if accounts[0].Balance().AsMajorUnits() != 100 {
				t.Errorf("expected 100, got %f", accounts[0].Balance().AsMajorUnits())
			}
		},
	)

	t.Run(
		"GetAll", func(t *testing.T) {
			accounts, err := accountRepository.GetAll(f.Ctx)
			if err != nil {
				t.Fatal(err)
			}
			if len(accounts) != 1 {
				t.Errorf("expected 1, got %d", len(accounts))
			}
			if accounts[0].Balance().AsMajorUnits() != 100 {
				t.Errorf("expected 100, got %f", accounts[0].Balance().AsMajorUnits())
			}
		},
	)

	t.Run(
		"GetByID", func(t *testing.T) {
			accountEntity, err := accountRepository.GetByID(f.Ctx, createdAccount.ID())
			if err != nil {
				t.Fatal(err)
			}
			if accountEntity.Balance().AsMajorUnits() != 100 {
				t.Errorf("expected 100, got %f", accountEntity.Balance().AsMajorUnits())
			}
			if accountEntity.Balance().Currency().Code != string(currency.UsdCode) {
				t.Errorf("expected %s, got %s", string(currency.UsdCode), accountEntity.Balance().Currency().Code)
			}
		},
	)

	t.Run(
		"Update", func(t *testing.T) {
			updatedAccount := moneyaccount.New(
				"test",
				money.New(20000, "USD"),
				moneyaccount.WithID(createdAccount.ID()),
				moneyaccount.WithAccountNumber("123"),
			)
			if _, err := accountRepository.Update(f.Ctx, updatedAccount); err != nil {
				t.Fatal(err)
			}
			accountEntity, err := accountRepository.GetByID(f.Ctx, createdAccount.ID())
			if err != nil {
				t.Fatal(err)
			}
			if accountEntity.Balance().AsMajorUnits() != 200 {
				t.Errorf("expected 200, got %f", accountEntity.Balance().AsMajorUnits())
			}
			if accountEntity.Balance().Currency().Code != string(currency.UsdCode) {
				t.Errorf("expected %s, got %s", string(currency.UsdCode), accountEntity.Balance().Currency().Code)
			}
		},
	)
}
