package counterparty

import (
	"context"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/pkg/repo"
)

type Field = int

const (
	NameField Field = iota
	TinField
	TypeField
	LegalTypeField
	LegalAddressField
	CreatedAtField
	UpdatedAtField
	TenantIDField
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

type Repository interface {
	Count(ctx context.Context, params *FindParams) (int64, error)
	GetAll(context.Context) ([]Counterparty, error)
	GetPaginated(context.Context, *FindParams) ([]Counterparty, error)
	GetByID(context.Context, uuid.UUID) (Counterparty, error)
	Create(context.Context, Counterparty) (Counterparty, error)
	Update(context.Context, Counterparty) (Counterparty, error)
	Delete(context.Context, uuid.UUID) error
}
