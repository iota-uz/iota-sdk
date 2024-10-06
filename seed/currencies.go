package seed

import (
	"context"

	"github.com/iota-agency/iota-erp/internal/domain/entities/currency"
	"github.com/iota-agency/iota-erp/internal/infrastructure/persistence"
)

func CreateCurrencies(ctx context.Context) error {
	currencyRepository := persistence.NewCurrencyRepository()
	for _, c := range currency.Currencies {
		if err := currencyRepository.CreateOrUpdate(ctx, &c); err != nil {
			return err
		}
	}
	return nil
}
