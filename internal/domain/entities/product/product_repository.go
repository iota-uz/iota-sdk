package product

import "context"

type Repository interface {
	GetPaginated(ctx context.Context, limit, offset int, sortBy []string) ([]*Product, error)
	Count(ctx context.Context) (int64, error)
	GetAll(ctx context.Context) ([]*Product, error)
	GetByID(ctx context.Context, id int64) (*Product, error)
	Create(ctx context.Context, data *Product) error
	Update(ctx context.Context, data *Product) error
	Delete(ctx context.Context, id int64) error
}
