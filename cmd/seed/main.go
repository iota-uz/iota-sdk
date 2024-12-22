package main

import (
	"context"
	"github.com/iota-agency/iota-sdk/modules"
	"github.com/iota-agency/iota-sdk/pkg/application"
	"github.com/iota-agency/iota-sdk/pkg/composables"
	"github.com/iota-agency/iota-sdk/pkg/configuration"
	"github.com/iota-agency/iota-sdk/pkg/event"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func main() {
	conf := configuration.Use()
	db, err := gorm.Open(postgres.Open(conf.DBOpts), &gorm.Config{}) //nolint:exhaustruct
	if err != nil {
		panic(err)
	}

	app := application.New(db, event.NewEventPublisher())
	if err := modules.Load(app, modules.BuiltInModules...); err != nil {
		panic(err)
	}
	if err := db.Transaction(func(tx *gorm.DB) error {
		ctx := composables.WithTx(context.Background(), tx)
		return app.Seed(ctx)
	}); err != nil {
		panic(err)
	}
}
