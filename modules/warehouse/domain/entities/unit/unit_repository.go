package unit

import "context"

type DateRange struct {
	From string
	To   string
}

type FindParams struct {
	Limit     int
	Offset    int
	SortBy    []string
	CreatedAt DateRange
}

type Repository interface {
	Count(ctx context.Context) (uint, error)
	GetAll(ctx context.Context) ([]*Unit, error)
	GetPaginated(ctx context.Context, params *FindParams) ([]*Unit, error)
	GetByID(ctx context.Context, id uint) (*Unit, error)
	GetByTitleOrShortTitle(ctx context.Context, name string) (*Unit, error)
	Create(ctx context.Context, upload *Unit) error
	CreateOrUpdate(ctx context.Context, upload *Unit) error
	Update(ctx context.Context, upload *Unit) error
	Delete(ctx context.Context, id uint) error
}
