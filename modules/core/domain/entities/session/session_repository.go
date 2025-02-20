package session

import "context"

type Field int

const (
	ExpiresAt Field = iota
	CreatedAt
)

type SortBy struct {
	Fields    []Field
	Ascending bool
}

type FindParams struct {
	Limit  int
	Offset int
	Token  string
	SortBy SortBy
}

type Repository interface {
	Count(ctx context.Context) (int64, error)
	GetAll(ctx context.Context) ([]*Session, error)
	GetPaginated(ctx context.Context, params *FindParams) ([]*Session, error)
	GetByToken(ctx context.Context, token string) (*Session, error)
	Create(ctx context.Context, user *Session) error
	Update(ctx context.Context, user *Session) error
	Delete(ctx context.Context, token string) error
}
