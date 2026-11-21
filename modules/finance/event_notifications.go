package finance

import (
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/permission"
	"github.com/iota-uz/iota-sdk/modules/core/notifications"
	"github.com/iota-uz/iota-sdk/modules/finance/permissions"
	"github.com/iota-uz/iota-sdk/modules/finance/services"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/manifest"
)

// labeledEvent is a native event with its localized presentation metadata
// (*native.EventDef satisfies it).
type labeledEvent interface {
	notifications.EventRef
	Label() manifest.Label
	Descriptions() manifest.Label
}

// eventNotification is the notification definition of one finance event: the
// recipient is the user who made the change, the title and text come from the
// event, and the link opens the list of the entity (finance entities have
// drawers on their lists, not pages of their own).
func eventNotification(ev labeledEvent, required permission.Permission, list string) notifications.Definition {
	return notifications.FromEvent(ev,
		notifications.WithModule("finance"),
		notifications.WithRequiredPermission(required),
		notifications.WithRecipients(notifications.ActorRecipient()),
		notifications.WithRender(notifications.EventContent(ev.Label(), ev.Descriptions(), nil, func(notifications.Event) string { return list })),
	)
}

// eventNotifications are the notification definitions of the finance events,
// contributed to the settings catalog by the component.
func eventNotifications() []notifications.Definition {
	return []notifications.Definition{
		eventNotification(services.PaymentCreatedEvent, permissions.PaymentRead, "/finance/payments"),
		eventNotification(services.PaymentUpdatedEvent, permissions.PaymentRead, "/finance/payments"),
		eventNotification(services.ExpenseCreatedEvent, permissions.ExpenseRead, "/finance/expenses"),
		eventNotification(services.DebtCreatedEvent, permissions.DebtRead, "/finance/debts"),
		eventNotification(services.DebtSettledEvent, permissions.DebtRead, "/finance/debts"),
		eventNotification(services.DebtWrittenOffEvent, permissions.DebtRead, "/finance/debts"),
		eventNotification(services.DebtCancelledEvent, permissions.DebtRead, "/finance/debts"),
	}
}
