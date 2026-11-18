package services

import (
	"context"
	"reflect"
	"slices"

	"github.com/iota-uz/iota-sdk/modules/crm/domain/aggregates/client"
	"github.com/iota-uz/iota-sdk/modules/logging/history"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/events"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/native"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/spi"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

// Versioned IDs of the CRM client events. Extension packages subscribe to
// them by these IDs (the kit names them crm.Created / crm.Updated).
const (
	ClientCreatedEventID = "crm.client.created.v1"
	ClientUpdatedEventID = "crm.client.updated.v1"
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

// ClientCreatedEvent is published by UpsertClient in the transaction that
// creates the client.
var ClientCreatedEvent = native.Event[ClientCreatedPayload](ClientCreatedEventID, native.EventSpec{
	Subject:     ClientEntity,
	Label:       map[string]string{"en": "Client created", "ru": "Клиент создан", "uz": "Mijoz yaratildi"},
	Description: "A CRM client was created. Published in the transaction that creates the client.",
})

// ClientUpdatedEvent is published by UpsertClient in the transaction that
// updates the client.
var ClientUpdatedEvent = native.Event[ClientUpdatedPayload](ClientUpdatedEventID, native.EventSpec{
	Subject:     ClientEntity,
	Label:       map[string]string{"en": "Client updated", "ru": "Клиент изменён", "uz": "Mijoz o'zgartirildi"},
	Description: "A CRM client was updated. Published in the transaction that updates the client; changed lists the crm.client fields the update changed.",
})

// entityFields maps the history snapshot keys to the crm.client field IDs
// that a changed list may name (personal identification is not exposed).
var entityFields = []struct{ snapshot, field string }{
	{"first_name", "firstName"}, {"last_name", "lastName"}, {"middle_name", "middleName"},
	{"phone", "phone"}, {"address", "address"}, {"email", "email"},
	{"date_of_birth", "dateOfBirth"}, {"gender", "gender"}, {"comments", "comments"},
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

// publishClientEvent enqueues crm.client.created.v1 (before == nil) or
// crm.client.updated.v1 in tx, so the event commits or rolls back with the
// client. The initiator is the user the invocation acts as (tx.Subject: the
// initiating user, or the owner of a subscription, schedule or endpoint), the
// same subject tx.Authorize checks, not the HTTP user of ctx, which a
// background root does not have.
func publishClientEvent(ctx context.Context, tx native.Tx, before, saved client.Client) error {
	tenant, err := composables.UseTenantID(ctx)
	if err != nil {
		return err
	}
	acting := tx.Subject()
	scope := spi.Scope{Subject: spi.Subject{TenantID: tenant, UserID: acting.UserID, Kind: acting.Kind}}
	ref := ClientRef(saved.ID())
	phone, email := contactOf(saved)
	if before == nil {
		_, err = events.Publish(ctx, tx.DB(), scope, ClientCreatedEvent, &ref, ClientCreatedPayload{
			ClientID: int64(saved.ID()), FirstName: saved.FirstName(),
			LastName: optional(saved.LastName()), MiddleName: optional(saved.MiddleName()), Phone: phone, Email: email,
		})
	} else {
		_, err = events.Publish(ctx, tx.DB(), scope, ClientUpdatedEvent, &ref, ClientUpdatedPayload{
			ClientID: int64(saved.ID()), FirstName: saved.FirstName(),
			LastName: optional(saved.LastName()), MiddleName: optional(saved.MiddleName()), Phone: phone, Email: email,
			Changed: changedFields(clientSnapshot(before), clientSnapshot(saved)),
		})
	}
	return serrors.Wrap("crm.publishClientEvent", err)
}
