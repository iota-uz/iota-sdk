package persistence

import (
	"context"
	"github.com/iota-agency/iota-sdk/modules/finance/domain/aggregates/payment"
	"github.com/iota-agency/iota-sdk/modules/finance/persistence/models"
	"github.com/iota-agency/iota-sdk/pkg/composables"
	"github.com/iota-agency/iota-sdk/pkg/mapping"
	"github.com/iota-agency/iota-sdk/pkg/service"
)

type GormPaymentRepository struct{}

func NewPaymentRepository() payment.Repository {
	return &GormPaymentRepository{}
}

func (g *GormPaymentRepository) GetPaginated(
	ctx context.Context, limit, offset int,
	sortBy []string,
) ([]*payment.Payment, error) {
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
	return mapping.MapDbModels(rows, toDomainPayment)
}

func (g *GormPaymentRepository) Count(ctx context.Context) (uint, error) {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return 0, service.ErrNoTx
	}
	var count int64
	if err := tx.Model(&models.Payment{}).Count(&count).Error; err != nil { //nolint:exhaustruct
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
	return mapping.MapDbModels(rows, toDomainPayment)
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
	// TODO: a nicer solution
	var transactionID uint
	model := &models.Payment{} // nolint:exhaustruct
	if err := tx.Model(model).Select("transaction_id").First(&transactionID, data.ID).Error; err != nil {
		return err
	}
	dbPayment, dbTransaction := toDBPayment(data)
	dbTransaction.ID = transactionID
	if err := tx.Updates(dbPayment).Error; err != nil {
		return err
	}
	return tx.Updates(dbTransaction).Error
}

func (g *GormPaymentRepository) Delete(ctx context.Context, id uint) error {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return service.ErrNoTx
	}
	return tx.Delete(&models.Payment{}, id).Error //nolint:exhaustruct
}
