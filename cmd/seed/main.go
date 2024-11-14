package main

import (
	"context"
	"github.com/iota-agency/iota-erp/internal/application"
	"github.com/iota-agency/iota-erp/internal/configuration"
	"github.com/iota-agency/iota-erp/internal/modules"
	"github.com/iota-agency/iota-erp/pkg/composables"
	"github.com/iota-agency/iota-erp/pkg/event"
	"github.com/iota-agency/iota-erp/seed"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type SeedFunc func(ctx context.Context, app *application.Application) error

func main() {
	conf := configuration.Use()
	db, err := gorm.Open(postgres.Open(conf.DBOpts), &gorm.Config{}) //nolint:exhaustruct
	if err != nil {
		panic(err)
	}

	seedFuncs := []SeedFunc{
		seed.CreatePermissions,
		seed.CreateCurrencies,
	}
	for _, module := range modules.LoadedModules {
		seedFuncs = append(seedFuncs, module.Seed)
	}

	app := application.New(db, event.NewEventPublisher())
	if err := db.Transaction(func(tx *gorm.DB) error {
		ctx := composables.WithTx(context.Background(), tx)
		for _, seedFunc := range seedFuncs {
			if err := seedFunc(ctx, app); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		panic(err)
	}
}
