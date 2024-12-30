package persistence

import (
	"context"
	"errors"
	"fmt"
	"github.com/iota-uz/iota-sdk/pkg/utils/repo"
	"strings"

	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/currency"
	"github.com/iota-uz/iota-sdk/modules/core/infrastructure/persistence/models"

	"github.com/iota-uz/iota-sdk/pkg/composables"
)

var (
	ErrCurrencyNotFound = errors.New("currency not found")
)

type GormCurrencyRepository struct{}

func NewCurrencyRepository() currency.Repository {
	return &GormCurrencyRepository{}
}

func (g *GormCurrencyRepository) GetPaginated(
	ctx context.Context, params *currency.FindParams,
) ([]*currency.Currency, error) {
	pool, err := composables.UsePool(ctx)
	if err != nil {
		return nil, err
	}
	where, args := []string{"1 = 1"}, []interface{}{}
	if params.Code != "" {
		where, args = append(where, fmt.Sprintf("code = $%d", len(args)+1)), append(args, params.Code)
	}

	rows, err := pool.Query(ctx, `
		SELECT code, name, symbol, created_at, updated_at FROM currencies
		WHERE `+strings.Join(where, " AND ")+`
		`+repo.FormatLimitOffset(params.Limit, params.Offset)+`
	`, args...)

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	currencies := make([]*currency.Currency, 0)
	for rows.Next() {
		var currency models.Currency
		if err := rows.Scan(
			&currency.Code,
			&currency.Name,
			&currency.Symbol,
			&currency.CreatedAt,
			&currency.UpdatedAt,
		); err != nil {
			return nil, err
		}

		domainCurrency, err := ToDomainCurrency(&currency)
		if err != nil {
			return nil, err
		}
		currencies = append(currencies, domainCurrency)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return currencies, nil
}

func (g *GormCurrencyRepository) Count(ctx context.Context) (uint, error) {
	pool, err := composables.UsePool(ctx)
	if err != nil {
		return 0, err
	}
	var count uint
	if err := pool.QueryRow(ctx, `
		SELECT COUNT(*) as count FROM currencies
	`).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

func (g *GormCurrencyRepository) GetAll(ctx context.Context) ([]*currency.Currency, error) {
	return g.GetPaginated(ctx, &currency.FindParams{
		Limit: 100000,
	})
}

func (g *GormCurrencyRepository) GetByCode(ctx context.Context, code string) (*currency.Currency, error) {
	currencies, err := g.GetPaginated(ctx, &currency.FindParams{
		Code: code,
	})
	if err != nil {
		return nil, err
	}
	if len(currencies) == 0 {
		return nil, ErrCurrencyNotFound
	}
	return currencies[0], nil
}

func (g *GormCurrencyRepository) Create(ctx context.Context, entity *currency.Currency) error {
	tx, err := composables.UsePoolTx(ctx)
	if err != nil {
		return err
	}
	row := ToDBCurrency(entity)
	if _, err := tx.Exec(ctx, `
		INSERT INTO currencies (code, name, symbol) VALUES ($1, $2, $3)
	`, row.Code, row.Name, row.Symbol); err != nil {
		return err
	}
	return nil
}

func (g *GormCurrencyRepository) Update(ctx context.Context, entity *currency.Currency) error {
	tx, err := composables.UsePoolTx(ctx)
	if err != nil {
		return err
	}
	row := ToDBCurrency(entity)
	if _, err := tx.Exec(ctx, `
		UPDATE currencies 
		SET name = $1, symbol = $2
		WHERE code = $3
	`, row.Name, row.Symbol, row.Code); err != nil {
		return err
	}
	return nil
}

func (g *GormCurrencyRepository) CreateOrUpdate(ctx context.Context, currency *currency.Currency) error {
	u, err := g.GetByCode(ctx, string(currency.Code))
	if err != nil && !errors.Is(err, ErrCurrencyNotFound) {
		return err
	}
	if u != nil {
		if err := g.Update(ctx, currency); err != nil {
			return err
		}
	} else {
		if err := g.Create(ctx, currency); err != nil {
			return err
		}
	}
	return nil
}

func (g *GormCurrencyRepository) Delete(ctx context.Context, code string) error {
	tx, err := composables.UsePoolTx(ctx)
	if err != nil {
		return composables.ErrNoTx
	}
	if _, err := tx.Exec(ctx, `DELETE FROM currencies where code = $1`, code); err != nil {
		return err
	}
	return nil
}
