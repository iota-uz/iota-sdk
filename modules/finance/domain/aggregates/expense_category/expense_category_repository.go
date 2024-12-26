package category

import (
	"context"
)

type DateRange struct {
	From string
	To   string
}

type FindParams struct {
	ID        uint
	Limit     int
	Offset    int
	SortBy    []string
	Query     string
	Field     string
	CreatedAt DateRange
}

type Repository interface {
	Count(ctx context.Context) (uint, error)
	GetAll(ctx context.Context) ([]*ExpenseCategory, error)
	GetPaginated(ctx context.Context, params *FindParams) ([]*ExpenseCategory, error)
	GetByID(ctx context.Context, id uint) (*ExpenseCategory, error)
	Create(ctx context.Context, user *ExpenseCategory) error
	Update(ctx context.Context, user *ExpenseCategory) error
	Delete(ctx context.Context, id uint) error
}
