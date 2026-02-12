package positionservice_test

import (
	"os"
	"testing"

	"github.com/iota-uz/iota-sdk/modules"
	"github.com/iota-uz/iota-sdk/modules/warehouse/permissions"
	"github.com/iota-uz/iota-sdk/pkg/itf"
	"github.com/xuri/excelize/v2"
)

var (
	Data = []map[string]interface{}{
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

	code := m.Run()

	os.Exit(code)
}

// setupTest creates all necessary dependencies for tests
func setupTest(t *testing.T) *itf.TestEnvironment {
	t.Helper()

	suite := itf.HTTP(t, modules.BuiltInModules...)
	suite.AsUser(itf.User(permissions.PositionCreate, permissions.PositionRead, permissions.ProductCreate, permissions.ProductRead, permissions.UnitCreate, permissions.UnitRead))
	return suite.Environment()
}

func createTestFile(path string) error {
	f := excelize.NewFile()
	defer func() {
		_ = f.Close()
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
