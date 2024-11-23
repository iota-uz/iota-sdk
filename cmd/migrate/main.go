package main

import (
	"github.com/iota-agency/iota-sdk/modules"
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
	loadedModules := modules.Load()
	for _, module := range loadedModules {
		if err := module.Register(app); err != nil {
			panic(err)
		}
	}
	if err := app.RunMigrations(); err != nil {
		panic(err)
	}
}
