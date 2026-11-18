package native

import (
	"encoding/json"
	"fmt"
	"regexp"

	"github.com/iota-uz/iota-sdk/pkg/extruntime/manifest"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/spi"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

var versionedEventID = regexp.MustCompile(`^[a-z][a-z0-9]*(\.[a-z][a-z0-9]*)*(\.[a-z][a-zA-Z0-9_]*)+\.v[0-9]+$`)

// EventSpec is the public part of a native event besides its ID and payload
// type.
type EventSpec struct {
	// Subject is the entity resource ID the event is about ("crm.client");
	// empty for an event without a subject.
	Subject     string
	Label       manifest.Label
	Description string
	// Payload overrides the type derived from the Go payload struct (TypeOf).
	Payload *manifest.Type
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

// EventDef is a native business event with the versioned payload type P. It
// is declared once, as a package-level value, contributed to the Set by its
// module and published by Go code with events.Publish in the transaction of
// the change it reports, so it commits or rolls back with that change.
type EventDef[P any] struct {
	id  string
	res manifest.Resource
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
		return nil, serrors.New(serrors.Invalid, fmt.Sprintf("native event %q must be a dotted ID ending in a version such as .v1", id)).WithOp(op)
	}
	payload := spec.Payload
	if payload == nil {
		t, err := TypeOf[P]()
		if err != nil {
			return nil, serrors.Wrap(op, fmt.Errorf("%s payload: %w", id, err))
		}
		payload = &t
	}
	p := *payload
	res := manifest.Resource{ID: id, Kind: manifest.ResEvent, Subject: spec.Subject, Payload: &p, Label: spec.Label, Description: spec.Description}
	res.ContractHash = manifest.ContractHashOf(res)
	return &EventDef[P]{id: id, res: res}, nil
}

func (e *EventDef[P]) ID() string                  { return e.id }
func (e *EventDef[P]) Resource() manifest.Resource { return cloneResource(e.res) }

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
