package persistence_test

import (
	"github.com/iota-agency/iota-sdk/modules/warehouse/domain/aggregates/position"
	"github.com/iota-agency/iota-sdk/modules/warehouse/domain/aggregates/product"
	"github.com/iota-agency/iota-sdk/modules/warehouse/domain/entities/unit"
	"testing"
	"time"

	"github.com/iota-agency/iota-sdk/modules/warehouse/domain/aggregates/order"
	"github.com/iota-agency/iota-sdk/modules/warehouse/persistence"
	"github.com/iota-agency/iota-sdk/pkg/testutils"
)

func TestGormOrderRepository_CRUD(t *testing.T) { //nolint:paralleltest
	ctx := testutils.GetTestContext()
	defer ctx.Tx.Commit()

	unitRepository := persistence.NewUnitRepository()
	positionRepository := persistence.NewPositionRepository()
	productRepository := persistence.NewProductRepository()
	orderRepository := persistence.NewOrderRepository()

	if err := unitRepository.Create(
		ctx.Context, &unit.Unit{
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
	if err := positionRepository.Create(ctx.Context, positionEntity); err != nil {
		t.Fatal(err)
	}

	productEntity := &product.Product{
		ID:         1,
		PositionID: 1,
		Rfid:       "EPS:321456",
		Status:     product.Approved,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}
	if err := productRepository.Create(ctx.Context, productEntity); err != nil {
		t.Fatal(err)
	}

	if err := orderRepository.Create(
		ctx.Context, &order.Order{
			ID:     1,
			Status: order.Pending,
			Type:   order.TypeIn,
			Items: []order.Item{
				{
					Position: *positionEntity,
					Products: []product.Product{*productEntity},
				},
			},
			CreatedAt: time.Now(),
		},
	); err != nil {
		t.Fatal(err)
	}

	t.Run( //nolint:paralleltest
		"Count", func(t *testing.T) {
			count, err := orderRepository.Count(ctx.Context)
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
			orders, err := orderRepository.GetPaginated(ctx.Context, &order.FindParams{
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
			if len(orders[0].Items) != 1 {
				t.Errorf("expected 1, got %d", len(orders[0].Items))
			}
		},
	)

	t.Run( //nolint:paralleltest
		"GetAll", func(t *testing.T) {
			orders, err := orderRepository.GetAll(ctx.Context)
			if err != nil {
				t.Fatal(err)
			}
			if len(orders) != 1 {
				t.Errorf("expected 1, got %d", len(orders))
			}
			if len(orders[0].Items) != 1 {
				t.Errorf("expected 1, got %d", len(orders[0].Items))
			}
			if orders[0].Status != order.Pending {
				t.Errorf("expected %s, got %s", order.Pending, orders[0].Status)
			}
		},
	)

	t.Run( //nolint:paralleltest
		"GetByID", func(t *testing.T) {
			orderEntity, err := orderRepository.GetByID(ctx.Context, 1)
			if err != nil {
				t.Fatal(err)
			}
			if orderEntity.Status != order.Pending {
				t.Errorf("expected %s, got %s", order.Pending, orderEntity.Status)
			}
			if len(orderEntity.Items) != 1 {
				t.Errorf("expected 1, got %d", len(orderEntity.Items))
			}
		},
	)

	t.Run( //nolint:paralleltest
		"Update", func(t *testing.T) {
			if err := orderRepository.Update(
				ctx.Context, &order.Order{
					ID:     1,
					Status: order.Complete,
				},
			); err != nil {
				t.Fatal(err)
			}
			orderEntity, err := orderRepository.GetByID(ctx.Context, 1)
			if err != nil {
				t.Fatal(err)
			}
			if orderEntity.Status != order.Complete {
				t.Errorf("expected %s, got %s", order.Complete, orderEntity.Status)
			}
		},
	)
}
