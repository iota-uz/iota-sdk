package employee

import "context"

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
	Count(ctx context.Context) (int64, error)
	GetAll(ctx context.Context) ([]Employee, error)
	GetPaginated(ctx context.Context, params *FindParams) ([]Employee, error)
	GetByID(ctx context.Context, id uint) (Employee, error)
	Create(ctx context.Context, data Employee) (Employee, error)
	Update(ctx context.Context, data Employee) error
	Delete(ctx context.Context, id uint) error
}
