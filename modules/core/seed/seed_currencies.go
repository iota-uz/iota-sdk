package seed

import (
	"context"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/currency"
	"github.com/iota-uz/iota-sdk/modules/core/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/configuration"
)

func CreateCurrencies(ctx context.Context, app application.Application) error {
	conf := configuration.Use()
	currencyRepository := persistence.NewCurrencyRepository()

	conf.Logger().Info("Seeding currencies")
	for _, c := range currency.Currencies {
		conf.Logger().Infof("Creating or updating currency: %s (%s)", c.Name, c.Code)
		if err := currencyRepository.CreateOrUpdate(ctx, &c); err != nil {
			conf.Logger().Errorf("Failed to create currency %s: %v", c.Code, err)
			return err
		}
	}
	conf.Logger().Info("Finished seeding currencies")
	return nil
}
