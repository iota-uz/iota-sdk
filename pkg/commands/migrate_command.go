package commands

import (
	"errors"
	"fmt"
	"github.com/iota-agency/iota-sdk/modules"
	"github.com/iota-agency/iota-sdk/pkg/application/dbutils"
	"github.com/iota-agency/iota-sdk/pkg/configuration"
	"github.com/iota-agency/iota-sdk/pkg/server"
	"gorm.io/gorm/logger"
	"os"
)

var (
	ErrNoCommand = errors.New("expected 'up' or 'down' subcommands")
)

func Migrate() error {
	if len(os.Args) < 2 {
		return ErrNoCommand
	}
	migration := os.Args[1]
	db, err := dbutils.ConnectDB(configuration.Use().DBOpts, logger.Warn)
	if err != nil {
		panic(err)
	}
	app := server.ConstructApp(db)
	if err := modules.Load(app, modules.BuiltInModules...); err != nil {
		return err
	}
	switch migration {
	case "up":
		if err := app.RunMigrations(); err != nil {
			return err
		}
	case "down":
		if err := app.RollbackMigrations(); err != nil {
			return err
		}
	case "redo":
		if err := app.RollbackMigrations(); err != nil {
			return errors.Join(err, errors.New("failed to rollback migrations"))
		}
		if err := app.RunMigrations(); err != nil {
			return errors.Join(err, errors.New("failed to run migrations"))
		}
	default:
		return fmt.Errorf("unsupported command: %s\nSupported commands: 'up', 'down', 'redo'", os.Args[1])
	}
	return nil
}
