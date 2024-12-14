package persistence

import (
	"context"
	"fmt"

	"github.com/iota-agency/iota-sdk/modules/finance/domain/aggregates/payment"
	"github.com/iota-agency/iota-sdk/modules/finance/persistence/models"
	"github.com/iota-agency/iota-sdk/pkg/composables"
	"github.com/iota-agency/iota-sdk/pkg/mapping"
)

type GormPaymentRepository struct{}

func NewPaymentRepository() payment.Repository {
	return &GormPaymentRepository{}
}

func (g *GormPaymentRepository) GetPaginated(
	ctx context.Context, params *payment.FindParams,
) ([]*payment.Payment, error) {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return nil, composables.ErrNoTx
	}
	var rows []*models.Payment
	tx = tx.Limit(params.Limit).Offset(params.Offset)
	if params.CreatedAt.To != "" && params.CreatedAt.From != "" {
		tx = tx.Where("created_at BETWEEN ? and ?", params.CreatedAt.From, params.CreatedAt.To)
	}
	for _, s := range params.SortBy {
		tx = tx.Order(s)
	}
	transactionArgs := []interface{}{}
	if params.Query != "" && params.Field != "" {
		transactionArgs = append(transactionArgs, fmt.Sprintf("%s::varchar ILIKE ?", params.Field), "%"+params.Query+"%")
	}
	if err := tx.Preload("Transaction", transactionArgs...).Find(&rows).Error; err != nil {
		return nil, err
	}
	return mapping.MapDbModels(rows, toDomainPayment)
}

func (g *GormPaymentRepository) Count(ctx context.Context) (uint, error) {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return 0, composables.ErrNoTx
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
		return nil, composables.ErrNoTx
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
		return nil, composables.ErrNoTx
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
		return composables.ErrNoTx
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
		return composables.ErrNoTx
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
		return composables.ErrNoTx
	}
	return tx.Delete(&models.Payment{}, id).Error //nolint:exhaustruct
}
