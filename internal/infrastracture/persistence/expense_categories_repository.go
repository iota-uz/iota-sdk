package persistence

import (
	"context"
	"github.com/iota-agency/iota-erp/internal/domain/expense_category"
	"github.com/iota-agency/iota-erp/sdk/composables"
	"github.com/iota-agency/iota-erp/sdk/service"
)

type GormExpenseCategoriesRepository struct {
}

func NewExpenseCategoriesRepository() category.Repository {
	return &GormExpenseCategoriesRepository{}
}

func (g *GormExpenseCategoriesRepository) GetPaginated(ctx context.Context, limit, offset int, sortBy []string) ([]*category.ExpenseCategory, error) {
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

func (g *GormExpenseCategoriesRepository) Count(ctx context.Context) (int64, error) {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return 0, service.ErrNoTx
	}
	var count int64
	if err := tx.Model(&category.ExpenseCategory{}).Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

func (g *GormExpenseCategoriesRepository) GetAll(ctx context.Context) ([]*category.ExpenseCategory, error) {
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

func (g *GormExpenseCategoriesRepository) GetByID(ctx context.Context, id int64) (*category.ExpenseCategory, error) {
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

func (g *GormExpenseCategoriesRepository) Create(ctx context.Context, data *category.ExpenseCategory) error {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return service.ErrNoTx
	}
	if err := tx.Create(data).Error; err != nil {
		return err
	}
	return nil
}

func (g *GormExpenseCategoriesRepository) Update(ctx context.Context, data *category.ExpenseCategory) error {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return service.ErrNoTx
	}
	if err := tx.Save(data).Error; err != nil {
		return err
	}
	return nil
}

func (g *GormExpenseCategoriesRepository) Delete(ctx context.Context, id int64) error {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return service.ErrNoTx
	}
	if err := tx.Delete(&category.ExpenseCategory{}, id).Error; err != nil {
		return err
	}
	return nil
}
