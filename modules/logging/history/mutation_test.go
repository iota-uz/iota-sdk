package history

import (
	"encoding/json"
	"math/big"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/logging/domain/aggregates/businessevent"
	"github.com/iota-uz/iota-sdk/pkg/fielddiff"
	"github.com/iota-uz/iota-sdk/pkg/money"
	"github.com/stretchr/testify/require"
)

func widgetDefinitions() []Definition {
	return Lifecycle("test.widget", "Widgets.History", Resource{Name: "test.widget", Fields: []Field{
		{Name: "title", LabelKey: "Widgets.Title", Type: ValueString},
		{Name: "price", LabelKey: "Widgets.Price", Type: ValueMoney},
		{Name: "due", LabelKey: "Widgets.Due", Type: ValueDate},
	}})
}

func TestRecordMutation_Lifecycle(t *testing.T) {
	t.Parallel()
	created := Snapshot{"title": "Lamp", "price": Money(money.New(1250, "USD")), "due": Date(time.Date(2026, 10, 28, 15, 0, 0, 0, time.UTC))}
	renamed := Snapshot{"title": "Desk lamp", "price": Money(money.New(1250, "USD")), "due": Date(time.Date(2026, 10, 28, 15, 0, 0, 0, time.UTC))}
	tests := []struct {
		name       string
		mutation   Mutation
		wantEvent  bool
		wantFields []fielddiff.Diff
	}{
		{
			name:      "created records every field as new",
			mutation:  Mutation{Type: "test.widget.created", Version: 1, Resource: "test.widget", ID: "1", After: created},
			wantEvent: true,
			wantFields: []fielddiff.Diff{
				{Field: "due", Before: fielddiff.Absent(), After: fielddiff.Present(json.RawMessage(`"2026-10-28"`))},
				{Field: "price", Before: fielddiff.Absent(), After: fielddiff.Present(json.RawMessage(`{"amount":"12.50","currency":"USD"}`))},
				{Field: "title", Before: fielddiff.Absent(), After: fielddiff.Present(json.RawMessage(`"Lamp"`))},
			},
		},
		{
			name:      "updated records only changed fields",
			mutation:  Mutation{Type: "test.widget.updated", Version: 1, Resource: "test.widget", ID: "1", Before: created, After: renamed},
			wantEvent: true,
			wantFields: []fielddiff.Diff{
				{Field: "title", Before: fielddiff.Present(json.RawMessage(`"Lamp"`)), After: fielddiff.Present(json.RawMessage(`"Desk lamp"`))},
			},
		},
		{
			name:     "update without changes records nothing",
			mutation: Mutation{Type: "test.widget.updated", Version: 1, Resource: "test.widget", ID: "1", Before: created, After: created},
		},
		{
			name:      "deleted keeps the last state",
			mutation:  Mutation{Type: "test.widget.deleted", Version: 1, Resource: "test.widget", ID: "1", Before: Snapshot{"title": "Lamp"}},
			wantEvent: true,
			wantFields: []fielddiff.Diff{
				{Field: "title", Before: fielddiff.Present(json.RawMessage(`"Lamp"`)), After: fielddiff.Absent()},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			repo := &memoryRepository{events: map[uuid.UUID]businessevent.Event{}}
			w := newTestWriter(t, repo)
			require.NoError(t, w.registry.Register(widgetDefinitions()...))
			ctx := testContext(t, &Execution{Channel: businessevent.ChannelUI, OperationID: "op-1"})

			require.NoError(t, RecordMutation(ctx, w, tt.mutation))

			if !tt.wantEvent {
				require.Empty(t, repo.events)
				return
			}
			require.Len(t, repo.events, 1)
			for _, e := range repo.events {
				require.Equal(t, tt.mutation.Type, e.Type)
				require.Equal(t, []businessevent.Subject{{Resource: "test.widget", ID: "1"}}, e.Subjects)
				require.Len(t, e.Changes, 1)
				require.Equal(t, tt.wantFields, e.Changes[0].Fields)
			}
		})
	}
}

func TestRecordMutation_DistinctUpdatesInOneOperation(t *testing.T) {
	t.Parallel()
	repo := &memoryRepository{events: map[uuid.UUID]businessevent.Event{}}
	w := newTestWriter(t, repo)
	require.NoError(t, w.registry.Register(widgetDefinitions()...))
	ctx := testContext(t, &Execution{Channel: businessevent.ChannelImport, OperationID: "op-1"})
	update := func(from, to string) Mutation {
		return Mutation{Type: "test.widget.updated", Version: 1, Resource: "test.widget", ID: "1", Before: Snapshot{"title": from}, After: Snapshot{"title": to}}
	}

	require.NoError(t, RecordMutation(ctx, w, update("a", "b")))
	require.NoError(t, RecordMutation(ctx, w, update("b", "c")))
	require.NoError(t, RecordMutation(ctx, w, update("b", "c")), "a retried update is the same event")

	require.Len(t, repo.events, 2)
}

func TestMinorUnits_Format(t *testing.T) {
	t.Parallel()
	tests := []struct {
		amount   int64
		fraction int
		want     string
	}{
		{1250, 2, "12.50"},
		{5, 2, "0.05"},
		{-5, 2, "-0.05"},
		{0, 2, "0.00"},
		{1000, 0, "1000"},
		{-123456, 3, "-123.456"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tt.want, MinorUnits(big.NewInt(tt.amount), tt.fraction))
		})
	}
}
