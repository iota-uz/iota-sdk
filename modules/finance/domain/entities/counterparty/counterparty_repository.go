package counterparty

import (
	"context"
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
	Count(context.Context) (int64, error)
	GetAll(context.Context) ([]Counterparty, error)
	GetPaginated(context.Context, *FindParams) ([]Counterparty, error)
	GetByID(context.Context, uint) (Counterparty, error)
	Create(context.Context, Counterparty) (Counterparty, error)
	Update(context.Context, Counterparty) error
	Delete(context.Context, uint) error
}
