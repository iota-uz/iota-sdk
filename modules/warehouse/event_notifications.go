package warehouse

import (
	"github.com/iota-uz/iota-sdk/modules/core/notifications"
	"github.com/iota-uz/iota-sdk/modules/warehouse/permissions"
	"github.com/iota-uz/iota-sdk/modules/warehouse/services/orderservice"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/manifest"
)

type labeledEvent interface {
	notifications.EventRef
	Label() manifest.Label
	Descriptions() manifest.Label
}

func eventNotification(ev labeledEvent) notifications.Definition {
	return notifications.FromEvent(ev,
		notifications.WithModule("warehouse"),
		notifications.WithRequiredPermission(permissions.OrderRead),
		notifications.WithRecipients(notifications.ActorRecipient()),
		notifications.WithRender(notifications.EventContent(ev.Label(), ev.Descriptions(), nil, func(notifications.Event) string { return "/warehouse/orders" })),
	)
}

// eventNotifications are the notification definitions of the warehouse order
// events, contributed to the settings catalog by the component.
func eventNotifications() []notifications.Definition {
	return []notifications.Definition{
		eventNotification(orderservice.OrderCreatedEvent),
		eventNotification(orderservice.OrderCompletedEvent),
	}
}
