// Package seed provides this package.
package seed

import (
	"context"

	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/currency"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/sirupsen/logrus"
)

var createCurrencies = application.Seed(
	func(ctx context.Context, currencyRepository currency.Repository, logger logrus.FieldLogger) error {
		logger.Info("Seeding currencies")
		for _, c := range currency.Currencies {
			logger.Infof("Creating or updating currency: %s (%s)", c.Name(), c.Code())
			if err := currencyRepository.CreateOrUpdate(ctx, c); err != nil {
				logger.Errorf("Failed to create currency %s: %v", c.Code(), err)
				return err
			}
		}
		logger.Info("Finished seeding currencies")
		return nil
	},
)

func CreateCurrencies(ctx context.Context, deps *application.SeedDeps) error {
	return createCurrencies(ctx, deps)
}
