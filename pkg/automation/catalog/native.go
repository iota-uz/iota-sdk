package catalog

import (
	"fmt"
	"sort"
	"strings"

	"github.com/iota-uz/iota-sdk/pkg/automation/expr"
	"github.com/iota-uz/iota-sdk/pkg/automation/graph"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/data"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/manifest"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/native"
)

// NativeEvents is the source of native (Go-owned) business events; *native.Set satisfies it.
type NativeEvents interface {
	Events() []native.EventDefinition
}

// EventMeta is the optional access and description metadata of a native event the adapter reads.
type EventMeta interface {
	Descriptions() manifest.Label
	ReadPermission() string
	Sensitive() map[string][]string // a dotted payload path → the permissions it needs
	Category() string
}

// nativeEntities is the optional part of a NativeEvents that knows the native entities (*native.Set):
// an event without its own read permission takes its subject entity's.
type nativeEntities interface {
	Entities() []data.NativeEntity
}

// failing is a trigger source that could not be built; the registry refuses it.
type failing interface {
	buildError() error
}

type nativeSource struct {
	events []graph.EventSpec
	access map[string]EventAccess
	labels map[string]manifest.Label
	err    error
}

// FromNativeEvents is the trigger source of native events: every event of the set, its payload translated
// with FieldFromManifest, grouped by its category (or the first segment of its ID). A payload of a type
// that cannot be translated makes the registration fail.
func FromNativeEvents(set NativeEvents) TriggerSource {
	src := &nativeSource{access: map[string]EventAccess{}, labels: map[string]manifest.Label{}}
	if set == nil {
		return src
	}
	entityRead := map[string]string{}
	if es, ok := set.(nativeEntities); ok {
		for _, e := range es.Entities() {
			entityRead[e.ID] = e.ReadPermission
		}
	}
	defs := set.Events()
	sort.Slice(defs, func(i, j int) bool { return defs[i].ID() < defs[j].ID() })
	for _, def := range defs {
		res := def.Resource()
		spec := graph.EventSpec{Key: def.ID(), Subject: res.Subject, Title: labelIn(res.Label, "", def.ID())}
		if spec.Subject == "" {
			spec.Subject = graph.SubjectNone
		}
		if res.Payload != nil {
			f, err := FieldFromManifest(*res.Payload)
			if err != nil {
				src.err = fmt.Errorf("native event %s: payload: %w", def.ID(), err)
				return src
			}
			if f = f.Unwrap(); f.Kind != expr.Object {
				src.err = fmt.Errorf("native event %s: payload is not an object", def.ID())
				return src
			}
			spec.Payload = expr.Fields(f.Props)
		}
		access := EventAccess{ReadPermission: entityRead[res.Subject]}
		spec.Group, _, _ = strings.Cut(def.ID(), ".")
		if m, ok := def.(EventMeta); ok {
			if p := m.ReadPermission(); p != "" {
				access.ReadPermission = p
			}
			access.Sensitive = m.Sensitive()
			if c := m.Category(); c != "" {
				spec.Group = c
			}
		}
		src.access[def.ID()] = access
		src.labels[def.ID()] = res.Label
		src.events = append(src.events, spec)
	}
	return src
}

func (s *nativeSource) Events() []graph.EventSpec {
	out := make([]graph.EventSpec, len(s.events))
	for i, e := range s.events {
		e.Payload = e.Payload.Clone()
		out[i] = e
	}
	return out
}

func (s *nativeSource) Requires() []ConnectorKind { return nil }

func (s *nativeSource) buildError() error { return s.err }

func (s *nativeSource) Access(key string) (EventAccess, bool) {
	a, ok := s.access[key]
	return a, ok
}

func (s *nativeSource) Localize(spec graph.EventSpec, locale string) graph.EventSpec {
	spec.Title = labelIn(s.labels[spec.Key], locale, spec.Title)
	return spec
}

// labelIn is the label in a locale, else in English, else fallback.
func labelIn(l manifest.Label, locale, fallback string) string {
	if v := l[locale]; locale != "" && v != "" {
		return v
	}
	if v := l["en"]; v != "" {
		return v
	}
	return fallback
}

// FieldFromManifest translates a contract type into the field language: string, date, uuid, int64 and
// decimal are strings (their wire form), datetime is a timestamp, json is dyn, a ref is an object
// {entity, key}; an optional or nullable type is optional. A property whose name is not a CEL
// identifier is an error.
func FieldFromManifest(t manifest.Type) (expr.Field, error) {
	f, err := fieldOf(t)
	if err != nil {
		return expr.Field{}, err
	}
	if t.Optional || t.Nullable {
		f = expr.OptionalOf(f)
	}
	return f, nil
}

func fieldOf(t manifest.Type) (expr.Field, error) {
	switch t.Kind {
	case manifest.KindString, manifest.KindDate, manifest.KindUUID, manifest.KindInt64, manifest.KindDecimal:
		return expr.StringField, nil
	case manifest.KindInt:
		return expr.IntField, nil
	case manifest.KindBool:
		return expr.BoolField, nil
	case manifest.KindDateTime:
		return expr.TimestampField, nil
	case manifest.KindJSON:
		return expr.DynField, nil
	case manifest.KindEnum:
		if len(t.Values) == 0 {
			return expr.Field{}, fmt.Errorf("an enum without values")
		}
		return expr.EnumOf(append([]string(nil), t.Values...)...), nil
	case manifest.KindRef:
		return expr.ObjectOf(expr.Fields{{Name: "entity", Field: expr.StringField}, {Name: "key", Field: expr.StringField}}), nil
	case manifest.KindArray:
		if t.Items == nil {
			return expr.Field{}, fmt.Errorf("an array without an item type")
		}
		item, err := FieldFromManifest(*t.Items)
		if err != nil {
			return expr.Field{}, fmt.Errorf("items: %w", err)
		}
		return expr.ListOf(item), nil
	case manifest.KindObject:
		props := make(expr.Fields, 0, len(t.Properties))
		for _, p := range t.Properties {
			if !expr.IsName(p.Name) {
				return expr.Field{}, fmt.Errorf("property %q is not a field name", p.Name)
			}
			f, err := FieldFromManifest(p.Type)
			if err != nil {
				return expr.Field{}, fmt.Errorf("%s: %w", p.Name, err)
			}
			props = append(props, expr.Prop{Name: p.Name, Field: f})
		}
		return expr.ObjectOf(props), nil
	}
	return expr.Field{}, fmt.Errorf("type %q is not supported", t.Kind)
}
