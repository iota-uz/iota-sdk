package prompt

import "context"

type Repository interface {
	Count(ctx context.Context) (int64, error)
	GetAll(ctx context.Context) ([]*Prompt, error)
	GetPaginated(ctx context.Context, limit, offset int, sortBy []string) ([]*Prompt, error)
	GetByID(ctx context.Context, id string) (*Prompt, error)
	Create(ctx context.Context, upload *Prompt) error
	Update(ctx context.Context, upload *Prompt) error
	Delete(ctx context.Context, id int64) error
}
