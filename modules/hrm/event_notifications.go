package hrm

import (
	"github.com/iota-uz/iota-sdk/modules/core/notifications"
	"github.com/iota-uz/iota-sdk/modules/hrm/permissions"
	"github.com/iota-uz/iota-sdk/modules/hrm/services"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/manifest"
)

type labeledEvent interface {
	notifications.EventRef
	Label() manifest.Label
	Descriptions() manifest.Label
}

func eventNotification(ev labeledEvent, body func(notifications.Event) string) notifications.Definition {
	return notifications.FromEvent(ev,
		notifications.WithModule("hrm"),
		notifications.WithRequiredPermission(permissions.EmployeeRead),
		notifications.WithRecipients(notifications.ActorRecipient()),
		notifications.WithRender(notifications.EventContent(ev.Label(), ev.Descriptions(), body, notifications.DataURL("/hrm/employees/", "employee_id"))),
	)
}

// eventNotifications are the notification definitions of the HRM events,
// contributed to the settings catalog by the component.
func eventNotifications() []notifications.Definition {
	return []notifications.Definition{
		eventNotification(services.EmployeeCreatedEvent, func(e notifications.Event) string { return notifications.DataName(e, "first_name", "last_name") }),
		eventNotification(services.EmployeeUpdatedEvent, nil),
	}
}
