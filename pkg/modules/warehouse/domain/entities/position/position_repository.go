package position

import "context"

type FindParams struct {
	Limit  int
	Offset int
	SortBy []string
	Search string
}

type Repository interface {
	Count(ctx context.Context) (int64, error)
	GetAll(ctx context.Context) ([]*Position, error)
	GetPaginated(ctx context.Context, params *FindParams) ([]*Position, error)
	GetByID(ctx context.Context, id uint) (*Position, error)
	Create(ctx context.Context, data *Position) error
	CreateOrUpdate(ctx context.Context, data *Position) error
	Update(ctx context.Context, data *Position) error
	Delete(ctx context.Context, id uint) error
}
