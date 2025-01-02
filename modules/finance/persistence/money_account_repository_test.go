package persistence_test

import (
	"context"
	currency "github.com/iota-uz/iota-sdk/modules/core/domain/entities/currency"
	"github.com/iota-uz/iota-sdk/modules/core/infrastructure/persistence"
	moneyAccount "github.com/iota-uz/iota-sdk/modules/finance/domain/aggregates/money_account"
	financepersistence "github.com/iota-uz/iota-sdk/modules/finance/persistence"
	"github.com/iota-uz/iota-sdk/pkg/testutils"
	"github.com/jackc/pgx/v5"
	"testing"
	"time"
)

func TestGormMoneyAccountRepository_CRUD(t *testing.T) { //nolint:paralleltest
	ctx := testutils.GetTestContext()
	defer func(Tx pgx.Tx, ctx context.Context) {
		err := Tx.Commit(ctx)
		if err != nil {
			t.Fatal(err)
		}
	}(ctx.Tx, ctx.Context)
	currencyRepository := persistence.NewCurrencyRepository()
	accountRepository := financepersistence.NewMoneyAccountRepository()

	if err := currencyRepository.Create(ctx.Context, &currency.USD); err != nil {
		t.Fatal(err)
	}
	createdAccount, err := accountRepository.Create(
		ctx.Context, &moneyAccount.Account{
			Name:          "test",
			AccountNumber: "123",
			Currency:      currency.USD,
			Balance:       100,
			Description:   "",
			CreatedAt:     time.Now(),
			UpdatedAt:     time.Now(),
		},
	)
	if err != nil {
		t.Fatal(err)
	}

	t.Run( //nolint:paralleltest
		"Count", func(t *testing.T) {
			count, err := accountRepository.Count(ctx.Context)
			if err != nil {
				t.Fatal(err)
			}
			if count != 1 {
				t.Errorf("expected 1, got %d", count)
			}
		},
	)

	t.Run( //nolint:paralleltest
		"GetPaginated", func(t *testing.T) {
			accounts, err := accountRepository.GetPaginated(ctx.Context, &moneyAccount.FindParams{Limit: 1})
			if err != nil {
				t.Fatal(err)
			}
			if len(accounts) != 1 {
				t.Errorf("expected 1, got %d", len(accounts))
			}
			if accounts[0].Balance != 100 {
				t.Errorf("expected 100, got %f", accounts[0].Balance)
			}
		},
	)

	t.Run( //nolint:paralleltest
		"GetAll", func(t *testing.T) {
			accounts, err := accountRepository.GetAll(ctx.Context)
			if err != nil {
				t.Fatal(err)
			}
			if len(accounts) != 1 {
				t.Errorf("expected 1, got %d", len(accounts))
			}
			if accounts[0].Balance != 100 {
				t.Errorf("expected 100, got %f", accounts[0].Balance)
			}
		},
	)

	t.Run( //nolint:paralleltest
		"GetByID", func(t *testing.T) {
			accountEntity, err := accountRepository.GetByID(ctx.Context, 1)
			if err != nil {
				t.Fatal(err)
			}
			if accountEntity.Balance != 100 {
				t.Errorf("expected 100, got %f", accountEntity.Balance)
			}
			if accountEntity.Currency.Code != currency.UsdCode {
				t.Errorf("expected %s, got %s", currency.UsdCode, accountEntity.Currency.Code)
			}
		},
	)

	t.Run( //nolint:paralleltest
		"Update", func(t *testing.T) {
			if err := accountRepository.Update(
				ctx.Context, &moneyAccount.Account{
					ID:      createdAccount.ID,
					Balance: 200,
				},
			); err != nil {
				t.Fatal(err)
			}
			accountEntity, err := accountRepository.GetByID(ctx.Context, 1)
			if err != nil {
				t.Fatal(err)
			}
			if accountEntity.Balance != 200 {
				t.Errorf("expected 200, got %f", accountEntity.Balance)
			}
			if accountEntity.Currency.Code != currency.UsdCode {
				t.Errorf("expected %s, got %s", currency.UsdCode, accountEntity.Currency.Code)
			}
		},
	)
}
