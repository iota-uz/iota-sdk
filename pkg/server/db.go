package server

import (
	models2 "github.com/iota-agency/iota-sdk/modules/finance/persistence/models"
	warehousemodels "github.com/iota-agency/iota-sdk/modules/warehouse/persistence/models"
	"github.com/iota-agency/iota-sdk/pkg/infrastructure/persistence/models"
)

var RegisteredModels = []interface{}{
	&models.Upload{},                    //nolint:exhaustruct
	&models.User{},                      //nolint:exhaustruct
	&models2.Payment{},                  //nolint:exhaustruct
	&models2.ExpenseCategory{},          //nolint:exhaustruct
	&models2.Expense{},                  //nolint:exhaustruct
	&warehousemodels.WarehouseUnit{},    //nolint:exhaustruct
	&warehousemodels.WarehouseOrder{},   //nolint:exhaustruct
	&models.Session{},                   //nolint:exhaustruct
	&models.Role{},                      //nolint:exhaustruct
	&models.Dialogue{},                  //nolint:exhaustruct
	&models.ActionLog{},                 //nolint:exhaustruct
	&models.Currency{},                  //nolint:exhaustruct
	&models2.Transaction{},              //nolint:exhaustruct
	&warehousemodels.WarehouseProduct{}, //nolint:exhaustruct
	&models2.MoneyAccount{},             //nolint:exhaustruct
}
