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

type FindParams struct {
	Limit  int
	Offset int
	SortBy SortBy
}

type DetailsFieldFilter struct {
	Path     []string
	Operator ComparisonOperator
	Value    any
}

type Repository interface {
	Count(ctx context.Context) (int64, error)
	GetPaginated(ctx context.Context, params *FindParams) ([]Transaction, error)
	GetByID(ctx context.Context, id uuid.UUID) (Transaction, error)
	GetByDetailsFields(ctx context.Context, gateway Gateway, filters []DetailsFieldFilter) ([]Transaction, error)
	GetAll(ctx context.Context) ([]Transaction, error)
	Save(ctx context.Context, data Transaction) (Transaction, error)
	Delete(ctx context.Context, id uuid.UUID) error
}
