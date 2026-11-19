package history

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/permission"
	"github.com/iota-uz/iota-sdk/modules/logging/domain/aggregates/businessevent"
	"github.com/iota-uz/iota-sdk/modules/logging/permissions"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/intl"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

const (
	DefaultLimit = 50
	MaxLimit     = 200
	// LegacyChannel is the channel of records migrated from the former action log, which stored none.
	LegacyChannel = "legacy"
	// maxResultBytes bounds a page; a page cut short by it is marked truncated.
	maxResultBytes = 1 << 20
)

var (
	ErrInvalidInput = serrors.NewInvalid("invalid history query").WithReason("invalid_input")
	ErrForbidden    = serrors.NewPermissionDenied("history is not available to the user").WithReason("forbidden")
)

type ScopeKind string

const (
	ScopeAll      ScopeKind = "all"
	ScopeResource ScopeKind = "resource"
	ScopeEntity   ScopeKind = "entity"
)

type Scope struct {
	Kind     ScopeKind `json:"kind"`
	Resource string    `json:"resource,omitempty"`
	ID       string    `json:"id,omitempty"`
}

// Query is the input of the history read, shared by the UI and the MCP tool. Omitted (nil)
// lists do not filter; empty ones are rejected. Resources keeps events about any of the listed
// resources and combines with the all scope only.
type Query struct {
	Scope      Scope                   `json:"scope"`
	Resources  []string                `json:"resources,omitempty"`
	ActorID    string                  `json:"actor_id,omitempty"`
	EventTypes []string                `json:"event_types,omitempty"`
	Channels   []string                `json:"channels,omitempty"`
	From       string                  `json:"from,omitempty"`
	To         string                  `json:"to,omitempty"`
	Limit      int                     `json:"limit,omitempty"`
	Cursor     string                  `json:"cursor,omitempty"`
	Outcomes   []businessevent.Outcome `json:"outcomes,omitempty"`
}

type Subject struct {
	Resource string `json:"resource"`
	ID       string `json:"id"`
	// Name is the object's caption (Resource.Labels, else the one recorded with the event); empty if unknown.
	Name string `json:"name,omitempty"`
	// Label names the object for reading: the resource label with Name, or with the ID when Name is empty.
	Label string  `json:"label"`
	URL   *string `json:"url"`
}

type Item struct {
	EventID       string                       `json:"event_id"`
	EventType     string                       `json:"event_type"`
	SchemaVersion int                          `json:"schema_version"`
	OccurredAt    time.Time                    `json:"occurred_at"`
	RecordedAt    time.Time                    `json:"recorded_at"`
	Title         string                       `json:"title"`
	Summary       string                       `json:"summary"`
	Actor         *businessevent.ActorSnapshot `json:"actor_snapshot"`
	Initiator     *businessevent.ActorSnapshot `json:"initiator_snapshot,omitempty"`
	Channel       string                       `json:"channel"`
	Outcome       businessevent.Outcome        `json:"outcome,omitempty"`
	OperationID   *string                      `json:"operation_id"`
	Subjects      []Subject                    `json:"subjects"`
	Changes       []businessevent.Change       `json:"changes"`
	// Display is Changes as a reader sees them, in the same order: localized field labels and value
	// texts, with codes translated as their fields declare. Pages and exports render it.
	Display          []ChangeDisplay `json:"display"`
	LegacyIncomplete bool            `json:"legacy_incomplete"`
	// Correlation IDs are shown in the event details only.
	RequestID string `json:"-"`
	RunID     string `json:"-"`
	ClientID  string `json:"-"`
}

type Result struct {
	Items      []Item  `json:"items"`
	NextCursor *string `json:"next_cursor"`
	Truncated  bool    `json:"truncated"`
}

// Service reads business history for the user in ctx. Tenant, resource and field permissions
// are applied on every read, before pagination.
type Service struct {
	registry *Registry
	repo     businessevent.Repository
}

func NewService(registry *Registry, repo businessevent.Repository) *Service {
	return &Service{registry: registry, repo: repo}
}

func (s *Service) History(ctx context.Context, q Query) (Result, error) {
	const op serrors.Op = "history.Service.History"
	u, err := composables.UseUser(ctx)
	if err != nil || u == nil || !u.Can(permissions.ViewLogs) {
		return Result{}, serrors.Wrap(op, ErrForbidden)
	}
	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return Result{}, serrors.Wrap(op, ErrForbidden)
	}
	params, limit, err := s.params(q)
	if err != nil {
		return Result{}, serrors.Wrap(op, err)
	}
	for _, name := range append([]string{params.Resource}, params.Resources...) {
		if name != "" && !s.registry.Readable(name, u.Can) {
			return Result{}, serrors.Wrap(op, ErrForbidden)
		}
	}
	if params.Readable, params.Restricted, err = s.access(ctx, u.Can); err != nil {
		return Result{}, serrors.Wrap(op, err)
	}
	binding, err := cursorBinding(tenantID, u.ID(), q)
	if err != nil {
		return Result{}, serrors.Wrap(op, err)
	}
	if q.Cursor != "" {
		position, err := decodeCursor(q.Cursor, binding)
		if err != nil {
			return Result{}, serrors.Wrap(op, err)
		}
		params.After = &position
	}
	params.Limit = limit + 1
	events, err := s.repo.Find(ctx, params)
	if err != nil {
		return Result{}, serrors.Wrap(op, err)
	}
	page := events[:min(len(events), limit)]
	readable := make([]businessevent.Event, len(page))
	for i, e := range page {
		readable[i] = s.registry.ForReader(e, u.Can)
	}
	names := s.subjectNames(ctx, readable)
	result := Result{Items: make([]Item, 0, len(page))}
	if len(events) > limit {
		if result.NextCursor, err = encodeCursor(events[limit-1].Position(), binding); err != nil {
			return Result{}, serrors.Wrap(op, err)
		}
	}
	size := 0
	for i, e := range readable {
		item := s.item(ctx, e, names)
		data, err := json.Marshal(item)
		if err != nil {
			return Result{}, serrors.Wrap(op, err)
		}
		if size += len(data); size > maxResultBytes && i > 0 {
			result.Truncated = true
			if result.NextCursor, err = encodeCursor(page[i-1].Position(), binding); err != nil {
				return Result{}, serrors.Wrap(op, err)
			}
			break
		}
		result.Items = append(result.Items, item)
	}
	return result, nil
}

// Subject returns the caption of one object for the reader in ctx, e.g. for the title of its history
// page. It is false when the reader may not see any event about the object, so a caption never
// discloses an object hidden by Resource.Visible.
func (s *Service) Subject(ctx context.Context, resource, id string) (Subject, bool) {
	u, err := composables.UseUser(ctx)
	if err != nil || u == nil || !u.Can(permissions.ViewLogs) || id == "" || !s.registry.Readable(resource, u.Can) {
		return Subject{}, false
	}
	params := businessevent.FindParams{Resource: resource, SubjectID: id, Limit: 1}
	if params.Readable, params.Restricted, err = s.access(ctx, u.Can); err != nil {
		return Subject{}, false
	}
	events, err := s.repo.Find(ctx, params)
	if err != nil || len(events) == 0 {
		return Subject{}, false
	}
	names := s.subjectNames(ctx, events[:1])
	return s.subject(ctx, resource, id, names[subjectRef(resource, id)]), true
}

// access lists the resources the reader may see and the object restrictions of those that limit it.
func (s *Service) access(ctx context.Context, can func(permission.Permission) bool) ([]string, []businessevent.Restriction, error) {
	resources := s.registry.Resources()
	readable := make([]string, 0, len(resources))
	var restricted []businessevent.Restriction
	for _, r := range resources {
		if !s.registry.Readable(r.Name, can) {
			continue
		}
		readable = append(readable, r.Name)
		if r.Visible == nil {
			continue
		}
		v, err := r.Visible(ctx)
		if err != nil {
			return nil, nil, fmt.Errorf("visibility of %s: %w", r.Name, err)
		}
		if !v.All {
			restricted = append(restricted, businessevent.Restriction{Resource: r.Name, IDs: v.IDs, Condition: v.Condition, Args: v.Args})
		}
	}
	return readable, restricted, nil
}

// subjectNames captions the subjects of the events, keyed by subjectRef: one provider call per
// resource with every ID, then the caption recorded with the newest event naming the object.
func (s *Service) subjectNames(ctx context.Context, events []businessevent.Event) map[string]string {
	names := map[string]string{}
	ids := map[string][]string{}
	seen := map[string]bool{}
	for _, e := range events {
		for _, subject := range e.Subjects {
			ref := subjectRef(subject.Resource, subject.ID)
			if seen[ref] {
				continue
			}
			seen[ref] = true
			ids[subject.Resource] = append(ids[subject.Resource], subject.ID)
		}
	}
	for _, r := range s.registry.Resources() {
		if r.Labels == nil || len(ids[r.Name]) == 0 {
			continue
		}
		labels, err := r.Labels(ctx, ids[r.Name])
		if err != nil {
			composables.UseLogger(ctx).WithError(err).WithField("resource", r.Name).Warn("business history: object captions are unavailable")
			continue
		}
		for _, id := range ids[r.Name] {
			if label := strings.TrimSpace(labels[id]); label != "" {
				names[subjectRef(r.Name, id)] = label
			}
		}
	}
	for _, e := range events {
		for _, subject := range e.Subjects {
			ref := subjectRef(subject.Resource, subject.ID)
			if _, ok := names[ref]; !ok && subject.Label != "" {
				names[ref] = subject.Label
			}
		}
	}
	return names
}

// Visible reports whether the user in ctx may read history about the resource.
func (s *Service) Visible(ctx context.Context, resource string) bool {
	u, err := composables.UseUser(ctx)
	return err == nil && u != nil && u.Can(permissions.ViewLogs) && s.registry.Readable(resource, u.Can)
}

func (s *Service) params(q Query) (businessevent.FindParams, int, error) {
	var params businessevent.FindParams
	known := map[string]bool{}
	for _, r := range s.registry.Resources() {
		known[r.Name] = true
	}
	switch q.Scope.Kind {
	case ScopeAll:
		if q.Scope.Resource != "" || q.Scope.ID != "" {
			return params, 0, invalidInput("scope all takes no resource or id")
		}
	case ScopeResource, ScopeEntity:
		if !known[q.Scope.Resource] {
			return params, 0, invalidInput("unknown resource %q", q.Scope.Resource)
		}
		if (q.Scope.Kind == ScopeEntity) == (q.Scope.ID == "") {
			return params, 0, invalidInput("scope %s requires id only for entity", q.Scope.Kind)
		}
		params.Resource, params.SubjectID = q.Scope.Resource, q.Scope.ID
	default:
		return params, 0, invalidInput("unknown scope %q", q.Scope.Kind)
	}
	if q.Resources != nil {
		if len(q.Resources) == 0 {
			return params, 0, invalidInput("resources must not be empty")
		}
		if q.Scope.Kind != ScopeAll {
			return params, 0, invalidInput("resources apply to scope all only")
		}
		for _, name := range q.Resources {
			if !known[name] {
				return params, 0, invalidInput("unknown resource %q", name)
			}
		}
		params.Resources = q.Resources
	}
	params.ActorID = q.ActorID
	if q.EventTypes != nil {
		if len(q.EventTypes) == 0 {
			return params, 0, invalidInput("event_types must not be empty")
		}
		types := map[string]bool{}
		for _, d := range s.registry.Definitions() {
			types[d.Type] = true
		}
		for _, t := range q.EventTypes {
			if !types[t] {
				return params, 0, invalidInput("unknown event type %q", t)
			}
		}
		params.EventTypes = q.EventTypes
	}
	if q.Channels != nil {
		if len(q.Channels) == 0 {
			return params, 0, invalidInput("channels must not be empty")
		}
		for _, c := range q.Channels {
			switch {
			case c == LegacyChannel:
				params.WithoutChannel = true
			case businessevent.Channel(c).Valid():
				params.Channels = append(params.Channels, businessevent.Channel(c))
			default:
				return params, 0, invalidInput("unknown channel %q", c)
			}
		}
	}
	if q.Outcomes != nil {
		if len(q.Outcomes) == 0 {
			return params, 0, invalidInput("outcomes must not be empty")
		}
		for _, o := range q.Outcomes {
			if !o.Valid() {
				return params, 0, invalidInput("unknown outcome %q", o)
			}
		}
		params.Outcomes = q.Outcomes
	}
	var err error
	if params.From, err = parseTime("from", q.From); err != nil {
		return params, 0, err
	}
	if params.To, err = parseTime("to", q.To); err != nil {
		return params, 0, err
	}
	if !params.From.IsZero() && !params.To.IsZero() && !params.From.Before(params.To) {
		return params, 0, invalidInput("from must be before to")
	}
	limit := q.Limit
	if limit == 0 {
		limit = DefaultLimit
	}
	if limit < 1 || limit > MaxLimit {
		return params, 0, invalidInput("limit must be between 1 and %d", MaxLimit)
	}
	return params, limit, nil
}

func (s *Service) item(ctx context.Context, e businessevent.Event, names map[string]string) Item {
	item := Item{
		EventID:       e.ID.String(),
		EventType:     e.Type,
		SchemaVersion: e.SchemaVersion,
		OccurredAt:    e.Position().Time,
		RecordedAt:    e.RecordedAt,
		Actor:         e.Actor,
		Initiator:     e.Initiator,
		Channel:       string(e.Channel),
		Outcome:       e.Outcome,
		Subjects:      make([]Subject, len(e.Subjects)),
		Changes:       e.Changes,
		Display:       make([]ChangeDisplay, 0, len(e.Changes)),
		RequestID:     e.RequestID,
		RunID:         e.RunID,
		ClientID:      e.ClientID,
	}
	def, known := s.registry.Get(e.Type, e.SchemaVersion)
	if known {
		item.Title = intl.TWithDefault(ctx, def.TitleKey, def.TitleKey)
		item.Summary = intl.TWithDefault(ctx, def.DescriptionKey, def.DescriptionKey)
	} else {
		item.Title = intl.TWithDefault(ctx, "BusinessHistory.UnknownEvent", "Unknown action")
	}
	if e.OperationID != "" {
		item.OperationID = &e.OperationID
	}
	if e.Legacy != nil {
		item.Channel = LegacyChannel
		item.LegacyIncomplete = e.Legacy.Incomplete
	}
	for i, subject := range e.Subjects {
		item.Subjects[i] = s.subject(ctx, subject.Resource, subject.ID, names[subjectRef(subject.Resource, subject.ID)])
	}
	for _, c := range e.Changes {
		resource, _ := def.resource(c.Resource)
		display := ChangeDisplay{
			Resource: c.Resource,
			ID:       c.ID,
			Subject:  s.subject(ctx, c.Resource, c.ID, names[subjectRef(c.Resource, c.ID)]).Label,
			Fields:   make([]FieldDisplay, len(c.Fields)),
		}
		for j, d := range c.Fields {
			field, ok := resource.field(d.Field)
			label := d.Field
			if ok {
				label = intl.TWithDefault(ctx, field.LabelKey, d.Field)
			}
			display.Fields[j] = FieldDisplay{
				Field:  d.Field,
				Label:  label,
				Before: displayValue(ctx, field, d.Before),
				After:  displayValue(ctx, field, d.After),
			}
		}
		item.Display = append(item.Display, display)
	}
	return item
}

// subject captions an object as "<resource label>: <name>", or "<resource label> #<id>" without a name.
func (s *Service) subject(ctx context.Context, resource, id, name string) Subject {
	label := resource
	for _, r := range s.registry.Resources() {
		if r.Name == resource && r.LabelKey != "" {
			label = intl.TWithDefault(ctx, r.LabelKey, resource)
		}
	}
	if name == "" {
		return Subject{Resource: resource, ID: id, Label: label + " #" + id}
	}
	return Subject{Resource: resource, ID: id, Name: name, Label: label + ": " + name}
}

type cursor struct {
	Time    time.Time `json:"t"`
	ID      uuid.UUID `json:"id"`
	Binding string    `json:"b"`
}

// cursorBinding ties a cursor to the tenant, the reader and every filter, so it cannot
// continue a different listing.
func cursorBinding(tenantID uuid.UUID, userID uint, q Query) (string, error) {
	q.Cursor, q.Limit = "", 0
	data, err := json.Marshal(struct {
		TenantID uuid.UUID `json:"tenant_id"`
		UserID   uint      `json:"user_id"`
		Query    Query     `json:"query"`
	}{tenantID, userID, q})
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:16]), nil
}

func encodeCursor(p businessevent.Position, binding string) (*string, error) {
	data, err := json.Marshal(cursor{Time: p.Time, ID: p.ID, Binding: binding})
	if err != nil {
		return nil, err
	}
	encoded := base64.RawURLEncoding.EncodeToString(data)
	return &encoded, nil
}

func decodeCursor(value, binding string) (businessevent.Position, error) {
	data, err := base64.RawURLEncoding.DecodeString(value)
	var c cursor
	if err != nil || json.Unmarshal(data, &c) != nil || c.Binding != binding {
		return businessevent.Position{}, invalidInput("cursor does not belong to this query")
	}
	return businessevent.Position{Time: c.Time, ID: c.ID}, nil
}

func parseTime(name, value string) (time.Time, error) {
	if value == "" {
		return time.Time{}, nil
	}
	t, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return time.Time{}, invalidInput("%s must be an RFC 3339 time", name)
	}
	return t, nil
}

func invalidInput(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrInvalidInput, fmt.Sprintf(format, args...))
}
