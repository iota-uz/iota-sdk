package authlog

import (
	"context"
)

type FindParams struct {
	ID     uint
	UserID uint
	Limit  int
	Offset int
	SortBy []string
}

type Repository interface {
	Count(ctx context.Context) (int64, error)
	GetAll(ctx context.Context) ([]*AuthenticationLog, error)
	GetPaginated(ctx context.Context, params *FindParams) ([]*AuthenticationLog, error)
	GetByID(ctx context.Context, id uint) (*AuthenticationLog, error)
	Create(ctx context.Context, upload *AuthenticationLog) error
	Update(ctx context.Context, upload *AuthenticationLog) error
	Delete(ctx context.Context, id uint) error
}
