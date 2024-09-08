package unit

import "context"

type Repository interface {
	Count(ctx context.Context) (int64, error)
	GetAll(ctx context.Context) ([]*Unit, error)
	GetPaginated(ctx context.Context, limit, offset int, sortBy []string) ([]*Unit, error)
	GetByID(ctx context.Context, id int64) (*Unit, error)
	Create(ctx context.Context, upload *Unit) error
	Update(ctx context.Context, upload *Unit) error
	Delete(ctx context.Context, id int64) error
}
