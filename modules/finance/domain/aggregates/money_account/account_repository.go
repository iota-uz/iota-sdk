// Package moneyaccount provides this package.
package moneyaccount

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/pkg/repo"
)

// ErrDuplicateAccountNumber is returned by the repository when an insert or
// update violates the per-tenant unique constraint on
// money_accounts(tenant_id, account_number). Controllers map it to a
// user-facing validation message instead of leaking the raw SQL error.
var ErrDuplicateAccountNumber = errors.New("moneyaccount: account number already exists in tenant")

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
