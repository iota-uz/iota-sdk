package server

import (
	"github.com/iota-agency/iota-sdk/modules/core/infrastructure/persistence/models"
	financemodels "github.com/iota-agency/iota-sdk/modules/finance/persistence/models"
	warehousemodels "github.com/iota-agency/iota-sdk/modules/warehouse/persistence/models"
)

var RegisteredModels = []interface{}{
	&models.Upload{},                    //nolint:exhaustruct
	&models.User{},                      //nolint:exhaustruct
	&financemodels.Payment{},            //nolint:exhaustruct
	&financemodels.ExpenseCategory{},    //nolint:exhaustruct
	&financemodels.Expense{},            //nolint:exhaustruct
	&warehousemodels.WarehouseUnit{},    //nolint:exhaustruct
	&warehousemodels.WarehouseOrder{},   //nolint:exhaustruct
	&models.Session{},                   //nolint:exhaustruct
	&models.Role{},                      //nolint:exhaustruct
	&models.Dialogue{},                  //nolint:exhaustruct
	&models.ActionLog{},                 //nolint:exhaustruct
	&models.Currency{},                  //nolint:exhaustruct
	&financemodels.Transaction{},        //nolint:exhaustruct
	&warehousemodels.WarehouseProduct{}, //nolint:exhaustruct
	&financemodels.MoneyAccount{},       //nolint:exhaustruct
}
