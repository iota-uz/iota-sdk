package server

import "github.com/iota-agency/iota-erp/internal/infrastructure/persistence/models"

var RegisteredModels = []interface{}{
	&models.Upload{},           //nolint:exhaustruct
	&models.User{},             //nolint:exhaustruct
	&models.Payment{},          //nolint:exhaustruct
	&models.ExpenseCategory{},  //nolint:exhaustruct
	&models.Expense{},          //nolint:exhaustruct
	&models.WarehouseUnit{},    //nolint:exhaustruct
	&models.WarehouseOrder{},   //nolint:exhaustruct
	&models.Session{},          //nolint:exhaustruct
	&models.Role{},             //nolint:exhaustruct
	&models.Dialogue{},         //nolint:exhaustruct
	&models.ActionLog{},        //nolint:exhaustruct
	&models.Currency{},         //nolint:exhaustruct
	&models.Transaction{},      //nolint:exhaustruct
	&models.WarehouseProduct{}, //nolint:exhaustruct
	&models.MoneyAccount{},     //nolint:exhaustruct
}
