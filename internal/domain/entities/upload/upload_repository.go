package upload

import (
	"context"
)

type Repository interface {
	Count(ctx context.Context) (int64, error)
	GetAll(ctx context.Context) ([]*Upload, error)
	GetPaginated(ctx context.Context, limit, offset int, sortBy []string) ([]*Upload, error)
	GetByID(ctx context.Context, id int64) (*Upload, error)
	Create(ctx context.Context, upload *Upload) error
	Update(ctx context.Context, upload *Upload) error
	Delete(ctx context.Context, id int64) error
}
