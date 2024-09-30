package position

import (
	"context"
)

type Repository interface {
	Count(ctx context.Context) (int64, error)
	GetAll(ctx context.Context) ([]*Position, error)
	GetPaginated(ctx context.Context, limit, offset int, sortBy []string) ([]*Position, error)
	GetByID(ctx context.Context, id int64) (*Position, error)
	Create(ctx context.Context, upload *Position) error
	Update(ctx context.Context, upload *Position) error
	Delete(ctx context.Context, id int64) error
}
