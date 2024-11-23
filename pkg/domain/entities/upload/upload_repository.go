package upload

import "context"

type FindParams struct {
	Limit  int
	Offset int
	SortBy []string
	Search string
	Type   string
}

type Repository interface {
	Count(ctx context.Context) (int64, error)
	GetAll(ctx context.Context) ([]*Upload, error)
	GetPaginated(ctx context.Context, params *FindParams) ([]*Upload, error)
	GetByID(ctx context.Context, id string) (*Upload, error)
	Create(ctx context.Context, data *Upload) error
	Update(ctx context.Context, data *Upload) error
	Delete(ctx context.Context, id string) error
}
