package persistence_test

import (
	"errors"
	"testing"
	"time"

	"github.com/iota-uz/iota-sdk/modules/warehouse/domain/entities/unit"
	"github.com/iota-uz/iota-sdk/modules/warehouse/infrastructure/persistence"
)

func TestGormUnitRepository_CRUD(t *testing.T) {
	f := setupTest(t)
	unitRepo := persistence.NewUnitRepository()

	if err := unitRepo.Create(
		f.ctx,
		&unit.Unit{
			ID:         1,
			Title:      "test",
			ShortTitle: "t",
			CreatedAt:  time.Now(),
			UpdatedAt:  time.Now(),
		},
	); err != nil {
		t.Fatal(err)
	}

	t.Run(
		"Count", func(t *testing.T) {
			count, err := unitRepo.Count(f.ctx)
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
			accounts, err := unitRepo.GetPaginated(f.ctx, &unit.FindParams{Limit: 1})
			if err != nil {
				t.Fatal(err)
			}
			if len(accounts) != 1 {
				t.Errorf("expected 1, got %d", len(accounts))
			}
		},
	)

	t.Run(
		"GetAll", func(t *testing.T) {
			units, err := unitRepo.GetAll(f.ctx)
			if err != nil {
				t.Fatal(err)
			}
			if len(units) != 1 {
				t.Errorf("expected 1, got %d", len(units))
			}
			if units[0].Title != "test" {
				t.Errorf("expected test, got %s", units[0].Title)
			}
		},
	)

	t.Run(
		"GetByID", func(t *testing.T) {
			unitEntity, err := unitRepo.GetByID(f.ctx, 1)
			if err != nil {
				t.Fatal(err)
			}
			if unitEntity.Title != "test" {
				t.Errorf("expected test, got %s", unitEntity.Title)
			}
		},
	)

	t.Run("GetByTitleOrShortTitle", func(t *testing.T) {
		u1, err := unitRepo.GetByTitleOrShortTitle(f.ctx, "test")
		if err != nil {
			t.Fatal(err)
		}

		if u1.Title != "test" {
			t.Errorf("expected test, got %s", u1.Title)
		}

		if u1.ShortTitle != "t" {
			t.Errorf("expected t, got %s", u1.ShortTitle)
		}

		u2, err := unitRepo.GetByTitleOrShortTitle(f.ctx, "t")
		if err != nil {
			t.Fatal(err)
		}

		if u2.ShortTitle != "t" {
			t.Errorf("expected t, got %s", u2.ShortTitle)
		}

		u3, err := unitRepo.GetByTitleOrShortTitle(f.ctx, "test2")
		if err == nil {
			t.Errorf("expected error, got %v", u3)
		}

		if !errors.Is(err, persistence.ErrUnitNotFound) {
			t.Errorf("expected ErrUnitNotFound, got %v", err)
		}
	})

	t.Run(
		"Update", func(t *testing.T) {
			if err := unitRepo.Update(
				f.ctx, &unit.Unit{
					ID:         1,
					Title:      "test2",
					ShortTitle: "t2",
					CreatedAt:  time.Now(),
					UpdatedAt:  time.Now(),
				},
			); err != nil {
				t.Fatal(err)
			}
			unitEntity, err := unitRepo.GetByID(f.ctx, 1)
			if err != nil {
				t.Fatal(err)
			}
			if unitEntity.Title != "test2" {
				t.Errorf("expected test2, got %s", unitEntity.Title)
			}
		},
	)
}
