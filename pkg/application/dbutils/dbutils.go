package dbutils

import (
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
