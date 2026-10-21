package services

import (
	"context"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/pkg/money"
)

// Revenue splits what is due from a client in one currency by its basis: the
// contract, what signed acceptance documents recognised, what the contract
// still has open, what was invoiced and what was paid. Every amount is in the
// same currency, zero when there is none.
type Revenue struct {
	Contract *money.Money
	Accepted *money.Money
	Open     *money.Money
	Invoiced *money.Money
	Paid     *money.Money
}

// ClientRevenueSource gives a client's revenue per currency. Finance has no
// projects of its own: the projects module provides it, and without it a
// client has none.
type ClientRevenueSource interface {
	ClientRevenue(ctx context.Context, counterpartyID uuid.UUID) ([]Revenue, error)
}

type noClientRevenue struct{}

// NewNoClientRevenue is the source used when no module provides revenue.
func NewNoClientRevenue() ClientRevenueSource {
	return noClientRevenue{}
}

func (noClientRevenue) ClientRevenue(context.Context, uuid.UUID) ([]Revenue, error) {
	return nil, nil
}
