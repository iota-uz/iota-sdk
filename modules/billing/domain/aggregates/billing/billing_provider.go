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
	Create(ctx context.Context, t Transaction) (Transaction, error)
	Cancel(ctx context.Context, t Transaction) (Transaction, error)
	Refund(ctx context.Context, t Transaction, quantity float64) (Transaction, error)
}
