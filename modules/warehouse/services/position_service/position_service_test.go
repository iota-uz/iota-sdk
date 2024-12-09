package position_service_test

import (
	"context"
	"fmt"
	"github.com/iota-agency/iota-sdk/modules/warehouse/permissions"
	"github.com/iota-agency/iota-sdk/modules/warehouse/persistence"
	"github.com/iota-agency/iota-sdk/modules/warehouse/services"
	"github.com/iota-agency/iota-sdk/modules/warehouse/services/position_service"
	"github.com/iota-agency/iota-sdk/modules/warehouse/services/product_service"
	"github.com/iota-agency/iota-sdk/pkg/constants"
	"github.com/iota-agency/iota-sdk/pkg/event"
	corepersistence "github.com/iota-agency/iota-sdk/pkg/infrastructure/persistence"
	coreservices "github.com/iota-agency/iota-sdk/pkg/services"
	"github.com/iota-agency/iota-sdk/pkg/testutils"
	"github.com/xuri/excelize/v2"
	"os"
	"testing"
)

var (
	TestFilePath = "test.xlsx"
)

func TestMain(m *testing.M) {
	if err := os.Chdir("../../../../"); err != nil {
		panic(err)
	}
	if err := createTestFile(TestFilePath); err != nil {
		panic(err)
	}
	code := m.Run()
	if err := os.Remove(TestFilePath); err != nil {
		fmt.Println(err)
	}
	os.Exit(code)
}

func createTestFile(path string) error {
	f := excelize.NewFile()
	defer func() {
		if err := f.Close(); err != nil {
			fmt.Println(err)
		}
	}()
	var data = []map[string]interface{}{
		{"A1": "Наименование", "B1": "Код в справочнике", "C1": "Ед. изм.", "D1": "Количество"},
		{"A2": "Дрель Молоток N.C.V (900W)", "B2": "3241324132", "C2": "шт", "D2": 10},
		{"A3": "Дрель Молоток N.C.V (900W)", "B3": "9230891234", "C3": "шт", "D3": 10},
		{"A4": "Дрель Молоток N.C.V (900W)", "B4": "3242198021", "C4": "шт", "D4": 10},
	}
	for _, v := range data {
		for k, val := range v {
			if err := f.SetCellValue("Sheet1", k, val); err != nil {
				return err
			}
		}
	}
	return f.SaveAs(path)
}

func TestPositionService_LoadFromFilePath(t *testing.T) {
	testCtx := testutils.GetTestContext()
	publisher := event.NewEventPublisher()

	unitRepo := persistence.NewUnitRepository()
	unitService := services.NewUnitService(unitRepo, publisher)
	testCtx.App.RegisterService(unitService)

	productRepo := persistence.NewProductRepository()
	productService := product_service.NewProductService(productRepo, publisher)
	testCtx.App.RegisterService(productService)

	uploadRepo := corepersistence.NewUploadRepository()
	storage, err := corepersistence.NewFSStorage()
	if err != nil {
		t.Error(err)
	}
	uploadService := coreservices.NewUploadService(uploadRepo, storage, publisher)
	testCtx.App.RegisterService(uploadService)

	positionRepo := persistence.NewPositionRepository()
	positionService := position_service.NewPositionService(positionRepo, publisher, testCtx.App)

	ctx := context.WithValue(testCtx.Context, constants.UserKey, testutils.MockUser(
		permissions.PositionCreate,
		permissions.PositionRead,
		permissions.ProductCreate,
		permissions.ProductRead,
		permissions.UnitCreate,
		permissions.UnitRead,
	))
	ctx = context.WithValue(ctx, constants.SessionKey, testutils.MockSession())

	if err := positionService.LoadFromFilePath(ctx, TestFilePath); err != nil {
		t.Error(err)
	}

	positions, err := positionRepo.GetAll(ctx)
	if err != nil {
		t.Error(err)
	}
	if len(positions) != 3 {
		t.Errorf("expected %d, got %d", 3, len(positions))
	}
	if positions[0].Title != "Дрель Молоток N.C.V (900W)" {
		t.Errorf("expected %s, got %s", "Дрель Молоток N.C.V (900W)", positions[0].Title)
	}

	if positions[0].Barcode != "3241324132" {
		t.Errorf("expected %s, got %s", "3241324132", positions[0].Barcode)
	}

	units, err := unitRepo.GetAll(ctx)
	if err != nil {
		t.Error(err)
	}
	if len(units) != 1 {
		t.Errorf("expected %d, got %d", 1, len(units))
	}

	products, err := productRepo.GetAll(ctx)
	if err != nil {
		t.Error(err)
	}
	if len(products) != 30 {
		t.Errorf("expected %d, got %d", 30, len(products))
	}
}
