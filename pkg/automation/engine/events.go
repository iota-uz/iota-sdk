package engine

import (
	"encoding/json"

	"github.com/iota-uz/iota-sdk/pkg/automation/catalog"
	"github.com/iota-uz/iota-sdk/pkg/automation/expr"
	"github.com/iota-uz/iota-sdk/pkg/automation/graph"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/manifest"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/native"
)

// Event keys the engine contributes to the catalog (Runtime.Triggers).
const (
	// EventSchedule starts a run on a cron schedule: trigger params {cron, timezone}.
	EventSchedule = "automation.schedule.v1"
	// EventManual starts a run on request (StartManual): trigger params {input_schema}.
	EventManual = "automation.manual.v1"
	// EventRunFinished is published when a run finishes; it only wakes waits (NoTrigger).
	EventRunFinished = "automation.run.finished.v1"
)

// EventRunFailed is published when a run fails, for its graph's owner (owner_user_id): the notification
// definition of package ownernotice delivers it. It only wakes waits (NoTrigger).
const EventRunFailed = "automation.run.failed.v1"

// RunFailedPayload is the payload of automation.run.failed.v1. It carries no error message: a message
// may hold values of sensitive fields the owner may not read.
type RunFailedPayload struct {
	RunID          string `json:"run_id"`
	AutomationID   string `json:"automation_id"`
	AutomationName string `json:"automation_name"`
	Version        int    `json:"version"`
	OwnerUserID    int64  `json:"owner_user_id,string" wire:"int64"`
	Code           string `json:"code"`
}

// RunFailedEvent is the native event of a failed run, published once per run in the transaction in which its
// instance fails.
var RunFailedEvent = native.Event[RunFailedPayload](EventRunFailed, native.EventSpec{
	Subject:      SubjectRun,
	Label:        manifest.Label{"en": "Automation run failed", "ru": "Запуск автоматизации завершился ошибкой", "uz": "Avtomatlashtirish ishga tushishi xato bilan tugadi"},
	Descriptions: manifest.Label{"en": "A run of an automation you own failed and needs attention.", "ru": "Запуск вашей автоматизации завершился ошибкой и требует внимания.", "uz": "Sizning avtomatlashtirishingiz ishga tushishi xato bilan tugadi."},
	Category:     "automation",
})

// SubjectRun is the entity of EventRunFinished: a run, keyed by its ID.
const SubjectRun = "automation.run"

// RunFinishedPayload is the payload of automation.run.finished.v1.
type RunFinishedPayload struct {
	RunID        string          `json:"runId"`
	AutomationID string          `json:"automationId"`
	Version      int             `json:"version"`
	Status       string          `json:"status"`
	Output       json.RawMessage `json:"output"`
	// OutputWithheld: the run read sensitive event fields, so its output (which may derive from them)
	// is not carried by this generic event; Output is null. Read it from the run with its access rules.
	OutputWithheld bool `json:"outputWithheld,omitempty"`
}

// RunFinished is the native event the engine publishes when a run's instance completes, fails or is
// terminated. Register it in the application's native.Set only for its outbox contract: the catalog
// learns it, with NoTrigger, from Runtime.Triggers.
var RunFinished = native.Event[RunFinishedPayload](EventRunFinished, native.EventSpec{
	Subject: SubjectRun,
	Label:   manifest.Label{"en": "Automation run finished", "ru": "Запуск автоматизации завершён", "uz": "Avtomatlashtirish ishga tushishi yakunlandi"},
	Payload: &manifest.Type{Kind: manifest.KindObject, Properties: []manifest.Property{
		{Name: "runId", Type: manifest.Type{Kind: manifest.KindString}},
		{Name: "automationId", Type: manifest.Type{Kind: manifest.KindString}},
		{Name: "version", Type: manifest.Type{Kind: manifest.KindInt}},
		{Name: "status", Type: manifest.Type{Kind: manifest.KindString}},
		{Name: "output", Type: manifest.Type{Kind: manifest.KindJSON, Optional: true, Nullable: true}},
		{Name: "outputWithheld", Type: manifest.Type{Kind: manifest.KindBool, Optional: true}},
	}},
	Category: "automation",
})

const scheduleParamsSchema = `{"type": "object", "properties": {
	"cron": {"type": "string", "minLength": 9, "maxLength": 120, "description": "5-field cron expression"},
	"timezone": {"type": "string", "maxLength": 64, "description": "IANA time zone; UTC by default"}
}, "required": ["cron"], "additionalProperties": false}`

const manualParamsSchema = `{"type": "object", "properties": {
	"input_schema": {"type": "object", "description": "JSON Schema of the input a person enters when starting the run"}
}, "additionalProperties": false}`

// engineTriggers is the trigger source of the engine's own events.
type engineTriggers struct{}

// Triggers is the trigger source of the engine's events: schedule and manual triggers and the run-finished
// signal. Register it in the catalog registry.
func Triggers() catalog.TriggerSource { return engineTriggers{} }

func (engineTriggers) Events() []graph.EventSpec {
	return []graph.EventSpec{
		{
			Key: EventSchedule, Title: "Schedule", Group: "automation", Subject: graph.SubjectNone,
			Description:   "Starts the graph on a cron schedule.",
			Payload:       expr.Fields{{Name: "scheduled_at", Field: expr.StringField}},
			TriggerParams: graph.MustSchemaParams(scheduleParamsSchema),
		},
		{
			Key: EventManual, Title: "Manual start", Group: "automation", Subject: graph.SubjectNone,
			Description:   "Starts the graph when a person presses Run.",
			Payload:       expr.Fields{{Name: "input", Field: expr.DynField}, {Name: "initiator_user_id", Field: expr.IntField}},
			TriggerParams: graph.MustSchemaParams(manualParamsSchema),
		},
		{
			Key: EventRunFinished, Title: "Automation run finished", Group: "automation", Subject: SubjectRun, NoTrigger: true,
			Description: "A run of a graph finished.",
			Payload: expr.Fields{
				{Name: "runId", Field: expr.StringField},
				{Name: "automationId", Field: expr.StringField},
				{Name: "version", Field: expr.IntField},
				{Name: "status", Field: expr.StringField},
				{Name: "output", Field: expr.DynField},
				{Name: "outputWithheld", Field: expr.OptionalOf(expr.BoolField)},
			},
		},
		{
			Key: EventRunFailed, Title: "Automation run failed", Group: "automation", Subject: SubjectRun, NoTrigger: true,
			Description: "A run of a graph failed.",
			Payload: expr.Fields{
				{Name: "run_id", Field: expr.StringField},
				{Name: "automation_id", Field: expr.StringField},
				{Name: "automation_name", Field: expr.StringField},
				{Name: "version", Field: expr.IntField},
				{Name: "owner_user_id", Field: expr.StringField},
				{Name: "code", Field: expr.StringField},
			},
		},
	}
}

func (engineTriggers) Requires() []catalog.ConnectorKind { return nil }

// Trigger kinds.
const (
	KindEvent    = "event"
	KindSchedule = "schedule"
	KindManual   = "manual"
	KindInbound  = "inbound"
)

// triggerKind is the kind of a trigger on an event of the catalog view.
func triggerKind(v catalog.View, event string) string {
	switch event {
	case EventSchedule:
		return KindSchedule
	case EventManual:
		return KindManual
	}
	if len(v.EventRequires[event]) > 0 {
		return KindInbound
	}
	return KindEvent
}
