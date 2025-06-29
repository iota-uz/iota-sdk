package persistence_test

import (
	"testing"
	"time"

	"github.com/iota-uz/iota-sdk/modules/warehouse/domain/aggregates/order"
	"github.com/iota-uz/iota-sdk/modules/warehouse/domain/aggregates/position"
	"github.com/iota-uz/iota-sdk/modules/warehouse/domain/aggregates/product"
	"github.com/iota-uz/iota-sdk/modules/warehouse/domain/entities/unit"
	"github.com/iota-uz/iota-sdk/modules/warehouse/infrastructure/persistence"
)

func TestGormOrderRepository_CRUD(t *testing.T) {
	t.Parallel()
	f := setupTest(t)

	unitRepository := persistence.NewUnitRepository()
	positionRepository := persistence.NewPositionRepository()
	productRepo := persistence.NewProductRepository()
	orderRepository := persistence.NewOrderRepository(productRepo)

	if err := unitRepository.Create(
		f.Ctx, &unit.Unit{
			ID:         1,
			Title:      "Unit 1",
			ShortTitle: "U1",
			CreatedAt:  time.Now(),
			UpdatedAt:  time.Now(),
		}); err != nil {
		t.Fatal(err)
	}
	positionEntity := &position.Position{
		ID:        1,
		Title:     "Position 1",
		Barcode:   "3141592653589",
		UnitID:    1,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := positionRepository.Create(f.Ctx, positionEntity); err != nil {
		t.Fatal(err)
	}

	orderEntity := order.New(order.TypeIn, order.Pending)
	if err := orderEntity.AddItem(
		positionEntity,
		product.New("EPS:242323", 1, product.Approved, positionEntity),
	); err != nil {
		t.Fatal(err)
	}
	if err := orderRepository.Create(f.Ctx, orderEntity); err != nil {
		t.Fatal(err)
	}

	t.Run(
		"Count", func(t *testing.T) {
			count, err := orderRepository.Count(f.Ctx)
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
			orders, err := orderRepository.GetPaginated(f.Ctx, &order.FindParams{
				Limit:  1,
				Offset: 0,
				SortBy: []string{"id desc"},
			})
			if err != nil {
				t.Fatal(err)
			}
			if len(orders) != 1 {
				t.Errorf("expected 1, got %d", len(orders))
			}
			if len(orders[0].Items()) != 1 {
				t.Errorf("expected 1, got %d", len(orders[0].Items()))
			}
		},
	)

	t.Run(
		"GetAll", func(t *testing.T) {
			orders, err := orderRepository.GetAll(f.Ctx)
			if err != nil {
				t.Fatal(err)
			}
			if len(orders) != 1 {
				t.Errorf("expected 1, got %d", len(orders))
			}
			firstOrder := orders[0]
			if len(firstOrder.Items()) != 1 {
				t.Errorf("expected 1, got %d", len(firstOrder.Items()))
			}
			if firstOrder.Status() != order.Pending {
				t.Errorf("expected %s, got %s", order.Pending, firstOrder.Status())
			}
		},
	)

	t.Run(
		"GetByID", func(t *testing.T) {
			entity, err := orderRepository.GetByID(f.Ctx, 1)
			if err != nil {
				t.Fatal(err)
			}
			if entity.Status() != order.Pending {
				t.Errorf("expected %s, got %s", order.Pending, entity.Status())
			}
			if len(entity.Items()) != 1 {
				t.Errorf("expected 1, got %d", len(entity.Items()))
			}
		},
	)

	t.Run(
		"Update", func(t *testing.T) {
			entity, err := orderRepository.GetByID(f.Ctx, 1)
			if err != nil {
				t.Fatal(err)
			}
			if err := entity.Complete(); err != nil {
				t.Fatal(err)
			}
			if err := orderRepository.Update(f.Ctx, entity); err != nil {
				t.Fatal(err)
			}
			updatedOrder, err := orderRepository.GetByID(f.Ctx, 1)
			if err != nil {
				t.Fatal(err)
			}
			if updatedOrder.Status() != order.Complete {
				t.Errorf("expected %s, got %s", order.Complete, updatedOrder.Status())
			}
			if len(updatedOrder.Items()) != 1 {
				t.Fatalf("expected 1, got %d", len(updatedOrder.Items()))
			}
			item := updatedOrder.Items()[0]
			if item.Products()[0].Status != product.InStock {
				t.Errorf("expected %s, got %s", product.InStock, item.Products()[0].Status)
			}
		},
	)

	t.Run(
		"Delete", func(t *testing.T) {
			if err := orderRepository.Delete(f.Ctx, 1); err != nil {
				t.Fatal(err)
			}
			count, err := orderRepository.Count(f.Ctx)
			if err != nil {
				t.Fatal(err)
			}
			if count != 0 {
				t.Errorf("expected 0, got %d", count)
			}
		},
	)
}
