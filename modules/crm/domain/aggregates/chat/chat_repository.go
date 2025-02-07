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
	Update(ctx context.Context, data Chat) (Chat, error)
	Delete(ctx context.Context, id uint) error
}

type MessageField int

const (
	MessageReadAt MessageField = iota
	MessageCreatedAt
)

type MessageFindParams struct {
	Limit  int
	Offset int
	Search string
	ChatID uint
	SortBy SortBy
}

type MessageRepository interface {
	Count(ctx context.Context) (int64, error)
	GetAll(ctx context.Context) ([]Message, error)
	GetPaginated(ctx context.Context, params *FindParams) ([]Message, error)
	GetByID(ctx context.Context, id uint) (Message, error)
	Create(ctx context.Context, data Message) (Message, error)
	Update(ctx context.Context, data Message) (Message, error)
	Delete(ctx context.Context, id uint) error
}
