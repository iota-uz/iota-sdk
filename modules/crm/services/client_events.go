package services

import (
	"context"
	"reflect"
	"slices"

	"github.com/google/uuid"

	"github.com/iota-uz/iota-sdk/modules/crm/domain/aggregates/client"
	"github.com/iota-uz/iota-sdk/modules/crm/permissions"
	"github.com/iota-uz/iota-sdk/modules/logging/history"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/events"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/manifest"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/native"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/spi"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

// Versioned IDs of the CRM client events. Extension packages subscribe to
// them by these IDs (the kit names them crm.Created / crm.Updated).
const (
	ClientCreatedEventID = "crm.client.created.v1"
	ClientUpdatedEventID = "crm.client.updated.v1"
	ClientDeletedEventID = "crm.client.deleted.v1"
)

// ClientCreatedPayload is the payload of crm.client.created.v1: the fields the
// crm.client entity publishes (personal identification never travels).
type ClientCreatedPayload struct {
	ClientID   int64   `json:"clientId,string" wire:"int64"`
	FirstName  string  `json:"firstName"`
	LastName   *string `json:"lastName,omitempty"`
	MiddleName *string `json:"middleName,omitempty"`
	Phone      *string `json:"phone,omitempty"`
	Email      *string `json:"email,omitempty"`
}

// ClientUpdatedPayload is the payload of crm.client.updated.v1: the client
// after the update and the IDs of the crm.client fields the update changed.
type ClientUpdatedPayload struct {
	ClientID   int64    `json:"clientId,string" wire:"int64"`
	FirstName  string   `json:"firstName"`
	LastName   *string  `json:"lastName,omitempty"`
	MiddleName *string  `json:"middleName,omitempty"`
	Phone      *string  `json:"phone,omitempty"`
	Email      *string  `json:"email,omitempty"`
	Changed    []string `json:"changed"`
}

// ClientDeletedPayload is the payload of crm.client.deleted.v1.
type ClientDeletedPayload struct {
	ClientID  int64  `json:"clientId,string" wire:"int64"`
	FirstName string `json:"firstName"`
}

// ClientCreatedEvent is published by UpsertClient in the transaction that
// creates the client.
var ClientCreatedEvent = native.Event[ClientCreatedPayload](ClientCreatedEventID, native.EventSpec{
	Subject:     ClientEntity,
	Label:       manifest.Label{"en": "Client created", "ru": "Клиент создан", "uz": "Mijoz yaratildi", "uz-Cyrl": "Мижоз яратилди"},
	Description: "A CRM client was created. Published in the transaction that creates the client.",
	Descriptions: manifest.Label{
		"en":      "A CRM client was created.",
		"ru":      "Создан клиент CRM.",
		"uz":      "CRM mijozi yaratildi.",
		"uz-Cyrl": "CRM мижози яратилди.",
	},
	ReadPermission: permissions.ClientRead.Name(),
	Sensitive:      clientContactFields,
	Category:       "crm.clients",
})

// ClientUpdatedEvent is published by UpsertClient in the transaction that
// updates the client.
var ClientUpdatedEvent = native.Event[ClientUpdatedPayload](ClientUpdatedEventID, native.EventSpec{
	Subject:     ClientEntity,
	Label:       manifest.Label{"en": "Client updated", "ru": "Клиент изменён", "uz": "Mijoz o'zgartirildi", "uz-Cyrl": "Мижоз ўзгартирилди"},
	Description: "A CRM client was updated. Published in the transaction that updates the client; changed lists the crm.client fields the update changed.",
	Descriptions: manifest.Label{
		"en":      "A CRM client was updated; the changed fields are listed.",
		"ru":      "Клиент CRM изменён; перечислены изменённые поля.",
		"uz":      "CRM mijozi o'zgartirildi; o'zgargan maydonlar ko'rsatiladi.",
		"uz-Cyrl": "CRM мижози ўзгартирилди; ўзгарган майдонлар кўрсатилади.",
	},
	ReadPermission: permissions.ClientRead.Name(),
	Sensitive:      clientContactFields,
	Category:       "crm.clients",
})

// ClientDeletedEvent is published by ClientService.Delete in the transaction
// that deletes the client.
var ClientDeletedEvent = native.Event[ClientDeletedPayload](ClientDeletedEventID, native.EventSpec{
	Subject:     ClientEntity,
	Label:       localized("Client deleted", "Клиент удалён", "Mijoz o'chirildi", "Мижоз ўчирилди"),
	Description: "A CRM client was deleted. Published in the transaction that deletes the client.",
	Descriptions: localized(
		"A CRM client was deleted.",
		"Клиент CRM удалён.",
		"CRM mijozi o'chirildi.",
		"CRM мижози ўчирилди."),
	ReadPermission: permissions.ClientRead.Name(),
	Category:       "crm.clients",
})

// localized builds the label of an event in the locales the SDK checks.
func localized(en, ru, uz, uzCyrl string) manifest.Label {
	return manifest.Label{"en": en, "ru": ru, "uz": uz, "uz-Cyrl": uzCyrl}
}

// clientContactFields are the payload fields of the client events that only a
// user who may read clients sees.
var clientContactFields = map[string][]string{
	"phone": {permissions.ClientRead.Name()},
	"email": {permissions.ClientRead.Name()},
}

// entityFields maps the history snapshot keys to the crm.client field IDs
// that a changed list may name. The passport and the PIN are named when they
// change, so an edit of only one of them still publishes its event; their
// values never travel.
var entityFields = []struct{ snapshot, field string }{
	{"first_name", "firstName"}, {"last_name", "lastName"}, {"middle_name", "middleName"},
	{"phone", "phone"}, {"address", "address"}, {"email", "email"},
	{"date_of_birth", "dateOfBirth"}, {"gender", "gender"}, {"comments", "comments"},
	{"passport", "passport"}, {"pin", "pin"},
}

func optional(v string) *string {
	if v == "" {
		return nil
	}
	return &v
}

func contactOf(c client.Client) (*string, *string) {
	var phone, email *string
	if c.Phone() != nil {
		phone = optional(c.Phone().Value())
	}
	if c.Email() != nil {
		email = optional(c.Email().Value())
	}
	return phone, email
}

// changedFields lists the crm.client fields whose values differ.
func changedFields(before, after history.Snapshot) []string {
	out := []string{}
	for _, f := range entityFields {
		if !reflect.DeepEqual(before[f.snapshot], after[f.snapshot]) {
			out = append(out, f.field)
		}
	}
	slices.Sort(out)
	return out
}

func clientCreatedPayload(saved client.Client) ClientCreatedPayload {
	phone, email := contactOf(saved)
	return ClientCreatedPayload{
		ClientID: int64(saved.ID()), FirstName: saved.FirstName(),
		LastName: optional(saved.LastName()), MiddleName: optional(saved.MiddleName()), Phone: phone, Email: email,
	}
}

func clientUpdatedPayload(before, saved client.Client) ClientUpdatedPayload {
	phone, email := contactOf(saved)
	return ClientUpdatedPayload{
		ClientID: int64(saved.ID()), FirstName: saved.FirstName(),
		LastName: optional(saved.LastName()), MiddleName: optional(saved.MiddleName()), Phone: phone, Email: email,
		Changed: changedFields(clientSnapshot(before), clientSnapshot(saved)),
	}
}

// publishClientEvent enqueues crm.client.created.v1 (before == nil) or
// crm.client.updated.v1 in tx, so the event commits or rolls back with the
// client. An update that changed nothing publishes nothing.
//
// The invocation subject (tx.Subject) authorizes the write; it is not the
// event's attribution. The initiator is the user who acted: the initiating
// user of a request. A job context (history.IsJob: a subscription, a schedule,
// an endpoint) has none, even when it is authorized as an event initiator, and
// neither has the owner a job runs as.
func publishClientEvent(ctx context.Context, tx native.Tx, before, saved client.Client) error {
	tenant, err := composables.UseTenantID(ctx)
	if err != nil {
		return err
	}
	acting := tx.Subject()
	scope := spi.Scope{Subject: spi.Subject{TenantID: tenant, Kind: acting.Kind}}
	if acting.Kind == spi.SubjectInitiator && !history.IsJob(ctx) && !foreignUser(ctx, tenant) {
		scope.Subject.UserID = acting.UserID
	}
	ref := ClientRef(saved.ID())
	if before == nil {
		_, err = events.Publish(ctx, tx.DB(), scope, ClientCreatedEvent, &ref, clientCreatedPayload(saved))
		return serrors.Wrap("crm.publishClientEvent", err)
	}
	payload := clientUpdatedPayload(before, saved)
	if len(payload.Changed) == 0 {
		return nil
	}
	_, err = events.Publish(ctx, tx.DB(), scope, ClientUpdatedEvent, &ref, payload)
	return serrors.Wrap("crm.publishClientEvent", err)
}

// foreignUser reports whether the user of ctx belongs to another tenant (a
// superadmin acting in this one), who is no initiator of an event of tenant.
func foreignUser(ctx context.Context, tenant uuid.UUID) bool {
	u, err := composables.UseUser(ctx)
	return err == nil && u != nil && u.TenantID() != tenant
}

// publishClientChange is publishClientEvent for the services' own
// transactions (composables.InTx): the event belongs to the client's tenant
// and the acting user of ctx, when there is one, is its initiator.
func publishClientChange(ctx context.Context, before, saved client.Client) error {
	const op = serrors.Op("crm.publishClientChange")
	ref := ClientRef(saved.ID())
	if before == nil {
		return serrors.Wrap(op, events.PublishInTx(ctx, saved.TenantID(), ClientCreatedEvent, &ref, clientCreatedPayload(saved)))
	}
	payload := clientUpdatedPayload(before, saved)
	if len(payload.Changed) == 0 {
		return nil
	}
	return serrors.Wrap(op, events.PublishInTx(ctx, saved.TenantID(), ClientUpdatedEvent, &ref, payload))
}

// publishClientDeleted enqueues crm.client.deleted.v1.
func publishClientDeleted(ctx context.Context, deleted client.Client) error {
	ref := ClientRef(deleted.ID())
	return serrors.Wrap("crm.publishClientDeleted", events.PublishInTx(ctx, deleted.TenantID(), ClientDeletedEvent, &ref, ClientDeletedPayload{
		ClientID: int64(deleted.ID()), FirstName: deleted.FirstName(),
	}))
}
