package projects

import (
	"github.com/iota-uz/iota-sdk/modules/core/notifications"
	"github.com/iota-uz/iota-sdk/modules/projects/permissions"
	"github.com/iota-uz/iota-sdk/modules/projects/services"
)

// eventNotifications are the notification definitions of the projects events,
// contributed to the settings catalog by the component. The notice links to the
// acceptance documents of the project the document belongs to.
func eventNotifications() []notifications.Definition {
	ev := services.AcceptanceSignedEvent
	return []notifications.Definition{
		notifications.FromEvent(ev,
			notifications.WithModule("projects"),
			notifications.WithRequiredPermission(permissions.ProjectRead),
			notifications.WithRecipients(notifications.ActorRecipient()),
			notifications.WithRender(notifications.EventContent(ev.Label(), ev.Descriptions(), nil, func(e notifications.Event) string {
				if id := e.Data["project_id"]; id != "" {
					return "/projects/" + id + "/acceptance"
				}
				return ""
			})),
		),
	}
}
