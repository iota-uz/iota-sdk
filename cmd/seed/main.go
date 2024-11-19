package main

import (
	"context"
	"github.com/iota-agency/iota-sdk/pkg/application"
	"github.com/iota-agency/iota-sdk/pkg/composables"
	"github.com/iota-agency/iota-sdk/pkg/configuration"
	"github.com/iota-agency/iota-sdk/pkg/event"
	"github.com/iota-agency/iota-sdk/pkg/modules"
	"github.com/iota-agency/iota-sdk/pkg/modules/shared"
	"github.com/iota-agency/iota-sdk/seed"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func main() {
	conf := configuration.Use()
	db, err := gorm.Open(postgres.Open(conf.DBOpts), &gorm.Config{}) //nolint:exhaustruct
	if err != nil {
		panic(err)
	}

	seedFuncs := []shared.SeedFunc{
		seed.CreatePermissions,
		seed.CreateCurrencies,
	}
	registry := modules.Load()
	for _, module := range registry.Modules() {
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
