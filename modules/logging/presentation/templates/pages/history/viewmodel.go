package history

import (
	"bytes"
	"encoding/json"
	"net/url"
	"strings"
	"time"

	"github.com/iota-uz/iota-sdk/modules/logging/domain/aggregates/businessevent"
	"github.com/iota-uz/iota-sdk/modules/logging/history"
	"github.com/iota-uz/iota-sdk/pkg/fielddiff"
)

type State string

const (
	StateReady   State = ""
	StateEmpty   State = "empty"
	StateDenied  State = "denied"
	StateInvalid State = "invalid"
	StateError   State = "error"
)

// Filters are the page's URL parameters; dates are whole days, to inclusive.
type Filters struct {
	Resource  string
	ID        string
	Actor     string
	ActorName string
	EventType string
	Channel   string
	Outcome   string
	From      string
	To        string
	Cursor    string
}

func (f Filters) Query() history.Query {
	q := history.Query{Scope: history.Scope{Kind: history.ScopeAll}, ActorID: f.Actor, Cursor: f.Cursor}
	switch {
	case f.ID != "":
		q.Scope = history.Scope{Kind: history.ScopeEntity, Resource: f.Resource, ID: f.ID}
	case f.Resource != "":
		q.Scope = history.Scope{Kind: history.ScopeResource, Resource: f.Resource}
	}
	if f.EventType != "" {
		q.EventTypes = []string{f.EventType}
	}
	if f.Channel != "" {
		q.Channels = []string{f.Channel}
	}
	if f.Outcome != "" {
		q.Outcomes = []businessevent.Outcome{businessevent.Outcome(f.Outcome)}
	}
	q.From = dayStart(f.From, 0)
	q.To = dayStart(f.To, 1)
	return q
}

// dayStart turns a date into an RFC 3339 time; anything else is passed on for the service to reject.
func dayStart(date string, offset int) string {
	if date == "" {
		return ""
	}
	day, err := time.Parse(time.DateOnly, date)
	if err != nil {
		return date
	}
	return day.AddDate(0, 0, offset).Format(time.RFC3339)
}

// URL keeps the scope and filters while replacing the given parameters; an empty value removes one.
func (f Filters) URL(basePath string, set ...string) string {
	values := url.Values{}
	for key, value := range map[string]string{
		"resource": f.Resource, "id": f.ID, "actor": f.Actor, "actor_name": f.ActorName, "event_type": f.EventType,
		"channel": f.Channel, "outcome": f.Outcome, "from": f.From, "to": f.To,
	} {
		if value != "" {
			values.Set(key, value)
		}
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

func (f Filters) Filtered() bool {
	return f.Actor != "" || f.EventType != "" || f.Channel != "" || f.Outcome != "" || f.From != "" || f.To != ""
}

type Option struct {
	Value string
	Label string
}

type PageProps struct {
	BasePath  string
	EventsURL string
	Filters   Filters
	Scope     string
	Resources []Option
	Types     []Option
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
	var groups [][]Event
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

func FormatTime(t time.Time) string {
	return t.Format("02.01.2006 15:04 MST")
}
