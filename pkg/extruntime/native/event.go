package native

import (
	"encoding/json"
	"fmt"
	"maps"
	"regexp"
	"slices"
	"strings"

	"github.com/iota-uz/iota-sdk/pkg/extruntime/manifest"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/spi"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

// EventKeyPattern is the one grammar of event keys, shared by native events
// and notification definitions: snake_case dotted segments ending in a
// version from .v1 ("core.user.created.v1").
const EventKeyPattern = `^[a-z][a-z0-9_]*(\.[a-z][a-z0-9_]*)+\.v[1-9][0-9]*$`

var versionedEventID = regexp.MustCompile(EventKeyPattern)

// ValidEventKey reports whether key follows EventKeyPattern.
func ValidEventKey(key string) bool { return versionedEventID.MatchString(key) }

// EventSpec is the public part of a native event besides its ID and payload
// type. Label, Description, Descriptions, ReadPermission, Sensitive and
// Category are presentation and access metadata: they are not part of the
// contract hash, so changing a text never changes the contract.
type EventSpec struct {
	// Subject is the entity resource ID the event is about ("crm.client");
	// empty for an event without a subject.
	Subject string
	// Label is the localized name of the event. The locales an application
	// requires are checked by its own coverage test, not by NewEvent (the SDK
	// checks en, ru, uz and uz-Cyrl for its events).
	Label manifest.Label
	// Description is the English text published in the manifest; when empty
	// it is Descriptions["en"].
	Description string
	// Descriptions is the localized description.
	Descriptions manifest.Label
	// Payload overrides the type derived from the Go payload struct (TypeOf).
	Payload *manifest.Type
	// ReadPermission is the RBAC permission name (permission.Name(), "User.Read")
	// a user needs to see the event in the palette and its payload in logs.
	// Empty means the read permission of the subject entity.
	ReadPermission string
	// Sensitive maps a dotted payload field path ("email", "contact.phone")
	// to the permissions without which the field is hidden (PII). Every path
	// must exist in the payload type.
	Sensitive map[string][]string
	// Category groups the event in the palette ("core.users", "finance.debts").
	Category string
}

// EventDefinition is the untyped view of a native event.
type EventDefinition interface {
	ID() string
	// Resource is the event contract published in the catalog and the kit.
	Resource() manifest.Resource
	// Intent validates a JSON payload and subject against the contract and
	// returns the outbox intent.
	Intent(subject *spi.ObjectRef, payload json.RawMessage) (spi.EventIntent, error)
}

// EventMetadata is the presentation and access metadata of a native event,
// which *EventDef implements. Catalogs read it from an EventDefinition with a
// type assertion.
type EventMetadata interface {
	Label() manifest.Label
	Descriptions() manifest.Label
	ReadPermission() string
	Sensitive() map[string][]string
	Category() string
}

var _ EventMetadata = (*EventDef[struct{}])(nil)

// EventDef is a native business event with the versioned payload type P. It
// is declared once, as a package-level value, contributed to the Set by its
// module and published by Go code with events.Publish in the transaction of
// the change it reports, so it commits or rolls back with that change.
type EventDef[P any] struct {
	id   string
	res  manifest.Resource
	spec EventSpec
}

// Event declares a native event. The ID ends in a version (".v1"): a payload
// change is a new event ID. It panics on an invalid ID or a payload type
// TypeOf cannot describe; NewEvent returns the error.
func Event[P any](id string, spec EventSpec) *EventDef[P] {
	e, err := NewEvent[P](id, spec)
	if err != nil {
		panic(err)
	}
	return e
}

// NewEvent is Event returning the error.
func NewEvent[P any](id string, spec EventSpec) (*EventDef[P], error) {
	const op = serrors.Op("extruntime.native.NewEvent")
	if !versionedEventID.MatchString(id) {
		return nil, serrors.New(serrors.Invalid, fmt.Sprintf("native event %q must be a dotted snake_case ID ending in a version from .v1 (%s)", id, EventKeyPattern)).WithOp(op)
	}
	payload := spec.Payload
	if payload == nil {
		t, err := TypeOf[P]()
		if err != nil {
			return nil, serrors.Wrap(op, fmt.Errorf("%s payload: %w", id, err))
		}
		payload = &t
	}
	for _, path := range slices.Sorted(maps.Keys(spec.Sensitive)) {
		if !hasPath(*payload, path) {
			return nil, serrors.New(serrors.Invalid, fmt.Sprintf("native event %s: sensitive field %q is not in the payload", id, path)).WithOp(op)
		}
		if len(spec.Sensitive[path]) == 0 {
			return nil, serrors.New(serrors.Invalid, fmt.Sprintf("native event %s: sensitive field %q names no permission", id, path)).WithOp(op)
		}
	}
	description := spec.Description
	if description == "" {
		description = spec.Descriptions["en"]
	}
	p := *payload
	res := manifest.Resource{ID: id, Kind: manifest.ResEvent, Subject: spec.Subject, Payload: &p, Label: maps.Clone(spec.Label), Description: description}
	res.ContractHash = manifest.ContractHashOf(res)
	spec.Payload = nil
	spec.Label = maps.Clone(spec.Label)
	spec.Descriptions = maps.Clone(spec.Descriptions)
	spec.Sensitive = cloneSensitive(spec.Sensitive)
	return &EventDef[P]{id: id, res: res, spec: spec}, nil
}

// hasPath reports whether the dotted path names a field of t; an array is
// walked through its items.
func hasPath(t manifest.Type, path string) bool {
	for _, name := range strings.Split(path, ".") {
		for t.Kind == manifest.KindArray && t.Items != nil {
			t = *t.Items
		}
		i := slices.IndexFunc(t.Properties, func(p manifest.Property) bool { return p.Name == name })
		if t.Kind != manifest.KindObject || i < 0 {
			return false
		}
		t = t.Properties[i].Type
	}
	return true
}

func cloneSensitive(in map[string][]string) map[string][]string {
	if in == nil {
		return nil
	}
	out := make(map[string][]string, len(in))
	for k, v := range in {
		out[k] = slices.Clone(v)
	}
	return out
}

func (e *EventDef[P]) ID() string                  { return e.id }
func (e *EventDef[P]) Resource() manifest.Resource { return cloneResource(e.res) }

// Label is the localized name of the event.
func (e *EventDef[P]) Label() manifest.Label { return maps.Clone(e.spec.Label) }

// Descriptions is the localized description of the event.
func (e *EventDef[P]) Descriptions() manifest.Label { return maps.Clone(e.spec.Descriptions) }

// ReadPermission is the permission that reveals the event and its payload;
// empty means the read permission of the subject entity.
func (e *EventDef[P]) ReadPermission() string { return e.spec.ReadPermission }

// Sensitive maps payload field paths to the permissions that reveal them.
func (e *EventDef[P]) Sensitive() map[string][]string { return cloneSensitive(e.spec.Sensitive) }

// Category is the palette group of the event.
func (e *EventDef[P]) Category() string { return e.spec.Category }

// Intent implements EventDefinition: the payload must match the declared
// type and the subject the declared subject entity.
func (e *EventDef[P]) Intent(subject *spi.ObjectRef, payload json.RawMessage) (spi.EventIntent, error) {
	if len(payload) == 0 {
		payload = json.RawMessage(`null`)
	}
	if err := manifest.ValidateValue(*e.res.Payload, payload); err != nil {
		return spi.EventIntent{}, fmt.Errorf("payload of %s: %w", e.id, err)
	}
	switch {
	case e.res.Subject != "" && subject == nil:
		return spi.EventIntent{}, fmt.Errorf("%s needs a subject of %s", e.id, e.res.Subject)
	case e.res.Subject == "" && subject != nil:
		return spi.EventIntent{}, fmt.Errorf("%s has no subject", e.id)
	case subject != nil && subject.Entity != e.res.Subject:
		return spi.EventIntent{}, fmt.Errorf("%s subject must be a %s ref, got %s", e.id, e.res.Subject, subject.Entity)
	}
	return spi.EventIntent{Event: e.id, Subject: subject, Payload: payload}, nil
}

// IntentOf is the typed Intent: it encodes payload with encoding/json.
func (e *EventDef[P]) IntentOf(subject *spi.ObjectRef, payload P) (spi.EventIntent, error) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return spi.EventIntent{}, serrors.Wrap("extruntime.native.EventDef.IntentOf", err)
	}
	return e.Intent(subject, raw)
}

func (e *EventDef[P]) addTo(s *Set) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	dup, err := s.claim(e.id, "event", func() bool { return s.events[e.id] == EventDefinition(e) })
	if err != nil || dup {
		return err
	}
	s.events[e.id] = e
	return nil
}
