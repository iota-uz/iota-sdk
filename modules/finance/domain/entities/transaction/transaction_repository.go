package transaction

import (
	"context"

	"github.com/google/uuid"
)

type DateRange struct {
	From string
	To   string
}

type FindParams struct {
	Limit     int
	Offset    int
	SortBy    []string
	CreatedAt DateRange
}

type Repository interface {
	Count(ctx context.Context) (int64, error)
	GetAll(ctx context.Context) ([]Transaction, error)
	GetPaginated(ctx context.Context, params *FindParams) ([]Transaction, error)
	GetByID(ctx context.Context, id uuid.UUID) (Transaction, error)
	Create(ctx context.Context, upload Transaction) error
	Update(ctx context.Context, upload Transaction) error
	Delete(ctx context.Context, id uuid.UUID) error
}
