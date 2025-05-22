package billing

import (
	"context"
)

type Gateway string

const (
	Click  Gateway = "click"
	Payme  Gateway = "payme"
	Octo   Gateway = "octo"
	Stripe Gateway = "stripe"
)

type Provider interface {
	Gateway() Gateway
	Create(ctx context.Context, tx Transaction) (Transaction, error)
	Cancel(ctx context.Context, tx Transaction) (Transaction, error)
	Refund(ctx context.Context, tx Transaction, quantity float64) (Transaction, error)
}
