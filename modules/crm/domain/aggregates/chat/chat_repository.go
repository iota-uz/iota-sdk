package chat

import "context"

type FindParams struct {
	Limit  int
	Offset int
	SortBy []string
}

type Repository interface {
	Count(ctx context.Context) (int64, error)
	GetAll(ctx context.Context) ([]Chat, error)
	GetPaginated(ctx context.Context, params *FindParams) ([]Chat, error)
	GetByID(ctx context.Context, id uint) (Chat, error)
	GetByPhone(ctx context.Context, phoneNumber string) (Chat, error)
	Create(ctx context.Context, data Chat) (Chat, error)
	Update(ctx context.Context, data Chat) (Chat, error)
	Delete(ctx context.Context, id uint) error
}
