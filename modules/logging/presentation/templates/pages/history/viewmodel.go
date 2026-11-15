package history

import (
	"bytes"
	"encoding/json"
	"net/url"
	"strings"
	"time"

	"github.com/iota-uz/iota-sdk/components/scaffold/filterbuilder"
	"github.com/iota-uz/iota-sdk/modules/logging/domain/aggregates/businessevent"
	"github.com/iota-uz/iota-sdk/modules/logging/history"
	"github.com/iota-uz/iota-sdk/pkg/fielddiff"
	"github.com/iota-uz/iota-sdk/pkg/filterq"
)

type State string

const (
	StateReady   State = ""
	StateEmpty   State = "empty"
	StateDenied  State = "denied"
	StateInvalid State = "invalid"
	StateError   State = "error"
)

// Filter field keys of the page's chip builder; they are also the `f` parameter field names.
const (
	FieldPeriod    = "period"
	FieldResource  = "resource"
	FieldEventType = "event_type"
	FieldChannel   = "channel"
	FieldOutcome   = "outcome"
)

// Filters are the page's URL parameters. Resource and ID pin the page to one resource or object
// and come from links; the chip builder owns the rest as Conditions.
type Filters struct {
	Resource   string
	ID         string
	Actor      string
	ActorName  string
	Conditions filterq.FilterSet
	Cursor     string
}

// ParseFilters reads the page's URL parameters; schema is the chip builder's. A bare resource
// parameter, as linked from lists, becomes a resource condition.
func ParseFilters(values url.Values, schema filterq.Schema) Filters {
	f := Filters{
		ID:         values.Get("id"),
		Actor:      values.Get("actor"),
		ActorName:  values.Get("actor_name"),
		Conditions: filterq.Decode(values, schema),
		Cursor:     values.Get("cursor"),
	}
	resource := values.Get("resource")
	switch {
	case f.ID != "":
		f.Resource = resource
	case resource != "" && !f.Conditions.Has(FieldResource):
		f.Conditions = append(filterq.FilterSet{{Field: FieldResource, Op: filterq.OpIs, Values: []string{resource}}}, f.Conditions...)
	}
	return f
}

// Query turns the filters into a history query; days start and end in loc, the application time zone.
func (f Filters) Query(loc *time.Location, now time.Time) history.Query {
	if loc == nil {
		loc = time.UTC
	}
	q := history.Query{Scope: history.Scope{Kind: history.ScopeAll}, ActorID: f.Actor, Cursor: f.Cursor}
	if f.ID != "" {
		q.Scope = history.Scope{Kind: history.ScopeEntity, Resource: f.Resource, ID: f.ID}
	}
	var from, to time.Time
	for _, c := range f.Conditions {
		switch c.Field {
		case FieldResource:
			if f.ID == "" {
				q.Resources = c.Values
			}
		case FieldEventType:
			q.EventTypes = c.Values
		case FieldChannel:
			q.Channels = c.Values
		case FieldOutcome:
			q.Outcomes = make([]businessevent.Outcome, len(c.Values))
			for i, v := range c.Values {
				q.Outcomes[i] = businessevent.Outcome(v)
			}
		case FieldPeriod:
			lo, hi, ok := dayRange(c, loc, now.In(loc))
			if !ok {
				continue
			}
			if !lo.IsZero() && lo.After(from) {
				from = lo
			}
			if !hi.IsZero() && (to.IsZero() || hi.Before(to)) {
				to = hi
			}
		}
	}
	if !from.IsZero() {
		q.From = from.Format(time.RFC3339)
	}
	if !to.IsZero() {
		q.To = to.Format(time.RFC3339)
	}
	return q
}

// dayRange resolves a period condition to a half-open time range whose bounds are day starts in
// loc; a zero bound is open.
func dayRange(c filterq.Condition, loc *time.Location, now time.Time) (time.Time, time.Time, bool) {
	day := func(i int) (time.Time, bool) {
		if i >= len(c.Values) {
			return time.Time{}, false
		}
		d, err := time.ParseInLocation(filterq.DateLayout, c.Values[i], loc)
		return d, err == nil
	}
	switch c.Op {
	case filterq.OpOn:
		d, ok := day(0)
		return d, d.AddDate(0, 0, 1), ok
	case filterq.OpBefore:
		d, ok := day(0)
		return time.Time{}, d, ok
	case filterq.OpAfter:
		d, ok := day(0)
		return d.AddDate(0, 0, 1), time.Time{}, ok
	case filterq.OpBetween:
		lo, hi, ok := c.DateRange(now)
		return lo, hi.AddDate(0, 0, 1), ok
	case filterq.OpIs, filterq.OpIsNot, filterq.OpEq, filterq.OpGt, filterq.OpLt:
	}
	return time.Time{}, time.Time{}, false
}

// URL keeps the scope and filters while replacing the given parameters; an empty value removes one.
func (f Filters) URL(basePath string, set ...string) string {
	values := url.Values{}
	for key, value := range map[string]string{"id": f.ID, "actor": f.Actor, "actor_name": f.ActorName} {
		if value != "" {
			values.Set(key, value)
		}
	}
	if f.ID != "" && f.Resource != "" {
		values.Set("resource", f.Resource)
	}
	if len(f.Conditions) > 0 {
		values.Set(filterq.PresenceParam, "1")
		filterq.EncodeTo(f.Conditions, values)
	}
	for i := 0; i+1 < len(set); i += 2 {
		if set[i+1] == "" {
			values.Del(set[i])
		} else {
			values.Set(set[i], set[i+1])
		}
	}
	if len(values) == 0 {
		return basePath
	}
	return basePath + "?" + values.Encode()
}

// Scope drops everything but the pinned resource and object.
func (f Filters) Scope() Filters {
	return Filters{Resource: f.Resource, ID: f.ID}
}

func (f Filters) Filtered() bool {
	return f.Actor != "" || len(f.Conditions) > 0
}

type PageProps struct {
	BasePath  string
	EventsURL string
	Filters   Filters
	Scope     string
	Builder   filterbuilder.Props
}

type EventsProps struct {
	BasePath string
	RetryURL string
	Filters  Filters
	State    State
	Groups   [][]Event
	NextURL  string
}

type Event struct {
	history.Item
	Changes []Change
}

type Change struct {
	Subject string
	Fields  []Field
}

type Field struct {
	Label  string
	Before fielddiff.FieldValue
	After  fielddiff.FieldValue
}

// Group joins consecutive events of one operation, so a bulk action reads as one entry.
func Group(items []history.Item, label func(item history.Item, resource, field string) string) [][]Event {
	groups := make([][]Event, 0, len(items))
	for _, item := range items {
		event := Event{Item: item}
		for _, c := range item.Changes {
			change := Change{Subject: subjectLabel(item, c.Resource, c.ID)}
			for _, d := range c.Fields {
				change.Fields = append(change.Fields, Field{Label: label(item, c.Resource, d.Field), Before: d.Before, After: d.After})
			}
			event.Changes = append(event.Changes, change)
		}
		last := len(groups) - 1
		if last >= 0 && item.OperationID != nil && groups[last][0].OperationID != nil && *groups[last][0].OperationID == *item.OperationID {
			groups[last] = append(groups[last], event)
			continue
		}
		groups = append(groups, []Event{event})
	}
	return groups
}

func subjectLabel(item history.Item, resource, id string) string {
	for _, s := range item.Subjects {
		if s.Resource == resource && s.ID == id {
			return s.Label
		}
	}
	return resource + " #" + id
}

func ActorName(a *businessevent.ActorSnapshot) string {
	if a == nil {
		return ""
	}
	if a.Kind != businessevent.ActorUser {
		return a.DisplayName
	}
	name := strings.Join(strings.Fields(a.LastName+" "+a.FirstName+" "+a.MiddleName), " ")
	if name == "" {
		return "#" + a.UserID
	}
	return name
}

// Value renders a stored field value for reading; hidden and absent values are told apart by the caller.
func Value(v fielddiff.FieldValue) string {
	var decoded any
	decoder := json.NewDecoder(bytes.NewReader(v.Value))
	decoder.UseNumber()
	if err := decoder.Decode(&decoded); err != nil {
		return string(v.Value)
	}
	switch value := decoded.(type) {
	case nil:
		return "—"
	case string:
		return value
	case bool, json.Number:
		return string(v.Value)
	case map[string]any:
		if amount, ok := value["amount"].(string); ok {
			currency, _ := value["currency"].(string)
			return amount + " " + currency
		}
	}
	return string(v.Value)
}

// TimeValue is the RFC 3339 form of an event time, rendered by components.LocalDateTime.
func TimeValue(t time.Time) string {
	return t.UTC().Format(time.RFC3339)
}
