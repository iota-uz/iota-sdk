package commands

import (
	"context"
	"errors"
	"fmt"
	"os"
	"runtime/debug"
	"time"

	"github.com/iota-uz/iota-sdk/modules"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/configuration"
	"github.com/iota-uz/iota-sdk/pkg/eventbus"

	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrNoCommand = errors.New("expected 'up', 'down' or 'redo' subcommands")
)

func Migrate(mods ...application.Module) error {
	defer func() {
		if r := recover(); r != nil {
			configuration.Use().Unload()
			debug.PrintStack()
			os.Exit(1)
		}
	}()

	if len(os.Args) < 2 {
		return ErrNoCommand
	}
	migration := os.Args[1]

	conf := configuration.Use()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()
	pool, err := pgxpool.New(ctx, conf.Database.Opts)
	if err != nil {
		panic(err)
	}
	app := application.New(pool, eventbus.NewEventPublisher(conf.Logger()))
	if err := modules.Load(app, mods...); err != nil {
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
