package services

import (
	"context"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/pkg/money"
)

// InvoiceSource sums what was invoiced on projects, one amount per currency.
// Projects do not issue invoices themselves: an application that has them
// provides the source, and without it nothing counts as invoiced.
type InvoiceSource interface {
	Invoiced(ctx context.Context, projectIDs []uuid.UUID) ([]*money.Money, error)
}

type noInvoices struct{}

// NewNoInvoices is the source used when the application has no invoices.
func NewNoInvoices() InvoiceSource {
	return noInvoices{}
}

func (noInvoices) Invoiced(context.Context, []uuid.UUID) ([]*money.Money, error) {
	return nil, nil
}
