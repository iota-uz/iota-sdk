package payment

import (
	"context"
)

type Repository interface {
	Count(ctx context.Context) (uint, error)
	GetAll(ctx context.Context) ([]*Payment, error)
	GetPaginated(ctx context.Context, limit, offset int, sortBy []string) ([]*Payment, error)
	GetByID(ctx context.Context, id uint) (*Payment, error)
	Create(ctx context.Context, payment *Payment) error
	Update(ctx context.Context, payment *Payment) error
	Delete(ctx context.Context, id uint) error
}
