package document

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/iota-uz/iota-sdk/pkg/lens/frame"
	"github.com/iota-uz/iota-sdk/pkg/lens/resource"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestCloneActionPreservesEmptyParamsAsNonNil(t *testing.T) {
	t.Parallel()
	// A dynamic explorer child action (e.g. the product-detail leaf list) carries
	// no params. cloneAction must keep Params as an empty-but-non-nil slice so it
	// marshals as `[]`, never `null` — the wire contract's `params: array` rejects
	// null and bricks the whole level in the client.
	src := &Action{
		Kind:    ActionNavigateToLeaf,
		Params:  []ActionParam{},
		Payload: map[string]Source{},
	}
	clone := cloneAction(src)
	require.NotNil(t, clone.Params, "cloned Params must not collapse to nil")
	require.Empty(t, clone.Params)
	encoded, err := json.Marshal(clone)
	require.NoError(t, err)
	require.Contains(t, string(encoded), `"params":[]`)
	require.NotContains(t, string(encoded), `"params":null`)
}

func TestResolvedDynamicChildActionsMarshalEmptyParamsAsArray(t *testing.T) {
	t.Parallel()
	// Mirrors «Накопленная премия» → «По продуктам» → product-detail: a dynamic
	// leaf list whose child action carries no params. After resolution AND the
	// snapshot node clone, each child action must marshal `params: []`, never
	// `null` — the client's `action.params: array` schema rejects null.
	leaf := &Action{
		Kind:      ActionNavigateToLeaf,
		URLSource: &Source{Kind: ValueSourceField, Name: "action_url"},
		Params:    []ActionParam{},
		Payload:   map[string]Source{},
	}
	level := Level{
		Path: NodePath{"root/product-detail"},
		DynamicChildren: &DynamicChildren{
			Key:    Source{Kind: ValueSourceField, Name: "id"},
			Label:  Source{Kind: ValueSourceField, Name: "label"},
			Action: leaf,
		},
	}
	frame := Frame{
		Columns: []Column{
			{Name: "id", Type: ColumnString},
			{Name: "label", Type: ColumnString},
			{Name: "action_url", Type: ColumnString},
		},
		Rows: [][]any{{"osago", "OSAGO", "/portfolio/policies?product=osago"}},
	}
	require.NoError(t, ResolveDynamicChildren(&frame, level))
	// The snapshot round-trip clones nodes — the path that previously nilled Params.
	cloned := cloneNodes(frame.Children)
	require.NotEmpty(t, cloned)
	for _, child := range cloned {
		require.NotNil(t, child.Action)
		encoded, err := json.Marshal(child.Action)
		require.NoError(t, err)
		require.Contains(t, string(encoded), `"params":[]`)
		require.NotContains(t, string(encoded), `"params":null`)
	}
}

func TestMemoryStore_PutGetAppendAreCloneSafe(t *testing.T) {
	t.Parallel()
	store := NewMemoryStore(time.Hour, 2)
	snapshot := &Snapshot{
		ID:     "snapshot-1",
		Params: map[string]any{"filters": map[string]any{"tenant": "one"}},
		Frames: map[FrameRef]Frame{"root": testFrame(1)},
	}
	require.NoError(t, store.Put(context.Background(), snapshot))
	snapshot.Params["filters"].(map[string]any)["tenant"] = "mutated"
	snapshot.Frames["root"].Rows[0][0] = 99

	loaded, err := store.Get(context.Background(), "snapshot-1")
	require.NoError(t, err)
	require.Equal(t, "one", loaded.Params["filters"].(map[string]any)["tenant"])
	require.Equal(t, 1, loaded.Frames["root"].Rows[0][0])
	loaded.Params["filters"].(map[string]any)["tenant"] = "changed again"

	deeper := map[FrameRef]Frame{"detail": testFrame(2), "root": testFrame(3)}
	require.NoError(t, store.Append(context.Background(), "snapshot-1", deeper))
	deeper["detail"].Rows[0][0] = 100
	loaded, err = store.Get(context.Background(), "snapshot-1")
	require.NoError(t, err)
	require.Equal(t, "one", loaded.Params["filters"].(map[string]any)["tenant"])
	require.Equal(t, 1, loaded.Frames["root"].Rows[0][0], "append must not replace an already materialized frame")
	require.Equal(t, 2, loaded.Frames["detail"].Rows[0][0])

	calculatedAt := time.Date(2026, time.July, 19, 13, 0, 0, 0, time.UTC)
	panelFrame := testFrame(4)
	require.NoError(t, store.PutPanel(context.Background(), "snapshot-1", "panel-a", "panel:panel-a", panelFrame, PanelCalculation{
		DurationMS: 12, CalculatedAt: calculatedAt,
	}))
	panelFrame.Rows[0][0] = 101
	loaded, err = store.Get(context.Background(), "snapshot-1")
	require.NoError(t, err)
	require.Equal(t, 4, loaded.Frames["panel:panel-a"].Rows[0][0])
	require.Equal(t, int64(12), loaded.Panels["panel-a"].DurationMS)
	require.Equal(t, calculatedAt, loaded.Panels["panel-a"].CalculatedAt)
}

func TestMemoryStoreBoundsFramesPerSnapshot(t *testing.T) {
	t.Parallel()
	store := NewMemoryStore(time.Hour, 2)
	frames := make(map[FrameRef]Frame, maxFramesPerSnapshot)
	for index := range maxFramesPerSnapshot {
		frames[FrameRef(fmt.Sprintf("frame:%d", index))] = testFrame(index)
	}
	require.NoError(t, store.Put(t.Context(), &Snapshot{ID: "bounded", Frames: frames}))
	require.ErrorContains(t, store.Append(t.Context(), "bounded", map[FrameRef]Frame{"overflow": testFrame(1)}), "cannot exceed")
	frames["overflow"] = testFrame(1)
	require.ErrorContains(t, store.Put(t.Context(), &Snapshot{ID: "too-large", Frames: frames}), "cannot exceed")
}

func TestMemoryStore_ExpiryAndUnknownSnapshots(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, time.July, 19, 12, 0, 0, 0, time.UTC)
	store := NewMemoryStore(time.Minute, 2).(*memoryStore)
	store.clock = func() time.Time { return now }
	require.NoError(t, store.Put(context.Background(), &Snapshot{ID: "expiring"}))
	now = now.Add(time.Minute + time.Nanosecond)

	_, err := store.Get(context.Background(), "expiring")
	require.ErrorIs(t, err, ErrSnapshotGone)
	require.ErrorIs(t, store.Append(context.Background(), "expiring", map[FrameRef]Frame{"late": testFrame(1)}), ErrSnapshotGone)
	_, err = store.Get(context.Background(), "unknown")
	require.ErrorIs(t, err, ErrSnapshotGone)
}

func TestMemoryStore_TTLSlidesOnAccess(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, time.July, 19, 12, 0, 0, 0, time.UTC)
	store := NewMemoryStore(time.Minute, 3).(*memoryStore)
	store.clock = func() time.Time { return now }
	require.NoError(t, store.Put(context.Background(), &Snapshot{ID: "visited"}))
	require.NoError(t, store.Put(context.Background(), &Snapshot{ID: "appended"}))
	require.NoError(t, store.Put(context.Background(), &Snapshot{ID: "untouched"}))

	now = now.Add(45 * time.Second)
	_, err := store.Get(context.Background(), "visited")
	require.NoError(t, err)
	require.NoError(t, store.Append(context.Background(), "appended", map[FrameRef]Frame{"detail": testFrame(1)}))

	now = now.Add(30 * time.Second)
	_, err = store.Get(context.Background(), "visited")
	require.NoError(t, err)
	_, err = store.Get(context.Background(), "appended")
	require.NoError(t, err)
	_, err = store.Get(context.Background(), "untouched")
	require.ErrorIs(t, err, ErrSnapshotGone)
}

func TestMemoryStore_DefaultsAndBound(t *testing.T) {
	t.Parallel()
	defaults := NewMemoryStore(0, 0).(*memoryStore)
	require.Equal(t, 30*time.Minute, defaults.ttl)
	require.Equal(t, defaultMaxEntries, defaults.maxEntries)

	store := NewMemoryStore(time.Hour, 1)
	require.NoError(t, store.Put(context.Background(), &Snapshot{ID: "first"}))
	require.NoError(t, store.Put(context.Background(), &Snapshot{ID: "second"}))
	_, err := store.Get(context.Background(), "first")
	require.ErrorIs(t, err, ErrSnapshotGone)
	_, err = store.Get(context.Background(), "second")
	require.NoError(t, err)
}

func TestMemoryStore_ContextAndValidation(t *testing.T) {
	t.Parallel()
	store := NewMemoryStore(time.Hour, 1)
	require.Error(t, store.Put(context.Background(), nil))
	require.Error(t, store.Put(context.Background(), &Snapshot{}))

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	require.ErrorIs(t, store.Put(ctx, &Snapshot{ID: "canceled"}), context.Canceled)
	_, err := store.Get(ctx, "canceled")
	require.ErrorIs(t, err, context.Canceled)
}

func testFrame(value int) Frame {
	return Frame{Columns: []Column{{Name: "value", Type: ColumnNumber}}, Rows: [][]any{{value}}}
}

func TestMemoryStoreFrameMetadataIsCloneSafe(t *testing.T) {
	t.Parallel()
	for _, operation := range []string{"put", "append", "panel"} {
		t.Run(operation, func(t *testing.T) {
			store := NewMemoryStore(time.Hour, 1)
			total, enabled := 60.0, true
			original := Frame{
				Total: &total, Colors: []string{"#123456"},
				Presentation: &Presentation{
					Stack: true, Sortable: &enabled, Expandable: &enabled, Exportable: &enabled,
					LineSeries: []string{"forecast"},
				},
			}
			encoded, err := json.Marshal(original)
			require.NoError(t, err)
			var expected Frame
			require.NoError(t, json.Unmarshal(encoded, &expected))
			snapshot := &Snapshot{ID: "metadata"}
			if operation == "put" {
				snapshot.Frames = map[FrameRef]Frame{"chart": original}
			}
			require.NoError(t, store.Put(t.Context(), snapshot))
			switch operation {
			case "append":
				require.NoError(t, store.Append(t.Context(), snapshot.ID, map[FrameRef]Frame{"chart": original}))
			case "panel":
				require.NoError(t, store.PutPanel(t.Context(), snapshot.ID, "chart", "chart", original, PanelCalculation{}))
			}
			mutate := func(value Frame) {
				*value.Total = 99
				value.Colors[0] = "changed"
				value.Presentation.Stack = false
				*value.Presentation.Sortable = false
				*value.Presentation.Expandable = false
				*value.Presentation.Exportable = false
				value.Presentation.LineSeries[0] = "changed"
			}
			// Falsely green if only values are compared without mutating both sides of the store boundary.
			mutate(original)
			loaded, err := store.Get(t.Context(), snapshot.ID)
			require.NoError(t, err)
			assertMetadata := func(actual Frame) {
				require.Equal(t, expected.Total, actual.Total)
				require.Equal(t, expected.Colors, actual.Colors)
				require.Equal(t, expected.Presentation, actual.Presentation)
			}
			assertMetadata(loaded.Frames["chart"])
			mutate(loaded.Frames["chart"])
			again, err := store.Get(t.Context(), snapshot.ID)
			require.NoError(t, err)
			assertMetadata(again.Frames["chart"])
		})
	}
}

func TestMemoryStoreBudgetAndTypedOwnership(t *testing.T) {
	// Exercise public Put/Get with a retained input alias, not a clone helper alone.
	budget := resource.NewBudget(10000)
	store := NewMemoryStoreWithOptions(MemoryStoreOptions{Budget: budget, MaxEntryBytes: 4000})
	values := map[string][]string{"x": {"before"}}
	source := &Snapshot{ID: "owned", Params: map[string]any{"typed": values}}
	require.NoError(t, store.Put(t.Context(), source))
	used := budget.Used()
	require.Positive(t, used)
	values["x"][0] = "after"
	got, err := store.Get(t.Context(), "owned")
	require.NoError(t, err)
	require.Equal(t, "before", got.Params["typed"].(map[string][]string)["x"][0])
	require.NoError(t, store.Put(t.Context(), got))
	require.Equal(t, used, budget.Used())
	require.Error(t, store.Put(t.Context(), &Snapshot{ID: "owned", Params: map[string]any{"large": strings.Repeat("x", 1000)}}))
	got, err = store.Get(t.Context(), "owned")
	require.NoError(t, err)
	require.Equal(t, "before", got.Params["typed"].(map[string][]string)["x"][0])
	require.NoError(t, store.(SnapshotDeleter).Delete(t.Context(), "owned"))
	require.Zero(t, budget.Used())
}
func TestMemoryStorePutPanelRejectsOversizeAtomically(t *testing.T) {
	// A rejected incremental update must neither replace the old frame nor consume budget.
	budget := resource.NewBudget(100000)
	store := NewMemoryStoreWithOptions(MemoryStoreOptions{Budget: budget, MaxEntryBytes: 4000})
	require.NoError(t, store.Put(t.Context(), &Snapshot{ID: "x", Frames: map[FrameRef]Frame{"a": {Rows: [][]any{{"small"}}}}}))
	before := budget.Used()
	err := store.PutPanel(t.Context(), "x", "panel", "a", Frame{Rows: [][]any{{strings.Repeat("x", 1000)}}}, PanelCalculation{})
	require.Error(t, err)
	require.Equal(t, before, budget.Used())
	got, err := store.Get(t.Context(), "x")
	require.NoError(t, err)
	require.Equal(t, "small", got.Frames["a"].Rows[0][0])
}

func TestMemoryStoreTouchSlidesTTLAndExpires(t *testing.T) {
	// Touch must preserve active backing snapshots without returning cloned frame payloads.
	now := time.Now()
	store := NewMemoryStore(time.Minute, 2).(*memoryStore)
	store.clock = func() time.Time { return now }
	require.NoError(t, store.Put(t.Context(), &Snapshot{ID: "touch"}))
	now = now.Add(45 * time.Second)
	require.NoError(t, store.Touch(t.Context(), "touch"))
	now = now.Add(45 * time.Second)
	require.NoError(t, store.Touch(t.Context(), "touch"))
	now = now.Add(2 * time.Minute)
	require.ErrorIs(t, store.Touch(t.Context(), "touch"), ErrSnapshotGone)
	require.Zero(t, store.budget.Used())
}
func TestMemoryStorePutPanelFrameLimit(t *testing.T) {
	// The incremental API must enforce the same limit as initial Put and Append.
	frames := make(map[FrameRef]Frame, maxFramesPerSnapshot)
	for i := 0; i < maxFramesPerSnapshot; i++ {
		frames[FrameRef(fmt.Sprint(i))] = Frame{}
	}
	store := NewMemoryStore(time.Minute, 2)
	require.NoError(t, store.Put(t.Context(), &Snapshot{ID: "full", Frames: frames}))
	require.Error(t, store.PutPanel(t.Context(), "full", "panel", "overflow", Frame{}, PanelCalculation{}))
	got, err := store.Get(t.Context(), "full")
	require.NoError(t, err)
	require.Len(t, got.Frames, maxFramesPerSnapshot)
}

func TestMemoryStoreAdmissionEvictsAndSweepsExpired(t *testing.T) {
	// Expired entries never read again must release capacity during fresh admission.
	budget := resource.NewBudget(2000)
	store := NewMemoryStoreWithOptions(MemoryStoreOptions{Budget: budget, MaxEntryBytes: 2000, TTL: time.Minute}).(*memoryStore)
	now := time.Now()
	store.clock = func() time.Time { return now }
	snapshot := func(id string) *Snapshot {
		return &Snapshot{ID: id, Params: map[string]any{"x": strings.Repeat("x", 150)}}
	}
	require.NoError(t, store.Put(t.Context(), snapshot("a")))
	require.NoError(t, store.Put(t.Context(), snapshot("b")))
	_, err := store.Get(t.Context(), "a")
	require.ErrorIs(t, err, ErrSnapshotGone)
	now = now.Add(2 * time.Minute)
	require.NoError(t, store.Put(t.Context(), snapshot("c")))
	_, err = store.Get(t.Context(), "b")
	require.ErrorIs(t, err, ErrSnapshotGone)
	require.Len(t, store.items, 1)
}

func TestMemoryStoreClonesAcceptedMixedStructAndFrameParams(t *testing.T) {
	// Structural preflight permitting empty private metadata must still isolate public fields.
	type mixed struct {
		Public  map[string][]string
		private string
	}
	typed := mixed{Public: map[string][]string{"x": {"before"}}}
	require.Empty(t, typed.private)
	fr, err := frame.New("source", frame.Field{Name: "x", Type: frame.FieldTypeString, Values: []any{"before"}})
	require.NoError(t, err)
	store := NewMemoryStore(time.Minute, 2)
	require.NoError(t, store.Put(t.Context(), &Snapshot{ID: "mixed", Params: map[string]any{"mixed": typed, "frame": *fr}}))
	typed.Public["x"][0] = "after"
	fr.Fields[0].Values[0] = "after"
	got, err := store.Get(t.Context(), "mixed")
	require.NoError(t, err)
	require.Equal(t, "before", got.Params["mixed"].(mixed).Public["x"][0])
	require.Equal(t, "before", got.Params["frame"].(frame.Frame).Fields[0].Values[0])
}
