package inventory

import (
	"context"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/pkg/repo"
)

type Field = int

const (
	CreatedAtField Field = iota
	UpdatedAtField
	TenantIDField
	NameField
	DescriptionField
	PriceField
	QuantityField
)

type SortBy = repo.SortBy[Field]
type Filter = repo.FieldFilter[Field]

type FindParams struct {
	Limit   int
	Offset  int
	SortBy  SortBy
	Search  string
	Filters []Filter
}

func (f *FindParams) FilterBy(field Field, filter repo.Filter) *FindParams {
	res := *f
	res.Filters = append(res.Filters, Filter{
		Column: field,
		Filter: filter,
	})
	return &res
}

type Repository interface {
	Count(ctx context.Context, params *FindParams) (int64, error)
	GetAll(ctx context.Context) ([]Inventory, error)
	GetPaginated(ctx context.Context, params *FindParams) ([]Inventory, error)
	GetByID(ctx context.Context, id uuid.UUID) (Inventory, error)
	Create(ctx context.Context, data Inventory) (Inventory, error)
	Update(ctx context.Context, data Inventory) (Inventory, error)
	Delete(ctx context.Context, id uuid.UUID) error
}
