package persistence

import (
	"context"
	transaction2 "github.com/iota-agency/iota-sdk/modules/finance/domain/entities/transaction"
	"github.com/iota-agency/iota-sdk/pkg/composables"
	"github.com/iota-agency/iota-sdk/pkg/graphql/helpers"
	"github.com/iota-agency/iota-sdk/pkg/service"

	"github.com/iota-agency/iota-sdk/pkg/infrastructure/persistence/models"
)

type GormTransactionRepository struct{}

func NewTransactionRepository() transaction2.Repository {
	return &GormTransactionRepository{}
}

func (g *GormTransactionRepository) GetPaginated(
	ctx context.Context,
	limit, offset int,
	sortBy []string,
) ([]*transaction2.Transaction, error) {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return nil, service.ErrNoTx
	}
	q := tx.Limit(limit).Offset(offset)
	q, err := helpers.ApplySort(q, sortBy)
	if err != nil {
		return nil, err
	}
	var entities []*transaction2.Transaction
	if err := q.Find(&entities).Error; err != nil {
		return nil, err
	}
	return entities, nil
}

func (g *GormTransactionRepository) Count(ctx context.Context) (int64, error) {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return 0, service.ErrNoTx
	}
	var count int64
	if err := tx.Model(&transaction2.Transaction{}).Count(&count).Error; err != nil { //nolint:exhaustruct
		return 0, err
	}
	return count, nil
}

func (g *GormTransactionRepository) GetAll(ctx context.Context) ([]*transaction2.Transaction, error) {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return nil, service.ErrNoTx
	}
	var entities []*transaction2.Transaction
	if err := tx.Find(&entities).Error; err != nil {
		return nil, err
	}
	return entities, nil
}

func (g *GormTransactionRepository) GetByID(ctx context.Context, id int64) (*transaction2.Transaction, error) {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return nil, service.ErrNoTx
	}
	var entity models.Transaction
	if err := tx.First(&entity, id).Error; err != nil {
		return nil, err
	}
	return toDomainTransaction(&entity)
}

func (g *GormTransactionRepository) Create(ctx context.Context, data *transaction2.Transaction) error {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return service.ErrNoTx
	}
	entity := toDBTransaction(data)
	return tx.Create(entity).Error
}

func (g *GormTransactionRepository) Update(ctx context.Context, data *transaction2.Transaction) error {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return service.ErrNoTx
	}
	entity := toDBTransaction(data)
	return tx.Save(entity).Error
}

func (g *GormTransactionRepository) Delete(ctx context.Context, id int64) error {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return service.ErrNoTx
	}
	return tx.Delete(&transaction2.Transaction{}, id).Error //nolint:exhaustruct
}
