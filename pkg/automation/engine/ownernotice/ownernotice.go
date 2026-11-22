// Package ownernotice delivers automation.run.failed.v1 to the owner of the failed graph through the
// core notification service: the engine publishes the event in the transaction in which the run fails,
// and the core outbox bridge enqueues the notice of this definition for the owner. A composition that
// runs the engine contributes it with Contribute.
package ownernotice

import (
	"fmt"

	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/notification"
	"github.com/iota-uz/iota-sdk/modules/core/notifications"
	"github.com/iota-uz/iota-sdk/pkg/automation/engine"
	"github.com/iota-uz/iota-sdk/pkg/composition"
)

// RecipientOwner is the semantic recipient of the definition: the user who last enabled the graph.
const RecipientOwner = "owner"

// Definition is the notification of a failed run: to the graph's owner by default, at warning level,
// with a link to the run.
func Definition() notifications.Definition {
	ev := engine.RunFailedEvent
	return notifications.FromEvent(ev,
		notifications.WithModule("automations"),
		notifications.WithLevel(notification.LevelWarning),
		notifications.WithRecipients(notifications.DataUserRecipient(RecipientOwner, map[string]string{
			"en": "Automation owner", "ru": "Владелец автоматизации", "uz": "Avtomatlashtirish egasi", "uz-Cyrl": "Автоматлаштириш эгаси",
		}, "owner_user_id")),
		notifications.WithDefaultRecipients(RecipientOwner),
		notifications.WithDedupeKey(func(e notifications.Event) string { return "automation.run.failed:" + e.Data["run_id"] }),
		notifications.WithRender(notifications.EventContent(ev.Label(), ev.Descriptions(), body, runURL)),
	)
}

func body(e notifications.Event) string {
	return fmt.Sprintf("%s (v%s): %s", e.Data["automation_name"], e.Data["version"], e.Data["code"])
}

func runURL(e notifications.Event) string {
	if e.Data["automation_id"] == "" || e.Data["run_id"] == "" {
		return ""
	}
	return "/automations/" + e.Data["automation_id"] + "/runs/" + e.Data["run_id"]
}

// Contribute registers the definition with the notification catalog of the composition.
func Contribute(builder *composition.Builder) {
	notifications.ContributeDefinitions(builder, Definition())
}
