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

// Provider is an interface that defines the methods that a payment provider must implement.
type Provider interface {
	// Gateway returns the gateway of the provider.
	Gateway() Gateway
	// Create creates a new transaction with the provider.
	Create(ctx context.Context, t Transaction) (Transaction, error)
	// Cancel cancels a transaction with the provider.
	Cancel(ctx context.Context, t Transaction) (Transaction, error)
	// Refund refunds a transaction with the provider.
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
