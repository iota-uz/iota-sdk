package notifications

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"strings"
	"unicode"

	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/notification"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/permission"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/manifest"
)

// EventRef is the business event a definition delivers. *native.EventDef[P]
// satisfies it without an adapter: the event is the fact, the definition is
// its projection for delivery.
type EventRef interface {
	ID() string
	Resource() manifest.Resource
}

// localizedDescriptions is the optional part of an EventRef that carries the
// localized description (native.EventMetadata).
type localizedDescriptions interface {
	Descriptions() manifest.Label
}

// DefinitionOption configures a definition built by FromEvent.
type DefinitionOption func(*Definition)

// FromEvent builds the notification definition of a business event. The
// event supplies the key, the name and description (its Label and
// Descriptions), the payload fields and the payload check against its
// contract; the definition owns only recipients, guard, render and level.
func FromEvent(ev EventRef, opts ...DefinitionOption) Definition {
	d := Definition{Event: ev}
	if ev != nil {
		d.Key = ev.ID()
	}
	for _, opt := range opts {
		if opt != nil {
			opt(&d)
		}
	}
	return d
}

// WithRecipients adds the semantic recipients rules may select.
func WithRecipients(keys ...RecipientDefinition) DefinitionOption {
	return func(d *Definition) { d.RecipientKeys = append(d.RecipientKeys, keys...) }
}

// WithDefaultRecipients names the semantic recipients the notification is
// delivered to while the tenant has no rule for the event: the listed
// recipients, the default level and the in-app channel plus personal
// preferences. Any rule an administrator saved, including a disabled one,
// replaces this default entirely.
func WithDefaultRecipients(keys ...string) DefinitionOption {
	return func(d *Definition) { d.DefaultRecipientKeys = append(d.DefaultRecipientKeys, keys...) }
}

// WithRender sets how a notice of the event is rendered for a language.
func WithRender(fn func(Event, string) (Content, error)) DefinitionOption {
	return func(d *Definition) { d.Render = fn }
}

// WithGuard sets the per-recipient check applied before delivery.
func WithGuard(fn func(context.Context, Event, user.User) (bool, error)) DefinitionOption {
	return func(d *Definition) { d.RecipientGuard = fn }
}

// WithLevel sets the default notification level.
func WithLevel(l notification.Level) DefinitionOption {
	return func(d *Definition) { d.DefaultLevel = l }
}

// WithRequiredPermission sets the permission every recipient must hold.
func WithRequiredPermission(p permission.Permission) DefinitionOption {
	return func(d *Definition) { d.RequiredPermission = p }
}

// WithModule sets the module the definition is listed under.
func WithModule(name string) DefinitionOption {
	return func(d *Definition) { d.Module = name }
}

// WithName overrides the name taken from the event Label.
func WithName(name map[string]string) DefinitionOption {
	return func(d *Definition) { d.Name = maps.Clone(name) }
}

// WithDescription overrides the description taken from the event.
func WithDescription(description map[string]string) DefinitionOption {
	return func(d *Definition) { d.Description = maps.Clone(description) }
}

// WithPayloadFields declares the Data fields renderers and rules read, in
// place of the fields derived from the event payload. Use it with WithData
// when the projection names other keys.
func WithPayloadFields(fields ...PayloadField) DefinitionOption {
	return func(d *Definition) { d.PayloadFields = append(d.PayloadFields, fields...) }
}

// WithData sets the projection of the typed payload into Data for renderers
// and rules. Without it the projection is flat over the top level: camelCase
// keys become snake_case, values become strings and nested objects and
// arrays JSON; an event whose payload is not an object needs WithData. The
// projection always replaces the Data an event arrives with: the payload is
// the source of truth.
func WithData(fn func(payload json.RawMessage) (map[string]string, error)) DefinitionOption {
	return func(d *Definition) { d.Data = fn }
}

// WithLegacyEvents accepts, in persisted dispatch jobs only
// (Catalog.NormalizeStored), events that match and carry no Payload without
// the payload contract check: the jobs a producer queued before its definition
// was built from the business event. They are still checked by the payload
// fields, ValidatePayload and Render. A new submission (Catalog.Normalize)
// always needs the payload.
func WithLegacyEvents(match func(Event) bool) DefinitionOption {
	return func(d *Definition) { d.LegacyEvent = match }
}

// WithDedupeKey sets the delivery identity of the events the outbox bridge
// enqueues (Event.DedupeKey). Use it when another producer enqueues the same
// notification under its own identity, for example the producer the event
// replaced, so whichever enqueues second is a no-op.
func WithDedupeKey(fn func(Event) string) DefinitionOption {
	return func(d *Definition) { d.DedupeKey = fn }
}

// checkPayload checks the payload of an event of an event-backed definition
// against the event contract. stored admits the legacy events of a persisted
// job (WithLegacyEvents).
func (d Definition) checkPayload(event Event, stored bool) error {
	if d.contract == nil {
		return nil
	}
	if len(bytes.TrimSpace(event.Payload)) == 0 {
		if stored && d.LegacyEvent != nil && d.LegacyEvent(event) {
			return nil
		}
		return fmt.Errorf("notification %s needs the payload of its event", d.Key)
	}
	if err := manifest.ValidateValue(*d.contract, event.Payload); err != nil {
		return fmt.Errorf("payload of %s: %w", d.Key, err)
	}
	return nil
}

// DataOf projects a typed payload into the flat Data of an event of the
// definition: Definition.Data, or FlatData without one.
func (d Definition) DataOf(payload json.RawMessage) (map[string]string, error) {
	if d.Data != nil {
		return d.Data(payload)
	}
	return FlatData(payload)
}

// FlatData is the default payload projection: each top-level key in
// snake_case, strings as they are, numbers and booleans as their JSON text,
// objects and arrays as compact JSON; null values are left out.
func FlatData(payload json.RawMessage) (map[string]string, error) {
	out := map[string]string{}
	if len(bytes.TrimSpace(payload)) == 0 || string(bytes.TrimSpace(payload)) == "null" {
		return out, nil
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(payload, &fields); err != nil {
		return nil, fmt.Errorf("notification payload must be a JSON object: %w", err)
	}
	for key, raw := range fields {
		raw = bytes.TrimSpace(raw)
		switch {
		case string(raw) == "null":
			continue
		case len(raw) > 0 && raw[0] == '"':
			var s string
			if err := json.Unmarshal(raw, &s); err != nil {
				return nil, err
			}
			out[SnakeCase(key)] = s
		case len(raw) > 0 && (raw[0] == '{' || raw[0] == '['):
			var buf bytes.Buffer
			if err := json.Compact(&buf, raw); err != nil {
				return nil, err
			}
			out[SnakeCase(key)] = buf.String()
		default:
			out[SnakeCase(key)] = string(raw)
		}
	}
	return out, nil
}

// SnakeCase converts a camelCase payload key to snake_case ("userId" →
// "user_id").
func SnakeCase(key string) string {
	var b strings.Builder
	for i, r := range key {
		if unicode.IsUpper(r) {
			if i > 0 {
				b.WriteByte('_')
			}
			r = unicode.ToLower(r)
		}
		b.WriteRune(r)
	}
	return b.String()
}

// prepareEvent fills what the event of an event-backed definition supplies.
func prepareEvent(d Definition) (Definition, error) {
	id := d.Event.ID()
	if d.Key != "" && d.Key != id {
		return Definition{}, fmt.Errorf("notification definition key %s differs from its event %s", d.Key, id)
	}
	d.Key = id
	res := d.Event.Resource()
	if res.Kind != manifest.ResEvent || res.Payload == nil {
		return Definition{}, fmt.Errorf("notification definition %s: the event has no payload contract", id)
	}
	if len(d.Name) == 0 {
		d.Name = maps.Clone(map[string]string(res.Label))
	}
	if len(d.Description) == 0 {
		if ld, ok := d.Event.(localizedDescriptions); ok {
			d.Description = maps.Clone(map[string]string(ld.Descriptions()))
		}
		if len(d.Description) == 0 && res.Description != "" {
			d.Description = map[string]string{"en": res.Description}
		}
	}
	if d.Data == nil && res.Payload.Kind != manifest.KindObject {
		return Definition{}, fmt.Errorf("notification definition %s: a %s payload needs WithData (the default projection reads an object)", id, res.Payload.Kind)
	}
	if len(d.PayloadFields) == 0 && d.Data == nil {
		for _, p := range res.Payload.Properties {
			d.PayloadFields = append(d.PayloadFields, PayloadField{Key: SnakeCase(p.Name)})
		}
		slices.SortFunc(d.PayloadFields, func(a, b PayloadField) int { return strings.Compare(a.Key, b.Key) })
	}
	contract := *res.Payload
	d.contract = &contract
	return d, nil
}
