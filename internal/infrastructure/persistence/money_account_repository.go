package persistence

import (
	"context"
	moneyAccount "github.com/iota-agency/iota-erp/internal/domain/aggregates/money_account"
	"github.com/iota-agency/iota-erp/internal/domain/entities/transaction"
	"github.com/iota-agency/iota-erp/internal/infrastructure/persistence/models"
	"github.com/iota-agency/iota-erp/sdk/composables"
	"github.com/iota-agency/iota-erp/sdk/service"
	"time"
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
	if err := q.Preload("Currency").Find(&rows).Error; err != nil {
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
	if err := tx.Model(&models.MoneyAccount{}).Count(&count).Error; err != nil { //nolint:exhaustruct
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
	if err := tx.Preload("Currency").Find(&rows).Error; err != nil {
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
	var entity models.MoneyAccount
	if err := tx.Preload("Currency").First(&entity, id).Error; err != nil {
		return nil, err
	}
	return toDomainMoneyAccount(&entity)
}

func (g *GormMoneyAccountRepository) RecalculateBalance(ctx context.Context, id uint) error {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return service.ErrNoTx
	}
	var balance float64
	q := tx.Model(&models.Transaction{}).Where("origin_account_id = ?", id).Or(
		"destination_account_id = ?", id,
	).Select("sum(amount)") //nolint:exhaustruct
	if err := q.Row().Scan(&balance); err != nil {
		return err
	}
	return tx.Model(&models.MoneyAccount{}).Where("id = ?", id).Update("balance", balance).Error //nolint:exhaustruct
}

func (g *GormMoneyAccountRepository) Create(ctx context.Context, data *moneyAccount.Account) error {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return service.ErrNoTx
	}
	entity := toDBMoneyAccount(data)
	if err := tx.Create(entity).Error; err != nil {
		return err
	}
	if err := tx.Create(
		&models.Transaction{
			ID:                   0,
			OriginAccountID:      nil,
			DestinationAccountID: &entity.ID,
			Amount:               data.Balance,
			Comment:              "Initial balance",
			CreatedAt:            data.CreatedAt,
			AccountingPeriod:     time.Now(),
			TransactionDate:      time.Now(),
			TransactionType:      string(transaction.IncomeType),
		},
	).Error; err != nil {
		return err
	}
	return nil
}

func (g *GormMoneyAccountRepository) Update(ctx context.Context, data *moneyAccount.Account) error {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return service.ErrNoTx
	}
	return tx.Updates(toDBMoneyAccount(data)).Error
}

func (g *GormMoneyAccountRepository) Delete(ctx context.Context, id uint) error {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return service.ErrNoTx
	}
	return tx.Delete(&models.MoneyAccount{}, id).Error //nolint:exhaustruct
}
