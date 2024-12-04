package order

import "context"

type FindParams struct {
	Limit  int
	Offset int
	SortBy []string
}

type Repository interface {
	GetPaginated(ctx context.Context, params *FindParams) ([]*Order, error)
	Count(ctx context.Context) (int64, error)
	GetAll(ctx context.Context) ([]*Order, error)
	GetByID(ctx context.Context, id uint) (*Order, error)
	Create(ctx context.Context, data *Order) error
	Update(ctx context.Context, data *Order) error
	Delete(ctx context.Context, id uint) error
}
