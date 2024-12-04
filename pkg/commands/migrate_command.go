package commands

import (
	"errors"
	"fmt"
	"github.com/iota-agency/iota-sdk/modules"
	"github.com/iota-agency/iota-sdk/pkg/application"
	"github.com/iota-agency/iota-sdk/pkg/application/dbutils"
	"github.com/iota-agency/iota-sdk/pkg/configuration"
	"github.com/iota-agency/iota-sdk/pkg/server"
	"gorm.io/gorm/logger"
	"log"
	"os"
)

var (
	ErrNoCommand = errors.New("expected 'up' or 'down' subcommands")
)

func HandleCommand(cmd string, app application.Application) error {
	var err error
	switch cmd {
	case "up":
		err = app.RunMigrations()
		if errors.Is(err, application.ErrNoMigrationsFound) {
			log.Println("No migrations found to run")
			return nil
		}
	case "down":
		err = app.RollbackMigrations()
		if errors.Is(err, application.ErrNoMigrationsFound) {
			log.Println("No migrations found to rollback")
			return nil
		}
	case "redo":
		err = app.RollbackMigrations()
		if errors.Is(err, application.ErrNoMigrationsFound) {
			log.Println("No migrations found to rollback")
			return nil
		}
		if err != nil {
			return errors.Join(err, errors.New("failed to rollback migrations"))
		}
		err = app.RunMigrations()
		if errors.Is(err, application.ErrNoMigrationsFound) {
			log.Println("No migrations found to run")
		}
	default:
		return fmt.Errorf("unsupported command: %s\nSupported commands: 'up', 'down', 'redo'", os.Args[1])
	}
	return err
}

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
	return HandleCommand(migration, app)
}
