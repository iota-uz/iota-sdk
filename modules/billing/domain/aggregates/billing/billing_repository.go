package billing

import (
	"context"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/pkg/repo"
)

type Field int
type ComparisonOperator string

const (
	CreatedAt Field = iota
	TenantIDField
)

const (
	OpEqual   ComparisonOperator = "="
	OpGreater ComparisonOperator = ">"
	OpLess    ComparisonOperator = "<"
	OpGTE     ComparisonOperator = ">="
	OpLTE     ComparisonOperator = "<="
	OpBetween ComparisonOperator = "between"
)

type SortByField = repo.SortByField[Field]
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

type DetailsFieldFilter struct {
	Path     []string
	Operator ComparisonOperator
	Value    any
}

type Repository interface {
	Count(ctx context.Context, params *FindParams) (int64, error)
	GetPaginated(ctx context.Context, params *FindParams) ([]Transaction, error)
	GetByID(ctx context.Context, id uuid.UUID) (Transaction, error)
	GetByDetailsFields(ctx context.Context, gateway Gateway, filters []DetailsFieldFilter) ([]Transaction, error)
	GetAll(ctx context.Context) ([]Transaction, error)
	Save(ctx context.Context, data Transaction) (Transaction, error)
	Delete(ctx context.Context, id uuid.UUID) error
}
