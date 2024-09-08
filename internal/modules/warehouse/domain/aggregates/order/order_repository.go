package order

import "context"

type Repository interface {
	GetPaginated(ctx context.Context, limit, offset int, sortBy []string) ([]*Order, error)
	Count(ctx context.Context) (int64, error)
	GetAll(ctx context.Context) ([]*Order, error)
	GetByID(ctx context.Context, id int64) (*Order, error)
	Create(ctx context.Context, data *Order) error
	Update(ctx context.Context, data *Order) error
	Delete(ctx context.Context, id int64) error
}
