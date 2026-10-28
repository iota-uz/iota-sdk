package history

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/logging/domain/aggregates/businessevent"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/fielddiff"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

var (
	ErrSchemaInvalid     = serrors.NewInvalid("business event does not match its schema").WithReason("schema_invalid")
	ErrActorMissing      = serrors.NewInvalid("business event has no actor").WithReason("actor_missing")
	ErrPersistenceFailed = serrors.NewInternal("business event was not persisted").WithReason("persistence_failed")
	// ErrExecutionMissing means the entry point never set a trusted execution: wrap it with
	// WithExecution, SystemExecution, JobExecution or ImportExecution.
	ErrExecutionMissing = serrors.NewInvalid("business action has no execution context; the entry point must set history.WithExecution or history.SystemExecution").WithReason("execution_missing")
)

var eventIDNamespace = uuid.MustParse("f4f27838-83d5-4f52-b673-7b16a7874ec4")

// Input is what an application service knows about the action. Actor, channel, tenant and
// correlation IDs come from the trusted context, never from Input.
type Input struct {
	Type    string
	Version int
	Outcome businessevent.Outcome
	// OccurredAt defaults to the time of recording.
	OccurredAt time.Time
	// Key tells apart events of one type about the same subjects within one operation.
	Key      string
	Subjects []businessevent.Subject
	Changes  []businessevent.Change
}

// Writer records business events inside the caller's transaction. A failed Record must abort
// the business change: callers return its error so the transaction rolls back.
type Writer interface {
	// Record returns the event ID. The ID is derived from the operation, event type and
	// subjects (or Key), so retrying the same event yields the same ID and stores nothing new;
	// retrying it with different content fails with businessevent.ErrConflict.
	Record(ctx context.Context, in Input) (uuid.UUID, error)
}

func NewWriter(registry *Registry, repo businessevent.Repository) Writer {
	return &writer{registry: registry, repo: repo, now: time.Now}
}

type writer struct {
	registry *Registry
	repo     businessevent.Repository
	now      func() time.Time
}

func (w *writer) Record(ctx context.Context, in Input) (uuid.UUID, error) {
	const op serrors.Op = "history.Writer.Record"
	event, err := w.build(ctx, in)
	if err != nil {
		return uuid.Nil, serrors.Wrap(op, err)
	}
	if err := w.repo.Append(ctx, event); err != nil {
		if errors.Is(err, businessevent.ErrConflict) {
			return uuid.Nil, serrors.Wrap(op, err)
		}
		return uuid.Nil, serrors.Wrap(op, fmt.Errorf("%w: %w", ErrPersistenceFailed, err))
	}
	return event.ID, nil
}

func (w *writer) build(ctx context.Context, in Input) (businessevent.Event, error) {
	def, ok := w.registry.Get(in.Type, in.Version)
	if !ok || def.Category == businessevent.CategoryLegacy {
		return businessevent.Event{}, schemaInvalid("unknown event type %s v%d", in.Type, in.Version)
	}
	exec, ok := UseExecution(ctx)
	if !ok {
		return businessevent.Event{}, ErrExecutionMissing
	}
	if !exec.Channel.Valid() || exec.OperationID == "" {
		return businessevent.Event{}, schemaInvalid("execution context with channel and operation is required")
	}
	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return businessevent.Event{}, schemaInvalid("tenant is required")
	}
	actor, err := resolveActor(ctx, exec)
	if err != nil {
		return businessevent.Event{}, err
	}
	if exec.Initiator != nil {
		if err := exec.Initiator.Validate(); err != nil {
			return businessevent.Event{}, schemaInvalid("invalid initiator: %v", err)
		}
	}
	if !in.Outcome.Valid() {
		return businessevent.Event{}, schemaInvalid("invalid outcome %q", in.Outcome)
	}
	if in.Outcome != businessevent.OutcomeSucceeded && len(in.Changes) > 0 {
		return businessevent.Event{}, schemaInvalid("%s events carry no changes", in.Outcome)
	}
	subjects, err := validateSubjects(def, in.Subjects)
	if err != nil {
		return businessevent.Event{}, err
	}
	changes, err := validateChanges(def, subjects, in.Changes)
	if err != nil {
		return businessevent.Event{}, err
	}
	now := w.now()
	occurredAt := in.OccurredAt
	if occurredAt.IsZero() {
		occurredAt = now
	}
	return businessevent.Event{
		ID:            eventID(exec.OperationID, in.Type, in.Key, subjects),
		TenantID:      tenantID,
		Type:          in.Type,
		SchemaVersion: in.Version,
		Category:      def.Category,
		OccurredAt:    occurredAt,
		RecordedAt:    now,
		Actor:         &actor,
		Initiator:     exec.Initiator,
		Channel:       exec.Channel,
		OperationID:   exec.OperationID,
		RequestID:     exec.RequestID,
		RunID:         exec.RunID,
		ClientID:      exec.ClientID,
		Outcome:       in.Outcome,
		Subjects:      subjects,
		Changes:       changes,
	}, nil
}

func resolveActor(ctx context.Context, exec Execution) (businessevent.ActorSnapshot, error) {
	actor := exec.Actor
	if actor == nil {
		u, err := composables.UseUser(ctx)
		if err != nil || u == nil {
			return businessevent.ActorSnapshot{}, ErrActorMissing
		}
		snapshot := businessevent.UserActor(u)
		actor = &snapshot
	}
	if err := actor.Validate(); err != nil {
		return businessevent.ActorSnapshot{}, fmt.Errorf("%w: %w", ErrActorMissing, err)
	}
	return *actor, nil
}

func validateSubjects(def Definition, subjects []businessevent.Subject) ([]businessevent.Subject, error) {
	if len(subjects) == 0 && !def.SubjectOptional {
		return nil, schemaInvalid("%s requires a subject", def.Type)
	}
	result := append(make([]businessevent.Subject, 0, len(subjects)), subjects...)
	seen := map[string]bool{}
	for _, s := range result {
		if _, ok := def.resource(s.Resource); !ok || s.ID == "" {
			return nil, schemaInvalid("subject %s/%s is not allowed for %s", s.Resource, s.ID, def.Type)
		}
		if seen[subjectRef(s.Resource, s.ID)] {
			return nil, schemaInvalid("duplicate subject %s/%s", s.Resource, s.ID)
		}
		seen[subjectRef(s.Resource, s.ID)] = true
	}
	sort.Slice(result, func(i, j int) bool {
		return subjectRef(result[i].Resource, result[i].ID) < subjectRef(result[j].Resource, result[j].ID)
	})
	return result, nil
}

func validateChanges(def Definition, subjects []businessevent.Subject, changes []businessevent.Change) ([]businessevent.Change, error) {
	known := map[string]bool{}
	for _, s := range subjects {
		known[subjectRef(s.Resource, s.ID)] = true
	}
	result := make([]businessevent.Change, 0, len(changes))
	seen := map[string]bool{}
	for _, c := range changes {
		ref := subjectRef(c.Resource, c.ID)
		if !known[ref] || seen[ref] {
			return nil, schemaInvalid("change of %s/%s does not match a single subject", c.Resource, c.ID)
		}
		seen[ref] = true
		resource, _ := def.resource(c.Resource)
		fields := make([]fielddiff.Diff, 0, len(c.Fields))
		names := map[string]bool{}
		for _, d := range c.Fields {
			field, ok := resource.field(d.Field)
			if !ok || names[d.Field] {
				return nil, schemaInvalid("field %s.%s is not allowed for %s", c.Resource, d.Field, def.Type)
			}
			names[d.Field] = true
			before, err := fieldValue(field, d.Before)
			if err != nil {
				return nil, schemaInvalid("%s.%s before: %v", c.Resource, d.Field, err)
			}
			after, err := fieldValue(field, d.After)
			if err != nil {
				return nil, schemaInvalid("%s.%s after: %v", c.Resource, d.Field, err)
			}
			fields = append(fields, fielddiff.Diff{Field: d.Field, Before: before, After: after})
		}
		sort.Slice(fields, func(i, j int) bool { return fields[i].Field < fields[j].Field })
		result = append(result, businessevent.Change{Resource: c.Resource, ID: c.ID, Fields: fields})
	}
	sort.Slice(result, func(i, j int) bool {
		return subjectRef(result[i].Resource, result[i].ID) < subjectRef(result[j].Resource, result[j].ID)
	})
	return result, nil
}

func fieldValue(field Field, v fielddiff.FieldValue) (fielddiff.FieldValue, error) {
	switch v.State {
	case fielddiff.StateAbsent, fielddiff.StateRedacted:
		return fielddiff.FieldValue{State: v.State}, nil
	case fielddiff.StateValue:
		if field.Redact {
			return fielddiff.Redacted(), nil
		}
		normalized, err := normalize(field.Type, v.Value)
		if err != nil {
			return fielddiff.FieldValue{}, err
		}
		return fielddiff.Present(normalized), nil
	default:
		return fielddiff.FieldValue{}, fmt.Errorf("unknown state %q", v.State)
	}
}

func eventID(operationID, eventType, key string, subjects []businessevent.Subject) uuid.UUID {
	if key == "" {
		refs := make([]string, len(subjects))
		for i, s := range subjects {
			refs[i] = subjectRef(s.Resource, s.ID)
		}
		key = strings.Join(refs, "\n")
	}
	return uuid.NewSHA1(eventIDNamespace, []byte(operationID+"\x00"+eventType+"\x00"+key))
}

func subjectRef(resource, id string) string {
	return resource + ":" + id
}

func schemaInvalid(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrSchemaInvalid, fmt.Sprintf(format, args...))
}

// Disabled is the writer of an application without the logging component: it records nothing
// and accepts any context, so services keep working until the host installs business history.
func Disabled() Writer {
	return disabledWriter{}
}

type disabledWriter struct{}

func (disabledWriter) Record(context.Context, Input) (uuid.UUID, error) {
	return uuid.Nil, nil
}
