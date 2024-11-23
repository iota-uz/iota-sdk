package server

import (
	uploadmodels "github.com/iota-agency/iota-sdk/modules/upload/persistence/models"
	warehousemodels "github.com/iota-agency/iota-sdk/modules/warehouse/persistence/models"
	"github.com/iota-agency/iota-sdk/pkg/infrastructure/persistence/models"
)

var RegisteredModels = []interface{}{
	&uploadmodels.Upload{},              //nolint:exhaustruct
	&models.User{},                      //nolint:exhaustruct
	&models.Payment{},                   //nolint:exhaustruct
	&models.ExpenseCategory{},           //nolint:exhaustruct
	&models.Expense{},                   //nolint:exhaustruct
	&warehousemodels.WarehouseUnit{},    //nolint:exhaustruct
	&warehousemodels.WarehouseOrder{},   //nolint:exhaustruct
	&models.Session{},                   //nolint:exhaustruct
	&models.Role{},                      //nolint:exhaustruct
	&models.Dialogue{},                  //nolint:exhaustruct
	&models.ActionLog{},                 //nolint:exhaustruct
	&models.Currency{},                  //nolint:exhaustruct
	&models.Transaction{},               //nolint:exhaustruct
	&warehousemodels.WarehouseProduct{}, //nolint:exhaustruct
	&models.MoneyAccount{},              //nolint:exhaustruct
}
