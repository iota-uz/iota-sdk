package persistence

import (
	"context"
	"github.com/iota-agency/iota-erp/internal/domain/entities/payment"
	"github.com/iota-agency/iota-erp/internal/infrastracture/persistence/models"
	"github.com/iota-agency/iota-erp/sdk/composables"
	"github.com/iota-agency/iota-erp/sdk/service"
)

type GormPaymentRepository struct {
}

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
	if err := q.Find(&rows).Error; err != nil {
		return nil, err
	}
	var ids []uint
	for _, r := range rows {
		ids = append(ids, r.ID)
	}
	var transactionRows []*models.Transaction
	if err := tx.Where("id IN ?", ids).Find(&transactionRows).Error; err != nil {
		return nil, err
	}
	transactionMap := make(map[uint]*models.Transaction, len(transactionRows))
	for _, tr := range transactionRows {
		transactionMap[tr.ID] = tr
	}
	var entities []*payment.Payment
	for _, r := range rows {
		if tr, ok := transactionMap[*r.TransactionID]; ok {
			p, err := toDomainPayment(r, tr)
			if err != nil {
				return nil, err
			}
			entities = append(entities, p)
		}
	}
	return entities, nil
}

func (g *GormPaymentRepository) Count(ctx context.Context) (uint, error) {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return 0, service.ErrNoTx
	}
	var count int64
	if err := tx.Model(&payment.Payment{}).Count(&count).Error; err != nil {
		return 0, err
	}
	return uint(count), nil
}

func (g *GormPaymentRepository) GetAll(ctx context.Context) ([]*payment.Payment, error) {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return nil, service.ErrNoTx
	}
	var entities []*payment.Payment
	if err := tx.Find(&entities).Error; err != nil {
		return nil, err
	}
	return entities, nil
}

func (g *GormPaymentRepository) GetByID(ctx context.Context, id uint) (*payment.Payment, error) {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return nil, service.ErrNoTx
	}
	var entity payment.Payment
	if err := tx.First(&entity, id).Error; err != nil {
		return nil, err
	}
	return &entity, nil
}

func (g *GormPaymentRepository) Create(ctx context.Context, data *payment.Payment) error {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return service.ErrNoTx
	}
	if err := tx.Create(data).Error; err != nil {
		return err
	}
	return nil
}

func (g *GormPaymentRepository) Update(ctx context.Context, data *payment.Payment) error {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return service.ErrNoTx
	}
	if err := tx.Save(data).Error; err != nil {
		return err
	}
	return nil
}

func (g *GormPaymentRepository) Delete(ctx context.Context, id uint) error {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return service.ErrNoTx
	}
	if err := tx.Delete(&payment.Payment{}, id).Error; err != nil {
		return err
	}
	return nil
}
