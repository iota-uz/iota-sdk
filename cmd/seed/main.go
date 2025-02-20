package main

import (
	"context"
	"os"
	"runtime/debug"
	"time"

	"github.com/iota-uz/iota-sdk/modules"
	"github.com/iota-uz/iota-sdk/modules/bichat"
	"github.com/iota-uz/iota-sdk/modules/core"
	"github.com/iota-uz/iota-sdk/modules/crm"
	"github.com/iota-uz/iota-sdk/modules/finance"
	"github.com/iota-uz/iota-sdk/modules/warehouse"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/configuration"
	"github.com/iota-uz/iota-sdk/pkg/eventbus"

	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	defer func() {
		if r := recover(); r != nil {
			configuration.Use().Unload()
			debug.PrintStack()
			os.Exit(1)
		}
	}()

	conf := configuration.Use()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()
	pool, err := pgxpool.New(ctx, conf.Database.Opts)
	if err != nil {
		panic(err)
	}
	app := application.New(pool, eventbus.NewEventPublisher(conf.Logger()))
	if err := modules.Load(app, modules.BuiltInModules...); err != nil {
		panic(err)
	}
	app.RegisterNavItems(core.NavItems...)
	app.RegisterNavItems(bichat.NavItems...)
	app.RegisterNavItems(finance.NavItems...)
	app.RegisterNavItems(warehouse.NavItems...)
	app.RegisterNavItems(crm.NavItems...)
	tx, err := pool.Begin(ctx)
	if err != nil {
		panic(err)
	}
	if err := app.Seed(composables.WithTx(context.Background(), tx)); err != nil {
		panic(err)
	}
	if err := tx.Commit(ctx); err != nil {
		panic(err)
	}
}
