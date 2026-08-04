package billing

import (
	"context"
	"time"
)

type Gateway string

const (
	Click      Gateway = "click"
	Payme      Gateway = "payme"
	Octo       Gateway = "octo"
	Stripe     Gateway = "stripe"
	Cash       Gateway = "cash"
	Transfer   Gateway = "transfer"
	Integrator Gateway = "integrator"
	Uzum       Gateway = "uzum"
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
	ProviderPaymentID string
}

// StatusChecker is an optional interface that providers can implement
// to support checking the current status of a payment.
type StatusChecker interface {
	CheckStatus(ctx context.Context, shopTransactionID string) (*StatusCheckResult, error)
}

type PayoutDocument struct {
	Type       string
	Series     string
	Number     string
	IssuedDate *time.Time
}

type CreatePayoutRequest struct {
	ExternalID      string
	Amount          int64
	Currency        string
	Destination     string
	DestinationName string
	Document        *PayoutDocument
	Metadata        map[string]string
}

type ConfirmPayoutRequest struct {
	ExternalID        string
	ProviderPaymentID string
}

type PayoutResult struct {
	ExternalID        string
	ProviderPaymentID string
	Status            string
	Amount            int64
	Commission        int64
}

// PayoutProvider is an optional capability for gateways that support
// irreversible outbound payments.
type PayoutProvider interface {
	CreatePayout(ctx context.Context, request CreatePayoutRequest) (*PayoutResult, error)
	ConfirmPayout(ctx context.Context, request ConfirmPayoutRequest) (*PayoutResult, error)
	CheckPayout(ctx context.Context, externalID string) (*PayoutResult, error)
}
