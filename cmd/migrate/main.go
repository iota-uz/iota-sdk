package main

import (
	"github.com/iota-agency/iota-sdk/pkg/application/dbutils"
	"github.com/iota-agency/iota-sdk/pkg/configuration"
	"github.com/iota-agency/iota-sdk/pkg/server"
	"gorm.io/gorm/logger"
)

func main() {
	db, err := dbutils.ConnectDB(configuration.Use().DBOpts, logger.Warn)
	if err != nil {
		panic(err)
	}
	app := server.ConstructApp(db)
	if err := app.RunMigrations(); err != nil {
		panic(err)
	}
}
