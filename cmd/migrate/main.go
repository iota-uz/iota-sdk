package main

import (
	"fmt"
	"os"

	"github.com/iota-agency/iota-sdk/modules"
	"github.com/iota-agency/iota-sdk/pkg/application/dbutils"
	"github.com/iota-agency/iota-sdk/pkg/configuration"
	"github.com/iota-agency/iota-sdk/pkg/server"
	"gorm.io/gorm/logger"
)

func main() {
	if len(os.Args) < 2 {
		panic("expected 'up' or 'down' subcommands")
	}
	migration := os.Args[1]
	db, err := dbutils.ConnectDB(configuration.Use().DBOpts, logger.Warn)
	if err != nil {
		panic(err)
	}
	app := server.ConstructApp(db)
	if err := modules.Load(app, modules.BuiltInModules...); err != nil {
		panic(err)
	}
	switch migration {
	case "up":
		if err := app.RunMigrations(); err != nil {
			panic(err)
		}
	case "down":
		if err := app.RollbackMigrations(); err != nil {
			panic(err)
		}
	default:
		panic(fmt.Sprintf("unsupported command: %s\nSupported commands: 'up' or 'down'", os.Args[1]))
	}
}
