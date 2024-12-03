package main

import (
	"context"
	"github.com/iota-agency/iota-sdk/internal/seed"
	"github.com/iota-agency/iota-sdk/modules"
	"github.com/iota-agency/iota-sdk/pkg/application"
	"github.com/iota-agency/iota-sdk/pkg/composables"
	"github.com/iota-agency/iota-sdk/pkg/configuration"
	"github.com/iota-agency/iota-sdk/pkg/server"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func main() {
	conf := configuration.Use()
	db, err := gorm.Open(postgres.Open(conf.DBOpts), &gorm.Config{}) //nolint:exhaustruct
	if err != nil {
		panic(err)
	}

	app := server.ConstructApp(db)
	if err := modules.Load(app, modules.BuiltInModules...); err != nil {
		panic(err)
	}
	seedFuncs := []application.SeedFunc{
		seed.CreatePermissions,
		seed.CreateCurrencies,
		seed.CreateUser,
	}

	if err := db.Transaction(func(tx *gorm.DB) error {
		ctx := composables.WithTx(context.Background(), tx)
		for _, seedFunc := range seedFuncs {
			if err := seedFunc(ctx, app); err != nil {
				return err
			}
		}
		return app.Seed(ctx)
	}); err != nil {
		panic(err)
	}
}
