package services

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/iota-uz/iota-sdk/modules/projects/domain/aggregates/acceptance"
	"github.com/iota-uz/iota-sdk/modules/projects/permissions"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/events"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/manifest"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/native"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/spi"
	"github.com/iota-uz/iota-sdk/pkg/money"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

// AcceptanceSignedEventID is the versioned ID of the event published when an
// acceptance document (an act, a delivery note) is signed.
const AcceptanceSignedEventID = "projects.acceptance.signed.v1"

// AcceptanceSignedPayload is the payload of projects.acceptance.signed.v1.
// Kind is the document kind (ACT, DELIVERY_NOTE, an application's own); Amount
// is the amount the signed document recognises, in minor units.
type AcceptanceSignedPayload struct {
	AcceptanceID uuid.UUID  `json:"acceptanceId"`
	ProjectID    uuid.UUID  `json:"projectId"`
	Kind         string     `json:"kind"`
	Number       string     `json:"number"`
	Amount       money.Wire `json:"amount"`
	SignedOn     string     `json:"signedOn" wire:"date"`
}

// AcceptanceSignedEvent is published in the transaction that signs the
// document: by AcceptanceService.Sign and by an Update whose signing day signs
// a draft.
var AcceptanceSignedEvent = native.Event[AcceptanceSignedPayload](AcceptanceSignedEventID, native.EventSpec{
	Subject: AcceptanceResource,
	Label: manifest.Label{
		"en": "Acceptance document signed", "ru": "Акт подписан",
		"uz": "Qabul hujjati imzolandi", "uz-Cyrl": "Қабул ҳужжати имзоланди",
	},
	Description: "An acceptance document of a project was signed. Published in the transaction that signs the document.",
	Descriptions: manifest.Label{
		"en":      "An acceptance document of a project was signed.",
		"ru":      "Подписан акт приёмки по проекту.",
		"uz":      "Loyihaning qabul hujjati imzolandi.",
		"uz-Cyrl": "Лойиҳанинг қабул ҳужжати имзоланди.",
	},
	ReadPermission: permissions.ProjectRead.Name(),
	Category:       "projects.acceptance",
})

// Events are the projects events the module contributes to the application's
// native set (projects.Natives).
func Events() []native.Item { return []native.Item{AcceptanceSignedEvent} }

// publishAcceptanceSigned enqueues projects.acceptance.signed.v1 in the
// transaction held by ctx when the change from before to after signed the
// document; every other change publishes nothing.
func publishAcceptanceSigned(ctx context.Context, before, after acceptance.Document) error {
	const op = serrors.Op("AcceptanceService.publishAcceptanceSigned")
	if before.Status() == acceptance.StatusSigned || after.Status() != acceptance.StatusSigned {
		return nil
	}
	amount, err := money.ToWire(after.Amount())
	if err != nil {
		return serrors.Wrap(op, err)
	}
	signedOn := after.SignedOn()
	if signedOn.IsZero() {
		signedOn = after.Date()
	}
	return serrors.Wrap(op, events.PublishInTx(ctx, after.TenantID(), AcceptanceSignedEvent,
		&spi.ObjectRef{Entity: AcceptanceResource, Key: after.ID().String()},
		AcceptanceSignedPayload{
			AcceptanceID: after.ID(), ProjectID: after.ProjectID(), Kind: string(after.Kind()), Number: after.Number(),
			Amount: amount, SignedOn: signedOn.Format(time.DateOnly),
		}))
}
