package server

import (
	"errors"
	"github.com/iota-agency/iota-erp/internal/infrastracture/persistence/models"
	"github.com/iota-agency/iota-erp/sdk/graphql/helpers"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"log"
	"os"
	"time"
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
	db, err := gorm.Open(postgres.Open(dbOpts), &gorm.Config{
		Logger:                 newLogger(level),
		SkipDefaultTransaction: true,
	})
	if err != nil {
		return nil, err
	}
	return db, nil
}

func CheckModels(db *gorm.DB) {
	models := []interface{}{
		&models.Upload{},
		&models.User{},
		&models.Payment{},
		&models.ExpenseCategory{},
		&models.Expense{},
		&models.WarehouseUnit{},
		&models.WarehouseOrder{},
		&models.Session{},
		&models.Role{},
		&models.Dialogue{},
		&models.ActionLog{},
	}
	var errs []error
	for _, model := range models {
		if err := helpers.CheckModelIsInSync(db, model); err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		log.Fatalf("models are out of sync: %v", errors.Join(errs...))
	}
}
