package debt

import (
	"context"

	"github.com/google/uuid"
)

type DateRange struct {
	From string
	To   string
}

type FindParams struct {
	Limit          int
	Offset         int
	SortBy         []string
	Query          string
	Field          string
	CreatedAt      DateRange
	CounterpartyID *uuid.UUID
	Type           *DebtType
	Status         *DebtStatus
}


type Repository interface {
	Count(ctx context.Context) (int64, error)
	GetAll(ctx context.Context) ([]Debt, error)
	GetPaginated(ctx context.Context, params *FindParams) ([]Debt, error)
	GetByID(ctx context.Context, id uuid.UUID) (Debt, error)
	GetByCounterpartyID(ctx context.Context, counterpartyID uuid.UUID) ([]Debt, error)
	GetCounterpartyAggregates(ctx context.Context) ([]CounterpartyAggregate, error)
	Create(ctx context.Context, debt Debt) (Debt, error)
	Update(ctx context.Context, debt Debt) (Debt, error)
	Delete(ctx context.Context, id uuid.UUID) error
}
