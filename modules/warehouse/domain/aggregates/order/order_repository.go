package order

import "context"

type DateRange struct {
	From string
	To   string
}

type FindParams struct {
	ID        uint
	Limit     int
	Offset    int
	SortBy    []string
	Query     string
	Field     string
	Status    string
	Type      string
	CreatedAt DateRange
}

type Repository interface {
	GetPaginated(ctx context.Context, params *FindParams) ([]Order, error)
	Count(ctx context.Context) (int64, error)
	GetAll(ctx context.Context) ([]Order, error)
	GetByID(ctx context.Context, id uint) (Order, error)
	Create(ctx context.Context, data Order) error
	Update(ctx context.Context, data Order) error
	Delete(ctx context.Context, id uint) error
}
