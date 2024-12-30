package orderservice_test

import (
	"context"
	"github.com/iota-uz/iota-sdk/modules/warehouse/domain/aggregates/order"
	"github.com/iota-uz/iota-sdk/modules/warehouse/domain/aggregates/position"
	"github.com/iota-uz/iota-sdk/modules/warehouse/domain/aggregates/product"
	"github.com/iota-uz/iota-sdk/modules/warehouse/domain/entities/unit"
	"github.com/iota-uz/iota-sdk/modules/warehouse/permissions"
	"github.com/iota-uz/iota-sdk/modules/warehouse/persistence"
	orderservice "github.com/iota-uz/iota-sdk/modules/warehouse/services/order_service"
	"github.com/iota-uz/iota-sdk/pkg/constants"
	"github.com/iota-uz/iota-sdk/pkg/testutils"
	"os"
	"testing"
	"time"
)

func TestMain(m *testing.M) {
	if err := os.Chdir("../../../../"); err != nil {
		panic(err)
	}
	code := m.Run()
	os.Exit(code)
}

func TestPositionService_LoadFromFilePath(t *testing.T) {
	testCtx := testutils.GetTestContext()
	defer testCtx.Tx.Commit()

	unitRepo := persistence.NewUnitRepository()
	positionRepo := persistence.NewPositionRepository(unitRepo)
	productRepo := persistence.NewProductRepository(positionRepo)
	orderRepo := persistence.NewOrderRepository(productRepo)
	orderService := orderservice.NewOrderService(testCtx.App.EventPublisher(), orderRepo, productRepo)

	if err := unitRepo.Create(testCtx.Context, &unit.Unit{
		ID:         1,
		Title:      "Test Unit",
		ShortTitle: "TU",
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}); err != nil {
		t.Fatal(err)
	}

	positionEntity := position.Position{
		ID:        1,
		Title:     "Test Position",
		Barcode:   "1234567890",
		UnitID:    1,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if err := positionRepo.Create(testCtx.Context, &positionEntity); err != nil {
		t.Fatal(err)
	}

	domainOrder := order.New(order.TypeIn, order.Pending)
	if err := domainOrder.AddItem(
		positionEntity,
		product.New("EPS:1234567890", 1, product.Approved, &positionEntity),
	); err != nil {
		t.Fatal(err)
	}

	if err := orderRepo.Create(testCtx.Context, domainOrder); err != nil {
		t.Fatal(err)
	}

	ctx := context.WithValue(testCtx.Context, constants.UserKey, testutils.MockUser(
		permissions.PositionCreate,
		permissions.PositionRead,
		permissions.ProductCreate,
		permissions.ProductRead,
		permissions.UnitCreate,
		permissions.UnitRead,
	))
	ctx = context.WithValue(ctx, constants.SessionKey, testutils.MockSession())

	_, err := orderService.Complete(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}

	orderEntity, err := orderRepo.GetByID(ctx, 1)
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
