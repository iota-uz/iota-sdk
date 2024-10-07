package server

import (
	"errors"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/iota-agency/iota-erp/internal/infrastructure/persistence/models"

	"github.com/iota-agency/iota-erp/sdk/graphql/helpers"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func newLogger(level logger.LogLevel) logger.Interface {
	return logger.New(
		log.New(os.Stdout, "\r\n", log.LstdFlags), // io writer
		logger.Config{
			SlowThreshold:             time.Second,
			LogLevel:                  level,
			IgnoreRecordNotFoundError: true,
			ParameterizedQueries:      true,
			Colorful:                  true,
		},
	)
}

func ConnectDB(dbOpts string, level logger.LogLevel) (*gorm.DB, error) {
	db, err := gorm.Open(postgres.Open(dbOpts), &gorm.Config{ //nolint:exhaustruct
		Logger:                 newLogger(level),
		SkipDefaultTransaction: true,
	})
	if err != nil {
		return nil, err
	}
	return db, nil
}

func CheckModels(db *gorm.DB) error {
	registeredModels := []interface{}{
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
	var errs []error
	for _, model := range registeredModels {
		if err := helpers.CheckModelIsInSync(db, model); err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) == 0 {
		return nil
	}
	return fmt.Errorf("models are out of sync: %w", errors.Join(errs...))
}
