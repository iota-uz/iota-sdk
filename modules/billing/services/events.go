package services

import (
	"context"

	"github.com/google/uuid"

	"github.com/iota-uz/iota-sdk/modules/billing/domain/aggregates/billing"
	"github.com/iota-uz/iota-sdk/modules/billing/permissions"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/events"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/manifest"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/native"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/spi"
	"github.com/iota-uz/iota-sdk/pkg/money"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

const (
	// TransactionResource is the native entity ID of billing transactions.
	TransactionResource = "billing.transaction"
	// TransactionStatusChangedEventID is the versioned ID of the event
	// published when the status of a billing transaction changes.
	TransactionStatusChangedEventID = "billing.transaction.status_changed.v1"
)

// TransactionStatusChangedPayload is the payload of
// billing.transaction.status_changed.v1. Amount is the transaction total in
// minor units.
type TransactionStatusChangedPayload struct {
	TransactionID uuid.UUID  `json:"transactionId"`
	Gateway       string     `json:"gateway"`
	From          string     `json:"from" wire:"enum=created|pending|completed|failed|canceled|refunded|partially-refunded|expired"`
	To            string     `json:"to" wire:"enum=created|pending|completed|failed|canceled|refunded|partially-refunded|expired"`
	Amount        money.Wire `json:"amount"`
}

// TransactionStatusChangedEvent is published by BillingService in the
// transaction that saves the new status, only when the status changed: a
// repeated gateway callback with the same status publishes nothing. A gateway
// callback has no acting user, so the event has no initiator.
var TransactionStatusChangedEvent = native.Event[TransactionStatusChangedPayload](TransactionStatusChangedEventID, native.EventSpec{
	Subject: TransactionResource,
	Label: manifest.Label{
		"en": "Billing transaction status changed", "ru": "Статус billing-транзакции изменился",
		"uz": "Billing tranzaksiyasi holati o'zgardi", "uz-Cyrl": "Billing транзакцияси ҳолати ўзгарди",
	},
	Description: "The status of a billing transaction changed. Published in the transaction that saves the new status, only when the status differs from the previous one.",
	Descriptions: manifest.Label{
		"en":      "The status of a billing transaction changed.",
		"ru":      "Изменился статус billing-транзакции.",
		"uz":      "Billing tranzaksiyasining holati o'zgardi.",
		"uz-Cyrl": "Billing транзакциясининг ҳолати ўзгарди.",
	},
	ReadPermission: permissions.BillingTransactionRead.Name(),
	Category:       "billing.transactions",
})

// Events are the billing events the module contributes to the application's
// native set (billing.Natives).
func Events() []native.Item { return []native.Item{TransactionStatusChangedEvent} }

// publishStatusChange enqueues billing.transaction.status_changed.v1 in the
// transaction held by ctx when the saved transaction left the status from.
func publishStatusChange(ctx context.Context, from billing.Status, saved billing.Transaction) error {
	const op = serrors.Op("BillingService.publishStatusChange")
	if from == saved.Status() {
		return nil
	}
	quantity := saved.Amount().Quantity()
	amount, err := money.ToWire(money.NewFromFloat(quantity, string(saved.Amount().Currency())))
	if err != nil {
		return serrors.Wrap(op, err)
	}
	return serrors.Wrap(op, events.PublishInTx(ctx, saved.TenantID(), TransactionStatusChangedEvent,
		&spi.ObjectRef{Entity: TransactionResource, Key: saved.ID().String()},
		TransactionStatusChangedPayload{
			TransactionID: saved.ID(), Gateway: string(saved.Gateway()),
			From: string(from), To: string(saved.Status()), Amount: amount,
		}))
}
