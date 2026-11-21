package billing

import (
	"github.com/iota-uz/iota-sdk/modules/billing/permissions"
	"github.com/iota-uz/iota-sdk/modules/billing/services"
	"github.com/iota-uz/iota-sdk/modules/core/notifications"
)

// eventNotifications are the notification definitions of the billing events,
// contributed to the settings catalog by the component. Billing transactions
// have no page, so the notice carries no link. A gateway callback has no actor,
// so the notice reaches the actor only when a user changed the status.
func eventNotifications() []notifications.Definition {
	ev := services.TransactionStatusChangedEvent
	return []notifications.Definition{
		notifications.FromEvent(ev,
			notifications.WithModule("billing"),
			notifications.WithRequiredPermission(permissions.BillingTransactionRead),
			notifications.WithRecipients(notifications.ActorRecipient()),
			notifications.WithRender(notifications.EventContent(ev.Label(), ev.Descriptions(), nil, nil)),
		),
	}
}
