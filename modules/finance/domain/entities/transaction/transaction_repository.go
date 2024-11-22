package transaction

import (
	"context"
)

type Repository interface {
	Count(ctx context.Context) (int64, error)
	GetAll(ctx context.Context) ([]*Transaction, error)
	GetPaginated(ctx context.Context, limit, offset int, sortBy []string) ([]*Transaction, error)
	GetByID(ctx context.Context, id int64) (*Transaction, error)
	Create(ctx context.Context, upload *Transaction) error
	Update(ctx context.Context, upload *Transaction) error
	Delete(ctx context.Context, id int64) error
}
