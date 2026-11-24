// Package notify holds the built-in notification nodes of automation graphs: notify, which sends a notice to
// users, groups, roles or the participants of the trigger event, and send_to_contact, which writes to the
// phone or the email of a record. Both are effect nodes: they queue the notice with
// notifications.AudienceSender.SendTx in the transaction of the step, so the notice commits with the step,
// and the delivery layer of the SDK (channel preferences, retries, the subject access check) does the rest
// after the commit. The nodes resolve no audience and check no access themselves.
package notify

import (
	"github.com/iota-uz/iota-sdk/modules/core/notifications"
	"github.com/iota-uz/iota-sdk/pkg/composition"
)

// EventKey is the key of the notification definition every notice of the nodes carries.
const EventKey = "automations.notify.v1"

// Module is the module the definition is listed under.
const Module = "automations"

// Fields of the notice's Data: what the graph wrote.
const (
	dataTitle     = "title"
	dataBody      = "body"
	dataActionURL = "action_url"
)

// Definition is the notification definition automations.notify.v1: the title, the text and the link are
// whatever the graph wrote; the recipients, the channels and the level are the node's.
func Definition() notifications.Definition {
	name := map[string]string{
		"en": "Automation notification", "ru": "Уведомление автоматизации",
		"uz": "Avtomatlashtirish bildirishnomasi", "uz-Cyrl": "Автоматлаштириш билдиришномаси",
	}
	description := map[string]string{
		"en":      "A notice sent by a node of an automation graph. The title and the text are written in the graph.",
		"ru":      "Уведомление, которое отправил узел графа автоматизации. Заголовок и текст задаёт граф.",
		"uz":      "Avtomatlashtirish grafining tuguni yuborgan bildirishnoma. Sarlavha va matnni graf belgilaydi.",
		"uz-Cyrl": "Автоматлаштириш графининг тугуни юборган билдиришнома. Сарлавҳа ва матнни граф белгилайди.",
	}
	return notifications.Definition{
		Module:      Module,
		Key:         EventKey,
		Name:        name,
		Description: description,
		PayloadFields: []notifications.PayloadField{
			{Key: dataTitle, Description: map[string]string{"en": "Title"}},
			{Key: dataBody, Description: map[string]string{"en": "Text"}, Required: true},
			{Key: dataActionURL, Description: map[string]string{"en": "Link inside the application"}},
		},
		Render: func(e notifications.Event, _ string) (notifications.Content, error) {
			return notifications.Content{Title: e.Data[dataTitle], Body: e.Data[dataBody], ActionURL: e.Data[dataActionURL]}, nil
		},
	}
}

// Contribute registers the definition with the notification catalog of the composition. A composition
// that runs the automation engine contributes it next to ownernotice.Contribute.
func Contribute(builder *composition.Builder) {
	notifications.ContributeDefinitions(builder, Definition())
}
