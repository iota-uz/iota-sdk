package billing

import (
	"context"
)

type Gateway string

const (
	Click      Gateway = "click"
	Payme      Gateway = "payme"
	Octo       Gateway = "octo"
	Stripe     Gateway = "stripe"
	Cash       Gateway = "cash"
	Integrator Gateway = "integrator"
)

type Provider interface {
	Gateway() Gateway
	Create(ctx context.Context, t Transaction) (Transaction, error)
	Cancel(ctx context.Context, t Transaction) (Transaction, error)
	Refund(ctx context.Context, t Transaction, quantity float64) (Transaction, error)
}

// StatusCheckResult represents the result of checking a payment status.
type StatusCheckResult struct {
	Status            string
	ShopTransactionID string
	OctoPaymentUUID   string
}

// StatusChecker is an optional interface that providers can implement
// to support checking the current status of a payment.
type StatusChecker interface {
	CheckStatus(ctx context.Context, shopTransactionId string) (*StatusCheckResult, error)
}
