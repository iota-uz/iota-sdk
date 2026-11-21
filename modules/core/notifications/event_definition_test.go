package notifications_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/notification"
	"github.com/iota-uz/iota-sdk/modules/core/notifications"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/manifest"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/native"
)

type invoicePaid struct {
	InvoiceID int64  `json:"invoiceId,string" wire:"int64"`
	Number    string `json:"number"`
	Paid      bool   `json:"paid"`
	Lines     []struct {
		Amount int `json:"amount"`
	} `json:"lines"`
	Note *string `json:"note,omitempty"`
}

var invoicePaidEvent = native.Event[invoicePaid]("billing.invoice.paid.v1", native.EventSpec{
	Subject:      "billing.invoice",
	Label:        manifest.Label{"en": "Invoice paid", "ru": "Счёт оплачен"},
	Description:  "An invoice was paid.",
	Descriptions: manifest.Label{"en": "An invoice was paid.", "ru": "Счёт оплачен полностью."},
})

func TestFromEvent_TheEventSuppliesKeyTextsAndPayloadContract(t *testing.T) {
	c := notifications.NewCatalog()
	require.Error(t, c.Register(notifications.FromEvent(invoicePaidEvent,
		notifications.WithRecipients(notifications.RecipientDefinition{Key: "actor", Name: map[string]string{"en": "Actor"}}),
	)), "a recipient without a resolver is still rejected")

	require.NoError(t, c.Register(notifications.FromEvent(invoicePaidEvent, notifications.WithModule("billing"), notifications.WithLevel(notification.LevelSuccess))))
	d, ok := c.Get("billing.invoice.paid.v1")
	require.True(t, ok)
	require.Same(t, invoicePaidEvent, d.Event.(*native.EventDef[invoicePaid]))
	require.Equal(t, map[string]string{"en": "Invoice paid", "ru": "Счёт оплачен"}, d.Name)
	require.Equal(t, map[string]string{"en": "An invoice was paid.", "ru": "Счёт оплачен полностью."}, d.Description)
	require.Equal(t, notification.LevelSuccess, d.DefaultLevel)
	keys := []string{}
	for _, f := range d.PayloadFields {
		keys = append(keys, f.Key)
	}
	require.Equal(t, []string{"invoice_id", "lines", "note", "number", "paid"}, keys)

	tenant := uuid.New()
	event := notifications.Event{Key: d.Key, ID: uuid.NewString(), TenantID: tenant,
		Payload: json.RawMessage(`{"invoiceId":"7","number":"I-7","paid":true,"lines":[{"amount":3}]}`)}
	normalized, err := c.Normalize(event)
	require.NoError(t, err)
	require.Equal(t, map[string]string{"invoice_id": "7", "number": "I-7", "paid": "true", "lines": `[{"amount":3}]`}, normalized.Data,
		"without WithData the payload is projected flat, keys in snake_case")
	require.Equal(t, 1, normalized.Version)

	event.Payload = json.RawMessage(`{"invoiceId":7}`)
	_, err = c.Normalize(event)
	require.ErrorContains(t, err, "payload of billing.invoice.paid.v1", "the payload is checked against the event contract")
	event.Payload = nil
	_, err = c.Normalize(event)
	require.Error(t, err, "an event-backed notification needs its payload")
}

func TestFromEvent_OptionsOverrideTextsAndProjection(t *testing.T) {
	c := notifications.NewCatalog()
	d := notifications.FromEvent(invoicePaidEvent,
		notifications.WithName(map[string]string{"en": "Paid"}),
		notifications.WithDescription(map[string]string{"en": "Tell accounting."}),
		notifications.WithData(func(payload json.RawMessage) (map[string]string, error) {
			var p invoicePaid
			if err := json.Unmarshal(payload, &p); err != nil {
				return nil, err
			}
			return map[string]string{"invoice": p.Number}, nil
		}),
		notifications.WithPayloadFields(notifications.PayloadField{Key: "invoice", Required: true}),
		notifications.WithRecipients(notifications.RecipientDefinition{Key: "accounting", Name: map[string]string{"en": "Accounting"}, Resolve: func(_ context.Context, _ notifications.Event) ([]uint, error) { return []uint{1}, nil }}),
		notifications.WithDefaultRecipients("accounting"),
	)
	require.NoError(t, c.Register(d))
	got, _ := c.Get(invoicePaidEvent.ID())
	require.Equal(t, map[string]string{"en": "Paid"}, got.Name)
	require.Equal(t, map[string]string{"en": "Tell accounting."}, got.Description)
	require.Equal(t, []string{"accounting"}, got.DefaultRecipientKeys)
	data, err := got.DataOf(json.RawMessage(`{"invoiceId":"7","number":"I-7","paid":true,"lines":[]}`))
	require.NoError(t, err)
	require.Equal(t, map[string]string{"invoice": "I-7"}, data)
}

func TestFromEvent_AKeyOtherThanTheEventIsRejected(t *testing.T) {
	d := notifications.FromEvent(invoicePaidEvent)
	d.Key = "billing.invoice.settled.v1"
	require.ErrorContains(t, notifications.NewCatalog().Register(d), "differs from its event")
}

func TestFlatData(t *testing.T) {
	data, err := notifications.FlatData(json.RawMessage(`{"userId":"5","firstName":"Ann","age":3,"meta":{"a": 1},"gone":null}`))
	require.NoError(t, err)
	require.Equal(t, map[string]string{"user_id": "5", "first_name": "Ann", "age": "3", "meta": `{"a":1}`}, data)
	_, err = notifications.FlatData(json.RawMessage(`[1]`))
	require.Error(t, err)
}

// The payload is the source of Data: stale or empty Data that arrives with a
// payload is replaced by the projection.
func TestFromEvent_DataIsAlwaysProjectedFromThePayload(t *testing.T) {
	c := notifications.NewCatalog()
	require.NoError(t, c.Register(notifications.FromEvent(invoicePaidEvent,
		notifications.WithPayloadFields(notifications.PayloadField{Key: "number", Required: true}),
		notifications.WithData(func(payload json.RawMessage) (map[string]string, error) {
			var p invoicePaid
			if err := json.Unmarshal(payload, &p); err != nil {
				return nil, err
			}
			return map[string]string{"number": p.Number}, nil
		}),
	)))
	payload := json.RawMessage(`{"invoiceId":"7","number":"I-7","paid":true,"lines":[]}`)
	for name, data := range map[string]map[string]string{
		"stale": {"number": "I-999", "extra": "x"},
		"empty": {},
	} {
		t.Run(name, func(t *testing.T) {
			got, err := c.Normalize(notifications.Event{Key: invoicePaidEvent.ID(), ID: uuid.NewString(), TenantID: uuid.New(), Payload: payload, Data: data})
			require.NoError(t, err)
			require.Equal(t, map[string]string{"number": "I-7"}, got.Data)
		})
	}
}

// The default projection reads an object, so an event with another payload
// shape registers only with its own projection.
func TestFromEvent_ANonObjectPayloadNeedsWithData(t *testing.T) {
	tagged := native.Event[[]string]("billing.invoice.tagged.v1", native.EventSpec{Label: manifest.Label{"en": "Invoice tagged"}})
	require.ErrorContains(t, notifications.NewCatalog().Register(notifications.FromEvent(tagged)), "needs WithData")
	require.NoError(t, notifications.NewCatalog().Register(notifications.FromEvent(tagged,
		notifications.WithData(func(payload json.RawMessage) (map[string]string, error) {
			var tags []string
			if err := json.Unmarshal(payload, &tags); err != nil {
				return nil, err
			}
			return map[string]string{"tags": strings.Join(tags, ",")}, nil
		}))))
}
