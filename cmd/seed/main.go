package main

import (
	"context"
	"github.com/iota-agency/iota-erp/internal/configuration"
	"github.com/iota-agency/iota-erp/sdk/composables"
	"github.com/iota-agency/iota-erp/seed"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type SeedFunc func(ctx context.Context) error

func main() {
	conf := configuration.Use()
	db, err := gorm.Open(postgres.Open(conf.DbOpts), &gorm.Config{})
	if err != nil {
		panic(err)
	}

	seedFuncs := []SeedFunc{
		seed.CreateInitialUser,
		seed.CreateCurrencies,
	}
	if err := db.Transaction(func(tx *gorm.DB) error {
		ctx := composables.WithTx(context.Background(), tx)
		for _, seedFunc := range seedFuncs {
			if err := seedFunc(ctx); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		panic(err)
	}
}
