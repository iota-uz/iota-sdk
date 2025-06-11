package moneyaccount

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
	Query     string
	Field     string
	CreatedAt DateRange
}

type Repository interface {
	Count(ctx context.Context) (int64, error)
	GetAll(ctx context.Context) ([]Account, error)
	GetPaginated(ctx context.Context, params *FindParams) ([]Account, error)
	GetByID(ctx context.Context, id uuid.UUID) (Account, error)
	RecalculateBalance(ctx context.Context, id uuid.UUID) error
	Create(ctx context.Context, data Account) (Account, error)
	Update(ctx context.Context, data Account) error
	Delete(ctx context.Context, id uuid.UUID) error
}
