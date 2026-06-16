// Package department provides this package.
package department

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/pkg/repo"
)

// Repository contract errors. These are part of the Repository interface's
// public surface so consumers (services, controllers, seeders) can match a
// specific failure with errors.Is without taking a dependency on any concrete
// infrastructure package. Repository implementations must wrap the underlying
// driver error so errors.Is(err, ErrNotFound) / ErrDuplicateCode resolves
// correctly.
var (
	// ErrNotFound is returned by Repository.GetByID / Delete when the row does
	// not exist (or sits in a different tenant — the persistence layer scopes
	// every query to the caller tenant, so a cross-tenant id surfaces as
	// not-found).
	ErrNotFound = errors.New("department not found")
	// ErrDuplicateCode is returned by Repository.Save when the (tenant_id, code)
	// pair already exists. Admin controllers map this to a field-level "code
	// already in use" message on the Code input.
	ErrDuplicateCode = errors.New("department: code already exists in tenant")
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
	// Deep-copy Filters so the shallow struct copy does not share the backing
	// array with the receiver (appending below could otherwise mutate it).
	res.Filters = make([]Filter, len(f.Filters), len(f.Filters)+1)
	copy(res.Filters, f.Filters)
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
	GetByIDs(ctx context.Context, ids []uuid.UUID) ([]Department, error)
	Save(ctx context.Context, department Department) (Department, error)
	Exists(ctx context.Context, id uuid.UUID) (bool, error)
	Delete(ctx context.Context, id uuid.UUID) error
}
