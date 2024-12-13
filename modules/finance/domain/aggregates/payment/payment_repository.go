package payment

import (
	"context"
)

type DateRange struct {
	From string
	To   string
}

type FindParams struct {
	Limit     int
	Offset    int
	SortBy    []string
	Query     string
	Field     string
	CreatedAt DateRange
}

type Repository interface {
	Count(ctx context.Context) (uint, error)
	GetAll(ctx context.Context) ([]*Payment, error)
	GetPaginated(ctx context.Context, params *FindParams) ([]*Payment, error)
	GetByID(ctx context.Context, id uint) (*Payment, error)
	Create(ctx context.Context, payment *Payment) error
	Update(ctx context.Context, payment *Payment) error
	Delete(ctx context.Context, id uint) error
}
