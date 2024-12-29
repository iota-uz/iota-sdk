package seed

import (
	"context"
	"github.com/iota-uz/iota-sdk/modules/core/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/domain/entities/currency"
)

func CreateCurrencies(ctx context.Context, app application.Application) error {
	currencyRepository := persistence.NewCurrencyRepository()
	for _, c := range currency.Currencies {
		if err := currencyRepository.CreateOrUpdate(ctx, &c); err != nil {
			return err
		}
	}
	return nil
}
