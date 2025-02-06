package chat

import "context"

type Field int

const (
	CreatedAt Field = iota
	LastMessageAt
)

type SortBy struct {
	Fields    []Field
	Ascending bool
}

type FindParams struct {
	Limit  int
	Offset int
	Search string
	SortBy SortBy
}

type Repository interface {
	Count(ctx context.Context) (int64, error)
	GetAll(ctx context.Context) ([]Chat, error)
	GetPaginated(ctx context.Context, params *FindParams) ([]Chat, error)
	GetByID(ctx context.Context, id uint) (Chat, error)
	GetByClientID(ctx context.Context, clientID uint) (Chat, error)
	Create(ctx context.Context, data Chat) (Chat, error)
	Delete(ctx context.Context, id uint) error
}
