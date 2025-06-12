package inventory

import (
	"context"

	"github.com/google/uuid"
)

type FindParams struct {
	Limit  int
	Offset int
	SortBy []string
	Query  string
	Field  string
}

type Repository interface {
	Count(ctx context.Context) (int64, error)
	GetAll(ctx context.Context) ([]Inventory, error)
	GetPaginated(ctx context.Context, params *FindParams) ([]Inventory, error)
	GetByID(ctx context.Context, id uuid.UUID) (Inventory, error)
	Create(ctx context.Context, data Inventory) (Inventory, error)
	Update(ctx context.Context, data Inventory) (Inventory, error)
	Delete(ctx context.Context, id uuid.UUID) error
}
