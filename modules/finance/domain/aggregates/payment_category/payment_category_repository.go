package payment_category

import (
	"context"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/pkg/repo"
)

type Field int

const (
	ID Field = iota
	Name
	Description
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
	GetAll(ctx context.Context) ([]PaymentCategory, error)
	GetPaginated(ctx context.Context, params *FindParams) ([]PaymentCategory, error)
	GetByID(ctx context.Context, id uuid.UUID) (PaymentCategory, error)
	Create(ctx context.Context, category PaymentCategory) (PaymentCategory, error)
	Update(ctx context.Context, category PaymentCategory) (PaymentCategory, error)
	Delete(ctx context.Context, id uuid.UUID) error
}
