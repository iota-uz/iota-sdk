package persistence_test

import (
	"testing"

	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/currency"
	"github.com/iota-uz/iota-sdk/modules/core/infrastructure/persistence"
	moneyaccount "github.com/iota-uz/iota-sdk/modules/finance/domain/aggregates/money_account"
	financepersistence "github.com/iota-uz/iota-sdk/modules/finance/infrastructure/persistence"
)

func TestGormMoneyAccountRepository_CRUD(t *testing.T) {
	t.Parallel()
	f := setupTest(t)
	currencyRepository := persistence.NewCurrencyRepository()
	accountRepository := financepersistence.NewMoneyAccountRepository()

	if err := currencyRepository.Create(f.ctx, &currency.USD); err != nil {
		t.Fatal(err)
	}
	createdAccount, err := accountRepository.Create(
		f.ctx,
		moneyaccount.New(
			"test",
			currency.USD,
			moneyaccount.WithAccountNumber("123"),
			moneyaccount.WithBalance(100),
		),
	)
	if err != nil {
		t.Fatal(err)
	}

	t.Run(
		"Count", func(t *testing.T) {
			count, err := accountRepository.Count(f.ctx)
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
			accounts, err := accountRepository.GetPaginated(f.ctx, &moneyaccount.FindParams{Limit: 1})
			if err != nil {
				t.Fatal(err)
			}
			if len(accounts) != 1 {
				t.Errorf("expected 1, got %d", len(accounts))
			}
			if accounts[0].Balance() != 100 {
				t.Errorf("expected 100, got %f", accounts[0].Balance())
			}
		},
	)

	t.Run(
		"GetAll", func(t *testing.T) {
			accounts, err := accountRepository.GetAll(f.ctx)
			if err != nil {
				t.Fatal(err)
			}
			if len(accounts) != 1 {
				t.Errorf("expected 1, got %d", len(accounts))
			}
			if accounts[0].Balance() != 100 {
				t.Errorf("expected 100, got %f", accounts[0].Balance())
			}
		},
	)

	t.Run(
		"GetByID", func(t *testing.T) {
			accountEntity, err := accountRepository.GetByID(f.ctx, 1)
			if err != nil {
				t.Fatal(err)
			}
			if accountEntity.Balance() != 100 {
				t.Errorf("expected 100, got %f", accountEntity.Balance())
			}
			if accountEntity.Currency().Code != currency.UsdCode {
				t.Errorf("expected %s, got %s", currency.UsdCode, accountEntity.Currency().Code)
			}
		},
	)

	t.Run(
		"Update", func(t *testing.T) {
			updatedAccount := moneyaccount.New(
				"test",
				currency.USD,
				moneyaccount.WithID(createdAccount.ID()),
				moneyaccount.WithAccountNumber("123"),
				moneyaccount.WithBalance(200),
			)
			if err := accountRepository.Update(f.ctx, updatedAccount); err != nil {
				t.Fatal(err)
			}
			accountEntity, err := accountRepository.GetByID(f.ctx, 1)
			if err != nil {
				t.Fatal(err)
			}
			if accountEntity.Balance() != 200 {
				t.Errorf("expected 200, got %f", accountEntity.Balance())
			}
			if accountEntity.Currency().Code != currency.UsdCode {
				t.Errorf("expected %s, got %s", currency.UsdCode, accountEntity.Currency().Code)
			}
		},
	)
}
