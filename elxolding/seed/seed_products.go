package seed

import (
	"context"
	"fmt"
	"github.com/iota-agency/iota-sdk/internal/application"
	"github.com/iota-agency/iota-sdk/internal/modules/warehouse/domain/aggregates/product"
	"github.com/iota-agency/iota-sdk/internal/modules/warehouse/persistence"
	"time"
)

func SeedProducts(ctx context.Context, app *application.Application) error {
	positionRepository := persistence.NewPositionRepository()
	productRepository := persistence.NewProductRepository()

	positions, err := positionRepository.GetAll(ctx)
	if err != nil {
		return err
	}

	for i := range 20000 {
		if err := productRepository.CreateOrUpdate(ctx, &product.Product{
			ID:         uint(i),
			PositionID: positions[i%len(positions)].ID,
			Rfid:       fmt.Sprintf("EPS:%d", i),
			Status:     product.InStock,
			CreatedAt:  time.Now(),
			UpdatedAt:  time.Now(),
		}); err != nil {
			return err
		}
	}
	return nil
}
