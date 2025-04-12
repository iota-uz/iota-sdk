package expense

import (
	"context"
	"github.com/iota-uz/iota-sdk/pkg/repo"
)

type Field int

const (
	ID Field = iota
	TransactionID
	CategoryID
	CreatedAt
	UpdatedAt
)

type DateRange struct {
	From string
	To   string
}

type FindParams struct {
	ID        uint
	Limit     int
	Offset    int
	SortBy    SortBy
	Query     string
	Field     string
	CreatedAt DateRange
	Filters   []FieldFilter
	Search    string
}

type FieldFilter struct {
	Column Field
	Filter repo.Filter
}

type SortBy struct {
	Fields    []Field
	Ascending bool
}

type Repository interface {
	Count(ctx context.Context, params *FindParams) (int64, error)
	GetByID(ctx context.Context, id uint) (Expense, error)
	GetAll(ctx context.Context) ([]Expense, error)
	GetPaginated(ctx context.Context, params *FindParams) ([]Expense, error)
	Create(ctx context.Context, data Expense) error
	Update(ctx context.Context, data Expense) error
	Delete(ctx context.Context, id uint) error
}
