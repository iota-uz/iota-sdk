package message

import "context"

type Field int

const (
	ReadAt Field = iota
	CreatedAt
)

type SortBy struct {
	Fields    []Field
	Ascending bool
}

type FindParams struct {
	Limit  int
	Offset int
	Search string
	ChatID uint
	SortBy SortBy
}

type Repository interface {
	Count(ctx context.Context) (int64, error)
	GetAll(ctx context.Context) ([]Message, error)
	GetPaginated(ctx context.Context, params *FindParams) ([]Message, error)
	GetByID(ctx context.Context, id uint) (Message, error)
	Create(ctx context.Context, data Message) (Message, error)
	Update(ctx context.Context, data Message) (Message, error)
	Delete(ctx context.Context, id uint) error
}
