package orderservice_test

import (
	"testing"
	"time"

	"github.com/iota-uz/iota-sdk/modules/warehouse/domain/aggregates/order"
	"github.com/iota-uz/iota-sdk/modules/warehouse/domain/aggregates/position"
	"github.com/iota-uz/iota-sdk/modules/warehouse/domain/aggregates/product"
	"github.com/iota-uz/iota-sdk/modules/warehouse/domain/entities/unit"
	"github.com/iota-uz/iota-sdk/modules/warehouse/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/modules/warehouse/services/orderservice"
)

func TestOrderService(t *testing.T) {
	t.Parallel()
	f := setupTest(t)

	unitRepo := persistence.NewUnitRepository()
	positionRepo := persistence.NewPositionRepository()
	productRepo := persistence.NewProductRepository()
	orderRepo := persistence.NewOrderRepository(productRepo)
	orderService := orderservice.NewOrderService(f.app.EventPublisher(), orderRepo, productRepo)

	if err := unitRepo.Create(f.ctx, &unit.Unit{
		ID:         1,
		Title:      "Test Unit",
		ShortTitle: "TU",
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}); err != nil {
		t.Fatal(err)
	}

	positionEntity := &position.Position{
		ID:        1,
		Title:     "Test Position",
		Barcode:   "1234567890",
		UnitID:    1,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if err := positionRepo.Create(f.ctx, positionEntity); err != nil {
		t.Fatal(err)
	}

	domainOrder := order.New(order.TypeIn, order.Pending)
	if err := domainOrder.AddItem(
		positionEntity,
		product.New("EPS:1234567890", 1, product.Approved, positionEntity),
	); err != nil {
		t.Fatal(err)
	}

	if err := orderRepo.Create(f.ctx, domainOrder); err != nil {
		t.Fatal(err)
	}

	_, err := orderService.Complete(f.ctx, 1)
	if err != nil {
		t.Fatal(err)
	}

	orderEntity, err := orderRepo.GetByID(f.ctx, 1)
	if err != nil {
		t.Fatal(err)
	}

	if orderEntity.Status() != order.Complete {
		t.Fatalf("expected %s, got %s", order.Complete, orderEntity.Status())
	}

	item := orderEntity.Items()[0]
	if item.Products()[0].Status != product.InStock {
		t.Fatalf("expected %s, got %s", product.InStock, item.Products()[0].Status)
	}
}
