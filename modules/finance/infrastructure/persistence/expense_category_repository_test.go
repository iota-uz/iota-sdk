package persistence_test

import (
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/currency"
	corepersistence "github.com/iota-uz/iota-sdk/modules/core/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/modules/finance/domain/aggregates/expense_category"
	"github.com/iota-uz/iota-sdk/modules/finance/infrastructure/persistence"
	"testing"
)

func TestGormExpenseCategoryRepository_CRUD(t *testing.T) {
	f := setupTest(t)
	currencyRepository := corepersistence.NewCurrencyRepository()
	categoryRepository := persistence.NewExpenseCategoryRepository()

	if err := currencyRepository.Create(f.ctx, &currency.USD); err != nil {
		t.Fatal(err)
	}
	createdCategory, err := categoryRepository.Create(
		f.ctx,
		category.New(
			"test",
			"test",
			100,
			&currency.USD,
		),
	)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("Count", func(t *testing.T) {
		count, err := categoryRepository.Count(f.ctx)
		if err != nil {
			t.Fatal(err)
		}
		if count != 1 {
			t.Errorf("expected 1, got %d", count)
		}
	})

	t.Run("GetPaginated", func(t *testing.T) {
		categories, err := categoryRepository.GetPaginated(f.ctx, &category.FindParams{})
		if err != nil {
			t.Fatal(err)
		}
		if len(categories) != 1 {
			t.Errorf("expected 1, got %d", len(categories))
		}
		if categories[0].Amount() != 100 {
			t.Errorf("expected 100, got %f", categories[0].Amount())
		}
	})

	t.Run("GetAll", func(t *testing.T) {
		categories, err := categoryRepository.GetAll(f.ctx)
		if err != nil {
			t.Fatal(err)
		}
		if len(categories) != 1 {
			t.Errorf("expected 1, got %d", len(categories))
		}
		if categories[0].Amount() != 100 {
			t.Errorf("expected 100, got %f", categories[0].Amount())
		}
	})

	t.Run("GetByID", func(t *testing.T) {
		categoryEntity, err := categoryRepository.GetByID(f.ctx, createdCategory.ID())
		if err != nil {
			t.Fatal(err)
		}
		if categoryEntity.Amount() != 100 {
			t.Errorf("expected 100, got %f", categoryEntity.Amount())
		}
		if categoryEntity.Currency().Code != currency.UsdCode {
			t.Errorf("expected %s, got %s", currency.UsdCode, categoryEntity.Currency().Code)
		}
	})

	t.Run("Update", func(t *testing.T) {
		updatedCategory, err := categoryRepository.Update(f.ctx, createdCategory.UpdateAmount(200))
		if err != nil {
			t.Fatal(err)
		}
		if updatedCategory.Amount() != 200 {
			t.Errorf("expected 200, got %f", updatedCategory.Amount())
		}
		if updatedCategory.Currency().Code != currency.UsdCode {
			t.Errorf("expected %s, got %s", currency.UsdCode, updatedCategory.Currency().Code)
		}
	})
}
