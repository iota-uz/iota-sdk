package inventory

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
	Status    string
	Type      string
	CreatedAt DateRange
}

type Repository interface {
	Count(ctx context.Context) (uint, error)
	GetAll(ctx context.Context) ([]*Check, error)
	GetPaginated(ctx context.Context, params *FindParams) ([]*Check, error)
	GetByID(ctx context.Context, id uint) (*Check, error)
	Create(ctx context.Context, upload *Check) error
	Update(ctx context.Context, upload *Check) error
	Delete(ctx context.Context, id uint) error
}
