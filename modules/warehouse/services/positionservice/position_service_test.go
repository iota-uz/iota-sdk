package positionservice_test

import (
	"context"
	persistence2 "github.com/iota-uz/iota-sdk/modules/core/infrastructure/persistence"
	coreservices "github.com/iota-uz/iota-sdk/modules/core/services"
	persistence3 "github.com/iota-uz/iota-sdk/modules/warehouse/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/modules/warehouse/permissions"
	"github.com/iota-uz/iota-sdk/modules/warehouse/services"
	"github.com/iota-uz/iota-sdk/modules/warehouse/services/positionservice"
	"github.com/iota-uz/iota-sdk/modules/warehouse/services/productservice"
	"github.com/iota-uz/iota-sdk/pkg/constants"
	"github.com/iota-uz/iota-sdk/pkg/event"
	"github.com/iota-uz/iota-sdk/pkg/testutils"
	"github.com/xuri/excelize/v2"
	"log"
	"os"
	"testing"
)

var (
	TestFilePath = "test.xlsx"
	Data         = []map[string]interface{}{
		{"A1": "Наименование", "B1": "Код в справочнике", "C1": "Ед. изм.", "D1": "Количество"},
		{"A2": "Дрель Молоток N.C.V (900W)", "B2": "3241324132", "C2": "шт", "D2": 10},
		{"A3": "Дрель Молоток N.C.V (900W)", "B3": "9230891234", "C3": "шт", "D3": 10},
		{"A4": "Дрель Молоток N.C.V (900W)", "B4": "3242198021", "C4": "шт", "D4": 30_000},
	}
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
		log.Println(err)
	}
	os.Exit(code)
}

func createTestFile(path string) error {
	f := excelize.NewFile()
	defer func() {
		if err := f.Close(); err != nil {
			log.Println(err)
		}
	}()
	for _, v := range Data {
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

	unitRepo := persistence3.NewUnitRepository()
	testCtx.App.RegisterServices(services.NewUnitService(unitRepo, publisher))

	positionRepo := persistence3.NewPositionRepository(unitRepo)
	productRepo := persistence3.NewProductRepository(positionRepo)
	testCtx.App.RegisterServices(productservice.NewProductService(productRepo, publisher))

	uploadRepo := persistence2.NewUploadRepository()
	storage, err := persistence2.NewFSStorage()
	if err != nil {
		t.Error(err)
	}
	testCtx.App.RegisterServices(coreservices.NewUploadService(uploadRepo, storage, publisher))

	positionService := positionservice.NewPositionService(positionRepo, publisher, testCtx.App)

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
	if len(positions) != len(Data)-1 {
		t.Errorf("expected %d, got %d", len(Data)-1, len(positions))
	}
	if positions[0].Title != Data[1]["A2"] {
		t.Errorf("expected %s, got %s", Data[1]["A2"], positions[0].Title)
	}

	if positions[0].Barcode != Data[1]["B2"] {
		t.Errorf("expected %s, got %s", Data[1]["B2"], positions[0].Barcode)
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
	if len(products) != 30_020 {
		t.Errorf("expected %d, got %d", 30_020, len(products))
	}
}
