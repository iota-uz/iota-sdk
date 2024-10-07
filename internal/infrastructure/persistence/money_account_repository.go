package persistence

import (
	"context"

	moneyAccount "github.com/iota-agency/iota-erp/internal/domain/aggregates/money_account"
	"github.com/iota-agency/iota-erp/internal/infrastructure/persistence/models"
	"github.com/iota-agency/iota-erp/sdk/composables"
	"github.com/iota-agency/iota-erp/sdk/service"
)

type GormMoneyAccountRepository struct{}

func NewMoneyAccountRepository() moneyAccount.Repository {
	return &GormMoneyAccountRepository{}
}

func (g *GormMoneyAccountRepository) GetPaginated(
	ctx context.Context, limit, offset int,
	sortBy []string,
) ([]*moneyAccount.Account, error) {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return nil, service.ErrNoTx
	}
	var rows []*models.MoneyAccount
	q := tx.Limit(limit).Offset(offset)
	for _, s := range sortBy {
		q = q.Order(s)
	}
	if err := q.Find(&rows).Error; err != nil {
		return nil, err
	}
	entities := make([]*moneyAccount.Account, len(rows))
	for i, r := range rows {
		p, err := toDomainMoneyAccount(r)
		if err != nil {
			return nil, err
		}
		entities[i] = p
	}
	return entities, nil
}

func (g *GormMoneyAccountRepository) Count(ctx context.Context) (uint, error) {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return 0, service.ErrNoTx
	}
	var count int64
	if err := tx.Model(&moneyAccount.Account{}).Count(&count).Error; err != nil { //nolint:exhaustruct
		return 0, err
	}
	return uint(count), nil
}

func (g *GormMoneyAccountRepository) GetAll(ctx context.Context) ([]*moneyAccount.Account, error) {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return nil, service.ErrNoTx
	}
	var rows []*models.MoneyAccount
	if err := tx.Find(&rows).Error; err != nil {
		return nil, err
	}
	entities := make([]*moneyAccount.Account, len(rows))
	for i, r := range rows {
		p, err := toDomainMoneyAccount(r)
		if err != nil {
			return nil, err
		}
		entities[i] = p
	}
	return entities, nil
}

func (g *GormMoneyAccountRepository) GetByID(ctx context.Context, id uint) (*moneyAccount.Account, error) {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return nil, service.ErrNoTx
	}
	var entity moneyAccount.Account
	if err := tx.First(&entity, id).Error; err != nil {
		return nil, err
	}
	return &entity, nil
}

func (g *GormMoneyAccountRepository) Create(ctx context.Context, data *moneyAccount.Account) error {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return service.ErrNoTx
	}
	row := toDBMoneyAccount(data)
	return tx.Create(row).Error
}

func (g *GormMoneyAccountRepository) Update(ctx context.Context, data *moneyAccount.Account) error {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return service.ErrNoTx
	}
	row := toDBMoneyAccount(data)
	return tx.Save(row).Error
}

func (g *GormMoneyAccountRepository) Delete(ctx context.Context, id uint) error {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return service.ErrNoTx
	}
	return tx.Delete(&models.MoneyAccount{}, id).Error
}
