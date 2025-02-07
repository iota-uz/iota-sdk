package messagetemplate

import "context"

type FindParams struct {
	Limit  int
	Offset int
	SortBy []string
}

type Repository interface {
	Count(ctx context.Context) (int64, error)
	GetAll(ctx context.Context) ([]MessageTemplate, error)
	GetPaginated(ctx context.Context, params *FindParams) ([]MessageTemplate, error)
	GetByID(ctx context.Context, id uint) (MessageTemplate, error)
	Create(ctx context.Context, data MessageTemplate) (MessageTemplate, error)
	Update(ctx context.Context, data MessageTemplate) (MessageTemplate, error)
	Delete(ctx context.Context, id uint) error
}
