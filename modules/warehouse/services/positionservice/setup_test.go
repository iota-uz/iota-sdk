package positionservice_test

import (
	"context"
	"log"
	"os"
	"testing"

	"github.com/iota-uz/iota-sdk/modules"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/session"
	"github.com/iota-uz/iota-sdk/modules/warehouse/permissions"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/testutils"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/xuri/excelize/v2"
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

	// Create the test file for position service tests
	if err := createTestFile(TestFilePath); err != nil {
		panic(err)
	}

	code := m.Run()

	// Clean up the test file
	if err := os.Remove(TestFilePath); err != nil {
		log.Println("Failed to remove test file:", err)
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

	// Setup application and run migrations
	app, err := testutils.SetupApplication(pool, modules.BuiltInModules...)
	if err != nil {
		t.Fatal(err)
	}

	// Run migrations first to create all tables including tenants
	if err := app.Migrations().Run(); err != nil {
		t.Fatal(err)
	}

	// Create a test tenant and add it to the context
	tenant, err := testutils.CreateTestTenant(ctx, pool)
	if err != nil {
		t.Fatal(err)
	}
	ctx = composables.WithTenantID(ctx, tenant.ID)

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
