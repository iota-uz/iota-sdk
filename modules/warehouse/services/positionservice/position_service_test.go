package positionservice_test

import (
	"testing"

	"github.com/iota-uz/iota-sdk/modules/warehouse/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/modules/warehouse/services/positionservice"
)

func TestPositionService_LoadFromFilePath(t *testing.T) {
	t.Parallel()
	f := setupTest(t)

	positionService := f.app.Service(positionservice.PositionService{}).(*positionservice.PositionService)

	if err := positionService.LoadFromFilePath(f.ctx, TestFilePath); err != nil {
		t.Fatal(err)
	}

	unitRepo := persistence.NewUnitRepository()
	positionRepo := persistence.NewPositionRepository()
	productRepo := persistence.NewProductRepository()

	positions, err := positionRepo.GetAll(f.ctx)
	if err != nil {
		t.Error(err)
	}

	if len(positions) != 3 {
		t.Errorf("expected 3 position, got %d", len(positions))
	}

	found := false
	for _, pos := range positions {
		if pos.Title == "Дрель Молоток N.C.V (900W)" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("position with title 'Дрель Молоток N.C.V (900W)' not found")
	}

	units, err := unitRepo.GetAll(f.ctx)
	if err != nil {
		t.Error(err)
	}

	if len(units) != 1 {
		t.Errorf("expected 1 unit, got %d", len(units))
	}

	if units[0].Title != "шт" {
		t.Errorf("expected title %s, got %s", "шт", units[0].Title)
	}

	if units[0].ShortTitle != "шт" {
		t.Errorf("expected short title %s, got %s", "шт", units[0].ShortTitle)
	}

	products, err := productRepo.GetAll(f.ctx)
	if err != nil {
		t.Error(err)
	}

	if len(products) != TotalProducts {
		t.Errorf("expected %d products, got %d", TotalProducts, len(products))
	}
}
