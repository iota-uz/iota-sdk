package expense

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
	Count(ctx context.Context) (uint, error)
	GetByID(ctx context.Context, id uint) (*Expense, error)
	GetAll(ctx context.Context) ([]*Expense, error)
	GetPaginated(ctx context.Context, params *FindParams) ([]*Expense, error)
	Create(ctx context.Context, data *Expense) error
	Update(ctx context.Context, data *Expense) error
	Delete(ctx context.Context, id uint) error
}
