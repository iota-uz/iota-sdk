// Package department provides this package.
package department

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
	CodeField
	ParentIDField
	OrderField
	StatusField
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
	GetPaginated(ctx context.Context, params *FindParams) ([]Department, error)
	GetByID(ctx context.Context, id uuid.UUID) (Department, error)
	Save(ctx context.Context, department Department) (Department, error)
	Exists(ctx context.Context, id uuid.UUID) (bool, error)
	Delete(ctx context.Context, id uuid.UUID) error
}
