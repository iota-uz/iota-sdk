package currency

import (
	"context"

	"github.com/iota-uz/iota-sdk/pkg/repo"
)

type Field int

const (
	FieldCode Field = iota
	FieldName
	FieldSymbol
	FieldCreatedAt
)

type SortByField = repo.SortByField[Field]
type SortBy = repo.SortBy[Field]

type FindParams struct {
	Code   string
	Limit  int
	Offset int
	SortBy SortBy
}

type Repository interface {
	Count(ctx context.Context) (uint, error)
	GetAll(ctx context.Context) ([]*Currency, error)
	GetPaginated(ctx context.Context, params *FindParams) ([]*Currency, error)
	GetByCode(ctx context.Context, code string) (*Currency, error)
	CreateOrUpdate(ctx context.Context, currency *Currency) error
	Create(ctx context.Context, currency *Currency) error
	Update(ctx context.Context, payment *Currency) error
	Delete(ctx context.Context, code string) error
}
