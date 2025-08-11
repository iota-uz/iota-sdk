package moneyaccount

import (
	"context"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/pkg/repo"
)

type Field int

const (
	ID Field = iota
	Name
	AccountNumber
	Balance
	Description
	CurrencyCode
	CreatedAt
	UpdatedAt
)

type SortBy = repo.SortBy[Field]

type Filter = repo.FieldFilter[Field]

type FindParams struct {
	ID      uuid.UUID
	Limit   int
	Offset  int
	SortBy  SortBy
	Filters []Filter
	Search  string
}

type Repository interface {
	Count(ctx context.Context, params *FindParams) (int64, error)
	GetAll(ctx context.Context) ([]Account, error)
	GetPaginated(ctx context.Context, params *FindParams) ([]Account, error)
	GetByID(ctx context.Context, id uuid.UUID) (Account, error)
	RecalculateBalance(ctx context.Context, id uuid.UUID) error
	Create(ctx context.Context, data Account) (Account, error)
	Update(ctx context.Context, data Account) (Account, error)
	Delete(ctx context.Context, id uuid.UUID) error
}
