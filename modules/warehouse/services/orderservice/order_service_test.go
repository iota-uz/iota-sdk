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
	orderService := orderservice.NewOrderService(f.App.EventPublisher(), orderRepo, productRepo)

	if err := unitRepo.Create(f.Ctx, &unit.Unit{
		ID:         1,
		Title:      "Test Unit",
		ShortTitle: "TU",
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}); err != nil {
		t.Fatal(err)
	}

	positionEntity := position.New("Test Position", "1234567890",
		position.WithID(1),
		position.WithUnitID(1),
		position.WithCreatedAt(time.Now()),
		position.WithUpdatedAt(time.Now()))

	if err := positionRepo.Create(f.Ctx, positionEntity); err != nil {
		t.Fatal(err)
	}

	domainOrder := order.New(order.TypeIn, order.WithStatus(order.Pending))
	domainOrder, err := domainOrder.AddItem(
		positionEntity,
		product.New("EPS:1234567890", product.InStock, product.WithPosition(positionEntity)),
	)
	if err != nil {
		t.Fatal(err)
	}

	if err := orderRepo.Create(f.Ctx, domainOrder); err != nil {
		t.Fatal(err)
	}

	_, err = orderService.Complete(f.Ctx, 1)
	if err != nil {
		t.Fatal(err)
	}

	orderEntity, err := orderRepo.GetByID(f.Ctx, 1)
	if err != nil {
		t.Fatal(err)
	}

	if orderEntity.Status() != order.Complete {
		t.Fatalf("expected %s, got %s", order.Complete, orderEntity.Status())
	}

	item := orderEntity.Items()[0]
	if item.Products()[0].Status() != product.InStock {
		t.Fatalf("expected %s, got %s", product.InStock, item.Products()[0].Status())
	}
}
