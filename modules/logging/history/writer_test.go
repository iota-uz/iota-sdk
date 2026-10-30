package history

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/internet"
	"github.com/iota-uz/iota-sdk/modules/logging/domain/aggregates/businessevent"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/fielddiff"
	"github.com/stretchr/testify/require"
)

type memoryRepository struct {
	events map[uuid.UUID]businessevent.Event
	err    error
}

func (r *memoryRepository) Append(_ context.Context, e businessevent.Event) error {
	if r.err != nil {
		return r.err
	}
	if _, ok := r.events[e.ID]; !ok {
		r.events[e.ID] = e
	}
	return nil
}

func (r *memoryRepository) Get(_ context.Context, id uuid.UUID) (businessevent.Event, error) {
	e, ok := r.events[id]
	if !ok {
		return businessevent.Event{}, businessevent.ErrNotFound
	}
	return e, nil
}

func (r *memoryRepository) ListByOperation(context.Context, string) ([]businessevent.Event, error) {
	return nil, nil
}

func paymentDefinition() Definition {
	return Definition{
		Type:           "finance.payment.posted",
		Version:        1,
		Category:       businessevent.CategoryBusiness,
		TitleKey:       "Payments.History.Posted.Title",
		DescriptionKey: "Payments.History.Posted.Description",
		Resources: []Resource{{Name: "payment", Fields: []Field{
			{Name: "amount", LabelKey: "Payments.Fields.Amount", Type: ValueMoney},
			{Name: "posted", LabelKey: "Payments.Fields.Posted", Type: ValueBoolean},
			{Name: "posted_at", LabelKey: "Payments.Fields.PostedAt", Type: ValueDateTime},
			{Name: "token", LabelKey: "Payments.Fields.Token", Type: ValueString, Redact: true},
		}}},
	}
}

func testUser(t *testing.T) user.User {
	t.Helper()
	email, err := internet.NewEmail("actor@example.com")
	require.NoError(t, err)
	return user.New("Ali", "Valiyev", email, user.UILanguageEN, user.WithID(7), user.WithMiddleName("Karimovich"))
}

func newTestWriter(t *testing.T, repo *memoryRepository) *writer {
	t.Helper()
	registry := NewRegistry()
	require.NoError(t, registry.Register(paymentDefinition(), LegacyActionLogDefinition()))
	w := NewWriter(registry, repo).(*writer)
	w.now = func() time.Time { return time.Date(2026, 10, 28, 9, 0, 0, 0, time.UTC) }
	return w
}

func testContext(t *testing.T, exec *Execution) context.Context {
	t.Helper()
	ctx := composables.WithTenantID(context.Background(), uuid.MustParse("00000000-0000-0000-0000-000000000001"))
	ctx = composables.WithUser(ctx, testUser(t))
	if exec != nil {
		ctx = WithExecution(ctx, *exec)
	}
	return ctx
}

func diff(field string, before, after fielddiff.FieldValue) fielddiff.Diff {
	return fielddiff.Diff{Field: field, Before: before, After: after}
}

func validInput() Input {
	return Input{
		Type:     "finance.payment.posted",
		Version:  1,
		Outcome:  businessevent.OutcomeSucceeded,
		Subjects: []businessevent.Subject{{Resource: "payment", ID: "42"}},
		Changes: []businessevent.Change{{Resource: "payment", ID: "42", Fields: []fielddiff.Diff{
			diff("posted", fielddiff.Present(json.RawMessage(`false`)), fielddiff.Present(json.RawMessage(`true`))),
		}}},
	}
}

func TestWriter_Record_RejectsInvalidInput(t *testing.T) {
	t.Parallel()
	ui := &Execution{Channel: businessevent.ChannelUI, OperationID: "op-1"}
	tests := []struct {
		name   string
		exec   *Execution
		mutate func(*Input)
		want   error
	}{
		{"unknown type", ui, func(in *Input) { in.Type = "finance.payment.unknown" }, ErrSchemaInvalid},
		{"unknown version", ui, func(in *Input) { in.Version = 2 }, ErrSchemaInvalid},
		{"legacy type", ui, func(in *Input) { in.Type, in.Changes, in.Subjects = LegacyActionLogType, nil, nil }, ErrSchemaInvalid},
		{"no execution context", nil, func(*Input) {}, ErrExecutionMissing},
		{"no operation", &Execution{Channel: businessevent.ChannelUI}, func(*Input) {}, ErrSchemaInvalid},
		{"unknown channel", &Execution{Channel: "http", OperationID: "op-1"}, func(*Input) {}, ErrSchemaInvalid},
		{"invalid outcome", ui, func(in *Input) { in.Outcome = "partial" }, ErrSchemaInvalid},
		{"denied with changes", ui, func(in *Input) { in.Outcome = businessevent.OutcomeDenied }, ErrSchemaInvalid},
		{"missing subject", ui, func(in *Input) { in.Subjects, in.Changes = nil, nil }, ErrSchemaInvalid},
		{"unregistered resource", ui, func(in *Input) { in.Subjects[0].Resource = "expense" }, ErrSchemaInvalid},
		{"change without subject", ui, func(in *Input) { in.Changes[0].ID = "43" }, ErrSchemaInvalid},
		{"unknown field", ui, func(in *Input) { in.Changes[0].Fields[0].Field = "comment" }, ErrSchemaInvalid},
		{"wrong value type", ui, func(in *Input) {
			in.Changes[0].Fields[0].After = fielddiff.Present(json.RawMessage(`"yes"`))
		}, ErrSchemaInvalid},
		{"time without zone", ui, func(in *Input) {
			in.Changes[0].Fields = []fielddiff.Diff{diff("posted_at", fielddiff.Absent(), fielddiff.Present(json.RawMessage(`"2026-10-28T09:00:00"`)))}
		}, ErrSchemaInvalid},
		{"money as number", ui, func(in *Input) {
			in.Changes[0].Fields = []fielddiff.Diff{diff("amount", fielddiff.Absent(), fielddiff.Present(json.RawMessage(`{"amount":10.5,"currency":"UZS"}`)))}
		}, ErrSchemaInvalid},
		{"service actor without name", &Execution{Channel: businessevent.ChannelJob, OperationID: "op-1", Actor: &businessevent.ActorSnapshot{SchemaVersion: 1, Kind: businessevent.ActorService, ID: "importer"}}, func(*Input) {}, ErrActorMissing},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			repo := &memoryRepository{events: map[uuid.UUID]businessevent.Event{}}
			in := validInput()
			in.Subjects = append([]businessevent.Subject(nil), in.Subjects...)
			in.Changes = []businessevent.Change{{Resource: "payment", ID: "42", Fields: append([]fielddiff.Diff(nil), in.Changes[0].Fields...)}}
			tt.mutate(&in)
			_, err := newTestWriter(t, repo).Record(testContext(t, tt.exec), in)
			require.ErrorIs(t, err, tt.want)
			require.Empty(t, repo.events)
		})
	}
}

func TestWriter_Record_RequiresActor(t *testing.T) {
	t.Parallel()
	repo := &memoryRepository{events: map[uuid.UUID]businessevent.Event{}}
	ctx := composables.WithTenantID(context.Background(), uuid.New())
	ctx = WithExecution(ctx, Execution{Channel: businessevent.ChannelAPI, OperationID: "op-1"})
	_, err := newTestWriter(t, repo).Record(ctx, validInput())
	require.ErrorIs(t, err, ErrActorMissing)
	require.Empty(t, repo.events)
}

func TestWriter_Record_BuildsEventFromTrustedContext(t *testing.T) {
	t.Parallel()
	repo := &memoryRepository{events: map[uuid.UUID]businessevent.Event{}}
	initiator := businessevent.UserActor(testUser(t))
	ctx := testContext(t, &Execution{Channel: businessevent.ChannelMCP, OperationID: "plan-1", Initiator: &initiator, RunID: "run-9", ClientID: "claude-desktop", RequestID: "req-1"})
	in := validInput()
	in.Changes[0].Fields = append(in.Changes[0].Fields,
		diff("token", fielddiff.Absent(), fielddiff.Present(json.RawMessage(`"secret-token"`))),
		diff("amount", fielddiff.Present(json.RawMessage(`{ "currency": "UZS", "amount": "100.50" }`)), fielddiff.Present(json.RawMessage(`null`))),
	)
	w := newTestWriter(t, repo)

	id, err := w.Record(ctx, in)
	require.NoError(t, err)
	again, err := w.Record(ctx, in)
	require.NoError(t, err)
	require.Equal(t, id, again, "the same event of the same operation keeps its ID on retry")
	other, err := w.Record(WithExecution(ctx, Execution{Channel: businessevent.ChannelMCP, OperationID: "plan-2"}), validInput())
	require.NoError(t, err)
	require.NotEqual(t, id, other)

	e := repo.events[id]
	require.Equal(t, businessevent.ChannelMCP, e.Channel)
	require.Equal(t, "plan-1", e.OperationID)
	require.Equal(t, "run-9", e.RunID)
	require.Equal(t, "claude-desktop", e.ClientID)
	require.Equal(t, &businessevent.ActorSnapshot{SchemaVersion: 1, Kind: businessevent.ActorUser, UserID: "7", FirstName: "Ali", LastName: "Valiyev", MiddleName: "Karimovich"}, e.Actor)
	require.Equal(t, w.now(), e.RecordedAt)
	require.Equal(t, w.now(), e.OccurredAt)
	require.Equal(t, businessevent.CategoryBusiness, e.Category)
	fields := e.Changes[0].Fields
	require.Equal(t, []string{"amount", "posted", "token"}, []string{fields[0].Field, fields[1].Field, fields[2].Field})
	require.JSONEq(t, `{"amount":"100.50","currency":"UZS"}`, string(fields[0].Before.Value))
	require.Equal(t, fielddiff.Absent(), fields[2].Before)
	require.Equal(t, fielddiff.Redacted(), fields[2].After)
}

func TestWriter_Record_ServiceActorWithInitiator(t *testing.T) {
	t.Parallel()
	repo := &memoryRepository{events: map[uuid.UUID]businessevent.Event{}}
	service := businessevent.ServiceActor("bank-sync", "Bank synchronization")
	initiator := businessevent.UserActor(testUser(t))
	ctx := WithExecution(composables.WithTenantID(context.Background(), uuid.New()), Execution{Channel: businessevent.ChannelJob, OperationID: "sync-1", Actor: &service, Initiator: &initiator})
	id, err := newTestWriter(t, repo).Record(ctx, validInput())
	require.NoError(t, err)
	require.Equal(t, &service, repo.events[id].Actor)
	require.Equal(t, &initiator, repo.events[id].Initiator)
}

func TestWriter_Record_WrapsPersistenceErrors(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		err  error
		want error
	}{
		{"database failure", errors.New("connection reset"), ErrPersistenceFailed},
		{"conflict", businessevent.ErrConflict, businessevent.ErrConflict},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			repo := &memoryRepository{err: tt.err}
			_, err := newTestWriter(t, repo).Record(testContext(t, &Execution{Channel: businessevent.ChannelUI, OperationID: "op-1"}), validInput())
			require.ErrorIs(t, err, tt.want)
		})
	}
}

func TestNormalize_CanonicalizesEqualValues(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		typ  ValueType
		a, b string
	}{
		{"money key order and spacing", ValueMoney, `{ "currency": "UZS", "amount": "100.50" }`, `{"amount":"100.50","currency":"UZS"}`},
		{"large integer keeps precision", ValueInteger, `9007199254740993`, ` 9007199254740993 `},
		{"null", ValueDecimal, ` null`, `null`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			a, err := normalize(tt.typ, json.RawMessage(tt.a))
			require.NoError(t, err)
			b, err := normalize(tt.typ, json.RawMessage(tt.b))
			require.NoError(t, err)
			require.Equal(t, b, a)
		})
	}
}

func TestNormalize_RejectsTrailingData(t *testing.T) {
	t.Parallel()
	for _, raw := range []string{`"abc"}`, `{"amount":"1","currency":"UZS"}}`, `1 2`} {
		_, err := normalize(ValueString, json.RawMessage(raw))
		require.Error(t, err, raw)
	}
}

func TestUserActor_AllowlistsFields(t *testing.T) {
	t.Parallel()
	u, err := testUser(t).SetPassword("hashed-secret")
	require.NoError(t, err)
	data, err := json.Marshal(businessevent.UserActor(u))
	require.NoError(t, err)
	var keys map[string]any
	require.NoError(t, json.Unmarshal(data, &keys))
	require.Equal(t, map[string]any{
		"schema_version": float64(1), "kind": "user", "user_id": "7",
		"first_name": "Ali", "last_name": "Valiyev", "middle_name": "Karimovich",
	}, keys)
}

func TestRegistry_Register(t *testing.T) {
	t.Parallel()
	v2 := paymentDefinition()
	v2.Version = 2
	noTitle := paymentDefinition()
	noTitle.TitleKey = ""
	badType := paymentDefinition()
	badType.Type = "Payment Posted"
	noResources := paymentDefinition()
	noResources.Resources = nil
	duplicateField := paymentDefinition()
	duplicateField.Resources[0].Fields = append(duplicateField.Resources[0].Fields, duplicateField.Resources[0].Fields[0])
	unknownType := paymentDefinition()
	unknownType.Resources[0].Fields[0].Type = "json"
	tests := []struct {
		name    string
		defs    []Definition
		wantErr bool
	}{
		{"versions side by side", []Definition{paymentDefinition(), v2}, false},
		{"duplicate version", []Definition{paymentDefinition(), paymentDefinition()}, true},
		{"missing title", []Definition{noTitle}, true},
		{"malformed type", []Definition{badType}, true},
		{"no resources", []Definition{noResources}, true},
		{"duplicate field", []Definition{duplicateField}, true},
		{"unknown value type", []Definition{unknownType}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := NewRegistry().Register(tt.defs...)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
		})
	}
}
