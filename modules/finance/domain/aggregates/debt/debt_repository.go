// Package debt provides this package.
package debt

import (
	"context"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/pkg/money"
	"github.com/iota-uz/iota-sdk/pkg/repo"
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

// AggregateField is what the counterparty summary can be filtered on. The
// debt fields narrow which debts are summed; AggregateOutstandingField is the
// outstanding total of a counterparty and filters the summed rows.
type AggregateField int

const (
	// AggregateCounterpartyField is the counterparty of a debt.
	AggregateCounterpartyField AggregateField = iota
	// AggregateCurrencyField is the currency of a debt's original amount.
	AggregateCurrencyField
	// AggregateTypeField is the debt type (receivable or payable).
	AggregateTypeField
	// AggregateOutstandingField is the outstanding amount of a counterparty,
	// summed over its debts in minor units.
	AggregateOutstandingField
)

// AggregateFilter is one condition of the counterparty summary.
type AggregateFilter = repo.FieldFilter[AggregateField]

type Repository interface {
	Count(ctx context.Context) (int64, error)
	GetAll(ctx context.Context) ([]Debt, error)
	GetPaginated(ctx context.Context, params *FindParams) ([]Debt, error)
	GetByID(ctx context.Context, id uuid.UUID) (Debt, error)
	// Lock takes the row lock of the debt until the transaction ends, so a
	// read-modify-write of it is serialized with concurrent ones.
	Lock(ctx context.Context, id uuid.UUID) error
	GetByCounterpartyID(ctx context.Context, counterpartyID uuid.UUID) ([]Debt, error)
	// GetCounterpartyAggregates sums the debts per counterparty, limited by
	// the filters (all AND-ed).
	GetCounterpartyAggregates(ctx context.Context, filters ...AggregateFilter) ([]CounterpartyAggregate, error)
	// OpenPayableTotals sums what is still owed on open payable debts, one
	// amount per currency. A non-nil accountID limits it to that account.
	OpenPayableTotals(ctx context.Context, accountID *uuid.UUID) ([]*money.Money, error)
	// OpenPayables lists the open payable debts to be paid from the account,
	// soonest due first.
	OpenPayables(ctx context.Context, accountID uuid.UUID) ([]Debt, error)
	Create(ctx context.Context, debt Debt) (Debt, error)
	Update(ctx context.Context, debt Debt) (Debt, error)
	Delete(ctx context.Context, id uuid.UUID) error
}
