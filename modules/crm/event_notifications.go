package crm

import (
	"unicode/utf8"

	"github.com/iota-uz/iota-sdk/modules/core/notifications"
	"github.com/iota-uz/iota-sdk/modules/crm/permissions"
	"github.com/iota-uz/iota-sdk/modules/crm/services"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/manifest"
)

type labeledEvent interface {
	notifications.EventRef
	Label() manifest.Label
	Descriptions() manifest.Label
}

func eventNotification(ev labeledEvent, body func(notifications.Event) string, url func(notifications.Event) string) notifications.Definition {
	return notifications.FromEvent(ev,
		notifications.WithModule("crm"),
		notifications.WithRequiredPermission(permissions.ClientRead),
		notifications.WithRecipients(notifications.ActorRecipient()),
		notifications.WithRender(notifications.EventContent(ev.Label(), ev.Descriptions(), body, url)),
	)
}

// preview cuts a message text to a notice-sized body.
func preview(text string) string {
	const limit = 140
	if utf8.RuneCountInString(text) <= limit {
		return text
	}
	return string([]rune(text)[:limit]) + "…"
}

// eventNotifications are the notification definitions of the CRM events,
// contributed to the settings catalog by the component.
func eventNotifications() []notifications.Definition {
	clientName := func(e notifications.Event) string { return notifications.DataName(e, "first_name") }
	clientURL := notifications.DataURL("/crm/clients/", "client_id")
	return []notifications.Definition{
		eventNotification(services.ClientCreatedEvent, clientName, clientURL),
		eventNotification(services.ClientUpdatedEvent, clientName, clientURL),
		eventNotification(services.ClientDeletedEvent, clientName, nil),
		eventNotification(services.ChatMessageAddedEvent, func(e notifications.Event) string { return preview(e.Data["body"]) }, notifications.DataURL("/crm/chats?chat_id=", "chat_id")),
	}
}
