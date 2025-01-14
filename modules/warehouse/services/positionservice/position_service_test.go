package positionservice_test

import (
	"context"
	"log"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/xuri/excelize/v2"

	"github.com/iota-uz/iota-sdk/modules"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/session"
	"github.com/iota-uz/iota-sdk/modules/warehouse/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/modules/warehouse/permissions"
	"github.com/iota-uz/iota-sdk/modules/warehouse/services/positionservice"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/testutils"
)

var (
	TestFilePath = "test.xlsx"
	Data         = []map[string]interface{}{
		{"A1": "Наименование", "B1": "Код в справочнике", "C1": "Ед. изм.", "D1": "Количество"},
		{"A2": "Дрель Молоток N.C.V (900W)", "B2": "3241324132", "C2": "шт", "D2": 10},
		{"A3": "Дрель Молоток N.C.V (900W)", "B3": "9230891234", "C3": "шт", "D3": 10},
		{"A4": "Дрель Молоток N.C.V (900W)", "B4": "3242198021", "C4": "шт", "D4": 3},
	}
	TotalProducts = 23
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

// testFixtures contains common test dependencies
type testFixtures struct {
	ctx  context.Context
	pool *pgxpool.Pool
	app  application.Application
}

// setupTest creates all necessary dependencies for tests
func setupTest(t *testing.T) *testFixtures {
	t.Helper()

	testutils.CreateDB(t.Name())
	pool := testutils.NewPool(testutils.DbOpts(t.Name()))

	ctx := composables.WithUser(context.Background(), testutils.MockUser(
		permissions.PositionCreate,
		permissions.PositionRead,
		permissions.ProductCreate,
		permissions.ProductRead,
		permissions.UnitCreate,
		permissions.UnitRead,
	))
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		if err := tx.Commit(ctx); err != nil {
			t.Fatal(err)
		}
		pool.Close()
	})

	ctx = composables.WithTx(ctx, tx)
	ctx = composables.WithSession(ctx, &session.Session{})

	app, err := testutils.SetupApplication(pool, modules.BuiltInModules...)
	if err != nil {
		t.Fatal(err)
	}

	return &testFixtures{
		ctx:  ctx,
		pool: pool,
		app:  app,
	}
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
	t.Parallel()
	f := setupTest(t)

	positionService := f.app.Service(positionservice.PositionService{}).(*positionservice.PositionService)

	if err := positionService.LoadFromFilePath(f.ctx, TestFilePath); err != nil {
		t.Fatal(err)
	}

	unitRepo := persistence.NewUnitRepository()
	positionRepo := persistence.NewPositionRepository()
	productRepo := persistence.NewProductRepository()

	positions, err := positionRepo.GetAll(f.ctx)
	if err != nil {
		t.Error(err)
	}
	if len(positions) != len(Data)-1 {
		t.Fatalf("expected %d, got %d", len(Data)-1, len(positions))
	}

	if positions[0].Title != Data[1]["A2"] {
		t.Errorf("expected %s, got %s", Data[1]["A2"], positions[0].Title)
	}

	if positions[0].Barcode != Data[1]["B2"] {
		t.Errorf("expected %s, got %s", Data[1]["B2"], positions[0].Barcode)
	}

	units, err := unitRepo.GetAll(f.ctx)
	if err != nil {
		t.Error(err)
	}
	if len(units) != 1 {
		t.Errorf("expected %d, got %d", 1, len(units))
	}

	products, err := productRepo.GetAll(f.ctx)
	if err != nil {
		t.Error(err)
	}
	if len(products) != TotalProducts {
		t.Errorf("expected %d, got %d", TotalProducts, len(products))
	}
}
