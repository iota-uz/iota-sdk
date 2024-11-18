package seed

import (
	"context"
	"fmt"
	"github.com/iota-agency/iota-erp/internal/application"
	"github.com/iota-agency/iota-erp/internal/modules/warehouse/domain/entities/position"
	"github.com/iota-agency/iota-erp/internal/modules/warehouse/domain/entities/unit"
	"github.com/iota-agency/iota-erp/internal/modules/warehouse/persistence"
	"time"
)

func SeedPositions(ctx context.Context, app *application.Application) error {
	unitRepository := persistence.NewUnitRepository()
	positionRepository := persistence.NewPositionRepository()

	if err := unitRepository.CreateOrUpdate(ctx, &unit.Unit{
		ID:         1,
		Title:      "Centimeter",
		ShortTitle: "cm",
	}); err != nil {
		return err
	}

	for i := range 20000 {
		if err := positionRepository.CreateOrUpdate(ctx, &position.Position{
			ID:        uint(i),
			Title:     fmt.Sprintf("Position %d", i),
			Barcode:   fmt.Sprintf("%d", i),
			UnitID:    1,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}); err != nil {
			return err
		}
	}
	return nil
}
