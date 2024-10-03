package persistence

import (
	"context"
	"github.com/iota-agency/iota-erp/internal/domain/entities/expense_category"
	"github.com/iota-agency/iota-erp/sdk/composables"
	"github.com/iota-agency/iota-erp/sdk/service"
)

type GormExpenseCategoryRepository struct {
}

func NewExpenseCategoryRepository() category.Repository {
	return &GormExpenseCategoryRepository{}
}

func (g *GormExpenseCategoryRepository) GetPaginated(ctx context.Context, limit, offset int, sortBy []string) ([]*category.ExpenseCategory, error) {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return nil, service.ErrNoTx
	}
	var categories []*category.ExpenseCategory
	q := tx.Limit(limit).Offset(offset)
	for _, s := range sortBy {
		q = q.Order(s)
	}
	if err := q.Find(&categories).Error; err != nil {
		return nil, err
	}
	return categories, nil
}

func (g *GormExpenseCategoryRepository) Count(ctx context.Context) (uint, error) {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return 0, service.ErrNoTx
	}
	var count int64
	if err := tx.Model(&category.ExpenseCategory{}).Count(&count).Error; err != nil {
		return 0, err
	}
	return uint(count), nil
}

func (g *GormExpenseCategoryRepository) GetAll(ctx context.Context) ([]*category.ExpenseCategory, error) {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return nil, service.ErrNoTx
	}
	var entities []*category.ExpenseCategory
	if err := tx.Find(&entities).Error; err != nil {
		return nil, err
	}
	return entities, nil
}

func (g *GormExpenseCategoryRepository) GetByID(ctx context.Context, id uint) (*category.ExpenseCategory, error) {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return nil, service.ErrNoTx
	}
	var entity category.ExpenseCategory
	if err := tx.First(&entity, id).Error; err != nil {
		return nil, err
	}
	return &entity, nil
}

func (g *GormExpenseCategoryRepository) Create(ctx context.Context, data *category.ExpenseCategory) error {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return service.ErrNoTx
	}
	if err := tx.Create(data).Error; err != nil {
		return err
	}
	return nil
}

func (g *GormExpenseCategoryRepository) Update(ctx context.Context, data *category.ExpenseCategory) error {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return service.ErrNoTx
	}
	if err := tx.Save(data).Error; err != nil {
		return err
	}
	return nil
}

func (g *GormExpenseCategoryRepository) Delete(ctx context.Context, id uint) error {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return service.ErrNoTx
	}
	if err := tx.Delete(&category.ExpenseCategory{}, id).Error; err != nil {
		return err
	}
	return nil
}
