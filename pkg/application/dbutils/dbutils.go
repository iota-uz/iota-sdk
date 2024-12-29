package dbutils

import (
	"errors"
	"fmt"
	"github.com/iota-uz/iota-sdk/pkg/graphql/helpers"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func ConnectDB(dbOpts string, loggerInstance logger.Interface) (*gorm.DB, error) {
	db, err := gorm.Open(
		postgres.Open(dbOpts),
		&gorm.Config{
			Logger:                 loggerInstance,
			SkipDefaultTransaction: true,
		},
	)
	if err != nil {
		return nil, err
	}
	return db, nil
}

func CheckModels(db *gorm.DB, modelsToTest []interface{}) error {
	var errs []error
	for _, model := range modelsToTest {
		if err := helpers.CheckModelIsInSync(db, model); err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) == 0 {
		return nil
	}
	return fmt.Errorf("models are out of sync: %w", errors.Join(errs...))
}
