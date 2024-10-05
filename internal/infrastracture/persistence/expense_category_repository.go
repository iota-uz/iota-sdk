package persistence

import (
	"context"
	"github.com/iota-agency/iota-erp/internal/domain/entities/expense_category"
	"github.com/iota-agency/iota-erp/internal/infrastracture/persistence/models"
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
	q := tx.Limit(limit).Offset(offset)
	for _, s := range sortBy {
		q = q.Order(s)
	}
	var rows []*models.ExpenseCategory
	if err := q.Preload("AmountCurrency").Find(&rows).Error; err != nil {
		return nil, err
	}
	var categories []*category.ExpenseCategory
	for _, row := range rows {
		e, err := toDomainExpenseCategory(row)
		if err != nil {
			return nil, err
		}
		categories = append(categories, e)
	}
	return categories, nil
}

func (g *GormExpenseCategoryRepository) Count(ctx context.Context) (uint, error) {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return 0, service.ErrNoTx
	}
	var count int64
	if err := tx.Model(&models.ExpenseCategory{}).Count(&count).Error; err != nil {
		return 0, err
	}
	return uint(count), nil
}

func (g *GormExpenseCategoryRepository) GetAll(ctx context.Context) ([]*category.ExpenseCategory, error) {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return nil, service.ErrNoTx
	}
	var rows []*models.ExpenseCategory
	if err := tx.Preload("AmountCurrency").Find(&rows).Error; err != nil {
		return nil, err
	}
	var entities []*category.ExpenseCategory
	for _, row := range rows {
		e, err := toDomainExpenseCategory(row)
		if err != nil {
			return nil, err
		}
		entities = append(entities, e)
	}
	return entities, nil
}

func (g *GormExpenseCategoryRepository) GetByID(ctx context.Context, id uint) (*category.ExpenseCategory, error) {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return nil, service.ErrNoTx
	}
	var entity models.ExpenseCategory
	if err := tx.Preload("AmountCurrency").First(&entity, id).Error; err != nil {
		return nil, err
	}
	return toDomainExpenseCategory(&entity)
}

func (g *GormExpenseCategoryRepository) Create(ctx context.Context, data *category.ExpenseCategory) error {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return service.ErrNoTx
	}
	entity := toDbExpenseCategory(data)
	if err := tx.Create(entity).Error; err != nil {
		return err
	}
	return nil
}

func (g *GormExpenseCategoryRepository) Update(ctx context.Context, data *category.ExpenseCategory) error {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return service.ErrNoTx
	}
	entity := toDbExpenseCategory(data)
	if err := tx.Save(entity).Error; err != nil {
		return err
	}
	return nil
}

func (g *GormExpenseCategoryRepository) Delete(ctx context.Context, id uint) error {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return service.ErrNoTx
	}
	if err := tx.Delete(&models.ExpenseCategory{}, id).Error; err != nil {
		return err
	}
	return nil
}
