package runtime

import (
	"context"
	"strings"
	"testing"

	"github.com/iota-uz/iota-sdk/pkg/lens"
	"github.com/iota-uz/iota-sdk/pkg/lens/datasource"
	"github.com/iota-uz/iota-sdk/pkg/lens/frame"
	"github.com/iota-uz/iota-sdk/pkg/lens/resource"
	"github.com/stretchr/testify/require"
)

// Falsely green if only the store is tested: identity and memo preflights must
// honor the configured store limit before either value reaches retention.
func TestRuntimeHonorsStorePayloadLimit(t *testing.T) {
	t.Parallel()
	for _, operation := range []string{"identity", "memo"} {
		t.Run(operation, func(t *testing.T) {
			t.Parallel()
			budget := resource.NewBudget(64 << 20)
			runtime := New(Options{Store: NewMemorySnapshotStore(MemoryStoreOptions{Budget: budget, MaxEntryBytes: 32 << 20})})
			value := strings.Repeat("x", 3<<20)
			if operation == "identity" {
				fr, err := frame.New("large", frame.Field{Name: "label", Type: frame.FieldTypeString, Values: []any{value}})
				require.NoError(t, err)
				set, err := frame.NewFrameSet(fr)
				require.NoError(t, err)
				_, _, err = runtime.executionIdentity(lens.DashboardSpec{ID: "large", Datasets: []lens.DatasetSpec{{Name: "large", Kind: lens.DatasetKindStatic, Static: set}}}, Request{DataScope: "tenant"}, nil)
				require.NoError(t, err)
				return
			}
			calls := 0
			for range 2 {
				result, err := MemoizeJSON(t.Context(), runtime, MemoRequest{Namespace: "large", DataScope: "tenant"}, func(context.Context) (string, error) { calls++; return value, nil })
				require.NoError(t, err)
				require.Equal(t, value, result)
			}
			require.Equal(t, 1, calls, "large permitted reports must remain cacheable")
			require.Positive(t, budget.Used())
		})
	}
}

// Falsely green if increasing a host limit disables the smaller explicit or
// store bound: all three preflights must still return a typed payload refusal.
func TestRuntimePayloadPolicyKeepsLowerBounds(t *testing.T) {
	t.Parallel()
	for _, opts := range []Options{
		{MaxEntryBytes: 1024},
		{MaxEntryBytes: 4096, Store: NewMemorySnapshotStore(MemoryStoreOptions{MaxEntryBytes: 1024})},
		{MaxEntryBytes: 1024, Store: NewMemorySnapshotStore(MemoryStoreOptions{MaxEntryBytes: 4096})},
	} {
		runtime := New(opts)
		value := strings.Repeat("x", 1024)
		_, err := MemoizeJSON(t.Context(), runtime, MemoRequest{Namespace: "small", DataScope: "tenant"}, func(context.Context) (string, error) { return value, nil })
		assertPayloadLimit(t, err)
		_, _, err = runtime.executionIdentity(lens.DashboardSpec{ID: value}, Request{}, nil)
		assertPayloadLimit(t, err)
		_, err = runtime.queryCacheKey(datasource.QueryRequest{Text: value})
		assertPayloadLimit(t, err)
	}
}

func assertPayloadLimit(t *testing.T, err error) {
	t.Helper()
	var limit *resource.LimitError
	require.ErrorAs(t, err, &limit)
	require.Equal(t, resource.LimitPayload, limit.Kind)
	status, controlled := resource.StatusCode(err)
	require.True(t, controlled)
	require.Equal(t, 413, status)
}
