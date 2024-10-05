package currency

import (
	"context"
)

type Repository interface {
	Count(ctx context.Context) (uint, error)
	GetAll(ctx context.Context) ([]*Currency, error)
	GetPaginated(ctx context.Context, limit, offset int, sortBy []string) ([]*Currency, error)
	GetByID(ctx context.Context, id uint) (*Currency, error)
	CreateOrUpdate(ctx context.Context, currency *Currency) error
	Create(ctx context.Context, currency *Currency) error
	Update(ctx context.Context, payment *Currency) error
	Delete(ctx context.Context, id uint) error
}
