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
	GetAll(ctx context.Context) ([]AuthLog, error)
	GetPaginated(ctx context.Context, params *FindParams) ([]AuthLog, error)
	GetByID(ctx context.Context, id uint) (AuthLog, error)
	Create(ctx context.Context, upload AuthLog) error
	Update(ctx context.Context, upload AuthLog) error
	Delete(ctx context.Context, id uint) error
}
