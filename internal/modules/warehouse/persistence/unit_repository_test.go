package persistence_test

import (
	"github.com/iota-agency/iota-sdk/internal/modules/warehouse/domain/entities/unit"
	"github.com/iota-agency/iota-sdk/internal/modules/warehouse/persistence"
	"github.com/iota-agency/iota-sdk/internal/testutils"
	"testing"
	"time"
)

func TestGormMoneyAccountRepository_CRUD(t *testing.T) { //nolint:paralleltest
	ctx := testutils.GetTestContext()
	defer ctx.Tx.Commit()
	unitRepository := persistence.NewUnitRepository()

	if err := unitRepository.Create(
		ctx.Context, &unit.Unit{
			ID:         1,
			Title:      "test",
			ShortTitle: "t",
			CreatedAt:  time.Now(),
			UpdatedAt:  time.Now(),
		},
	); err != nil {
		t.Fatal(err)
	}

	t.Run( //nolint:paralleltest
		"Count", func(t *testing.T) {
			count, err := unitRepository.Count(ctx.Context)
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
			accounts, err := unitRepository.GetPaginated(ctx.Context, 1, 0, []string{})
			if err != nil {
				t.Fatal(err)
			}
			if len(accounts) != 1 {
				t.Errorf("expected 1, got %d", len(accounts))
			}
		},
	)

	t.Run( //nolint:paralleltest
		"GetAll", func(t *testing.T) {
			units, err := unitRepository.GetAll(ctx.Context)
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

	t.Run( //nolint:paralleltest
		"GetByID", func(t *testing.T) {
			unitEntity, err := unitRepository.GetByID(ctx.Context, 1)
			if err != nil {
				t.Fatal(err)
			}
			if unitEntity.Title != "test" {
				t.Errorf("expected test, got %s", unitEntity.Title)
			}
		},
	)

	t.Run( //nolint:paralleltest
		"Update", func(t *testing.T) {
			if err := unitRepository.Update(
				ctx.Context, &unit.Unit{
					ID:         1,
					Title:      "test2",
					ShortTitle: "t2",
					CreatedAt:  time.Now(),
					UpdatedAt:  time.Now(),
				},
			); err != nil {
				t.Fatal(err)
			}
			unitEntity, err := unitRepository.GetByID(ctx.Context, 1)
			if err != nil {
				t.Fatal(err)
			}
			if unitEntity.Title != "test2" {
				t.Errorf("expected test2, got %s", unitEntity.Title)
			}
		},
	)
}
