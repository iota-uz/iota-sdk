package billing

import (
	"context"
	"github.com/google/uuid"
)

type Field int
type DetailsField string

const (
	CreatedAt Field = iota
)

const (
	MerchantTransID DetailsField = "merchant_trans_id"
)

type SortBy struct {
	Fields    []Field
	Ascending bool
}

type FindParams struct {
	Limit  int
	Offset int
	SortBy SortBy
}

type Repository interface {
	Count(ctx context.Context) (int64, error)
	GetPaginated(ctx context.Context, params *FindParams) ([]Transaction, error)
	GetByID(ctx context.Context, id uuid.UUID) (Transaction, error)
	GetByDetailsField(ctx context.Context, field DetailsField, value any) (Transaction, error)
	GetAll(ctx context.Context) ([]Transaction, error)
	Save(ctx context.Context, data Transaction) (Transaction, error)
	Delete(ctx context.Context, id uuid.UUID) error
}
