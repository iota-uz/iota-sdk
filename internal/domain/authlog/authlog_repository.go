package authlog

import (
	"context"
)

type Repository interface {
	Count(ctx context.Context) (int64, error)
	GetAll(ctx context.Context) ([]*AuthenticationLog, error)
	GetPaginated(ctx context.Context, limit, offset int, sortBy []string) ([]*AuthenticationLog, error)
	GetByID(ctx context.Context, id int64) (*AuthenticationLog, error)
	Create(ctx context.Context, upload *AuthenticationLog) error
	Update(ctx context.Context, upload *AuthenticationLog) error
	Delete(ctx context.Context, id int64) error
}
