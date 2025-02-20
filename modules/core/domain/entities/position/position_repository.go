package position

import "context"

type Field int

const (
	Name Field = iota
	Descripton
	CreatedAt
)

type SortBy struct {
	Fields    []Field
	Ascending bool
}

type FindParams struct {
	ID     int64
	Limit  int
	Offset int
	SortBy SortBy
}

type Repository interface {
	Count(ctx context.Context) (int64, error)
	GetAll(ctx context.Context) ([]*Position, error)
	GetPaginated(ctx context.Context, params *FindParams) ([]*Position, error)
	GetByID(ctx context.Context, id int64) (*Position, error)
	Create(ctx context.Context, upload *Position) error
	Update(ctx context.Context, upload *Position) error
	Delete(ctx context.Context, id int64) error
}
