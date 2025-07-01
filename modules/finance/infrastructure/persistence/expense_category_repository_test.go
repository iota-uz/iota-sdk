package persistence_test

import (
	"testing"

	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/currency"
	corepersistence "github.com/iota-uz/iota-sdk/modules/core/infrastructure/persistence"
	category "github.com/iota-uz/iota-sdk/modules/finance/domain/aggregates/expense_category"
	"github.com/iota-uz/iota-sdk/modules/finance/infrastructure/persistence"
)

func TestGormExpenseCategoryRepository_CRUD(t *testing.T) {
	f := setupTest(t)
	currencyRepository := corepersistence.NewCurrencyRepository()
	categoryRepository := persistence.NewExpenseCategoryRepository()

	if err := currencyRepository.Create(f.Ctx, &currency.USD); err != nil {
		t.Fatal(err)
	}
	createdCategory, err := categoryRepository.Create(
		f.Ctx,
		category.New(
			"test", // name
			category.WithDescription("test"),
		),
	)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("Count", func(t *testing.T) {
		count, err := categoryRepository.Count(f.Ctx, &category.FindParams{})
		if err != nil {
			t.Fatal(err)
		}
		if count != 1 {
			t.Errorf("expected 1, got %d", count)
		}
	})

	t.Run("GetPaginated", func(t *testing.T) {
		categories, err := categoryRepository.GetPaginated(f.Ctx, &category.FindParams{})
		if err != nil {
			t.Fatal(err)
		}
		if len(categories) != 1 {
			t.Errorf("expected 1, got %d", len(categories))
		}
	})

	t.Run("GetAll", func(t *testing.T) {
		categories, err := categoryRepository.GetAll(f.Ctx)
		if err != nil {
			t.Fatal(err)
		}
		if len(categories) != 1 {
			t.Errorf("expected 1, got %d", len(categories))
		}
	})

	t.Run("GetByID", func(t *testing.T) {
		_, err := categoryRepository.GetByID(f.Ctx, createdCategory.ID())
		if err != nil {
			t.Fatal(err)
		}
	})

	t.Run("Update", func(t *testing.T) {
		updatedCategory, err := categoryRepository.Update(f.Ctx, createdCategory)
		if err != nil {
			t.Fatal(err)
		}
		if updatedCategory.Name() != "test" {
			t.Errorf("expected test, got %s", updatedCategory.Name())
		}
	})
}
