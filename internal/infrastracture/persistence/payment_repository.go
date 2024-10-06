package persistence

import (
	"context"
	"github.com/iota-agency/iota-erp/internal/infrastracture/persistence/models"

	"github.com/iota-agency/iota-erp/internal/domain/entities/payment"
	"github.com/iota-agency/iota-erp/sdk/composables"
	"github.com/iota-agency/iota-erp/sdk/service"
)

type GormPaymentRepository struct{}

func NewPaymentRepository() payment.Repository {
	return &GormPaymentRepository{}
}

func (g *GormPaymentRepository) GetPaginated(ctx context.Context, limit, offset int, sortBy []string) ([]*payment.Payment, error) {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return nil, service.ErrNoTx
	}
	var rows []*models.Payment
	q := tx.Limit(limit).Offset(offset)
	for _, s := range sortBy {
		q = q.Order(s)
	}
	if err := q.Preload("Transaction").Find(&rows).Error; err != nil {
		return nil, err
	}
	var entities []*payment.Payment
	for _, r := range rows {
		p, err := toDomainPayment(r)
		if err != nil {
			return nil, err
		}
		entities = append(entities, p)
	}
	return entities, nil
}

func (g *GormPaymentRepository) Count(ctx context.Context) (uint, error) {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return 0, service.ErrNoTx
	}
	var count int64
	if err := tx.Model(&models.Payment{}).Count(&count).Error; err != nil {
		return 0, err
	}
	return uint(count), nil
}

func (g *GormPaymentRepository) GetAll(ctx context.Context) ([]*payment.Payment, error) {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return nil, service.ErrNoTx
	}
	var rows []*models.Payment
	if err := tx.Preload("Transaction").Find(&rows).Error; err != nil {
		return nil, err
	}
	var entities []*payment.Payment
	for _, r := range rows {
		p, err := toDomainPayment(r)
		if err != nil {
			return nil, err
		}
		entities = append(entities, p)
	}
	return entities, nil
}

func (g *GormPaymentRepository) GetByID(ctx context.Context, id uint) (*payment.Payment, error) {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return nil, service.ErrNoTx
	}
	var row models.Payment
	if err := tx.Preload("Transaction").First(&row, id).Error; err != nil {
		return nil, err
	}
	return toDomainPayment(&row)
}

func (g *GormPaymentRepository) Create(ctx context.Context, data *payment.Payment) error {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return service.ErrNoTx
	}
	paymentRow, transactionRow := toDBPayment(data)
	if err := tx.Create(transactionRow).Error; err != nil {
		return err
	}
	paymentRow.TransactionID = transactionRow.ID
	return tx.Create(paymentRow).Error
}

func (g *GormPaymentRepository) Update(ctx context.Context, data *payment.Payment) error {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return service.ErrNoTx
	}
	entity, transaction := toDBPayment(data)
	if err := tx.Save(entity).Error; err != nil {
		return err
	}
	return tx.Save(transaction).Error
}

func (g *GormPaymentRepository) Delete(ctx context.Context, id uint) error {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return service.ErrNoTx
	}
	return tx.Delete(&models.Payment{}, id).Error
}
