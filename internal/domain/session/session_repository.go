package session

import (
	"context"
)

type Repository interface {
	Count(ctx context.Context) (int64, error)
	GetAll(ctx context.Context) ([]*Session, error)
	GetPaginated(ctx context.Context, limit, offset int, sortBy []string) ([]*Session, error)
	GetByToken(ctx context.Context, id string) (*Session, error)
	Create(ctx context.Context, user *Session) error
	Update(ctx context.Context, user *Session) error
	Delete(ctx context.Context, id int64) error
}
