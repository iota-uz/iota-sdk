package persistence_test

import (
	"testing"
	"time"

	"github.com/iota-agency/iota-sdk/modules/warehouse/domain/aggregates/order"
	"github.com/iota-agency/iota-sdk/modules/warehouse/persistence"
	"github.com/iota-agency/iota-sdk/pkg/testutils"
)

func TestGormOrderRepository_CRUD(t *testing.T) { //nolint:paralleltest
	ctx := testutils.GetTestContext()
	defer ctx.Tx.Commit()
	orderRepository := persistence.NewOrderRepository()

	if err := orderRepository.Create(
		ctx.Context, &order.Order{
			ID:        1,
			Status:    order.Pending,
			Type:      order.TypeIn,
			CreatedAt: time.Now(),
			// Add other necessary fields
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
			orders, err := orderRepository.GetPaginated(ctx.Context, 1, 0, []string{})
			if err != nil {
				t.Fatal(err)
			}
			if len(orders) != 1 {
				t.Errorf("expected 1, got %d", len(orders))
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
			// Add other necessary checks
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
