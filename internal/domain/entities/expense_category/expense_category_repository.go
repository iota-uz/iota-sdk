package category

import (
	"context"
)

type Repository interface {
	Count(ctx context.Context) (int64, error)
	GetAll(ctx context.Context) ([]*ExpenseCategory, error)
	GetPaginated(ctx context.Context, limit, offset int, sortBy []string) ([]*ExpenseCategory, error)
	GetByID(ctx context.Context, id int64) (*ExpenseCategory, error)
	Create(ctx context.Context, user *ExpenseCategory) error
	Update(ctx context.Context, user *ExpenseCategory) error
	Delete(ctx context.Context, id int64) error
}
