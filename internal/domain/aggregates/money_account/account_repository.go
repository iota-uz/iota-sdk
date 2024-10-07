package moneyaccount

import (
	"context"
)

type Repository interface {
	Count(ctx context.Context) (uint, error)
	GetAll(ctx context.Context) ([]*Account, error)
	GetPaginated(ctx context.Context, limit, offset int, sortBy []string) ([]*Account, error)
	GetByID(ctx context.Context, id uint) (*Account, error)
	Create(ctx context.Context, payment *Account) error
	Update(ctx context.Context, payment *Account) error
	Delete(ctx context.Context, id uint) error
}
