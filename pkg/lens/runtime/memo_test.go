package runtime

import (
	"context"
	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/pkg/lens/resource"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMemoizeJSON_Scenario(t *testing.T) {
	t.Parallel()
	type report struct {
		Values []int `json:"values"`
	}
	for _, tt := range []struct {
		name string
		run  func(*testing.T, func(string) (report, error), *atomic.Int32)
	}{
		{"same scope returns an isolated clone", func(t *testing.T, load func(string) (report, error), calls *atomic.Int32) {
			t.Helper()
			one, err := load("a")
			require.NoError(t, err)
			one.Values[0] = 99
			two, err := load("a")
			require.NoError(t, err)
			require.Equal(t, []int{1, 2}, two.Values)
			require.Equal(t, int32(1), calls.Load())
		}},
		{"different scope recomputes", func(t *testing.T, load func(string) (report, error), calls *atomic.Int32) {
			t.Helper()
			_, err := load("a")
			require.NoError(t, err)
			_, err = load("b")
			require.NoError(t, err)
			require.Equal(t, int32(2), calls.Load())
		}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			runtime := New(Options{})
			var calls atomic.Int32
			load := func(scope string) (report, error) {
				return MemoizeJSON(context.Background(), runtime, MemoRequest{Namespace: "report", DataScope: scope, Input: map[string]any{"year": 2026}}, func(context.Context) (report, error) { calls.Add(1); return report{Values: []int{1, 2}}, nil })
			}
			tt.run(t, load, &calls)
		})
	}
}

func TestMemoizeJSONColdResultOwnsItsValues(t *testing.T) {
	// A warm-hit-only check misses the cold singleflight result's producer alias.
	type report struct {
		Values []int `json:"values"`
	}
	runtime := New(Options{})
	source := report{Values: []int{1, 2}}
	result, err := MemoizeJSON(t.Context(), runtime, MemoRequest{Namespace: "owned", DataScope: "tenant"}, func(context.Context) (report, error) { return source, nil })
	require.NoError(t, err)
	source.Values[0] = 99
	require.Equal(t, []int{1, 2}, result.Values)
}

type memoJoinedContext struct {
	context.Context
	joined chan struct{}
	once   sync.Once
}

func (c *memoJoinedContext) Done() <-chan struct{} {
	c.once.Do(func() { close(c.joined) })
	return c.Context.Done()
}
func TestMemoizeJSONCoalescedColdCallersAreIndependent(t *testing.T) {
	// Done is evaluated after DoChan registration, so both cold waiters really share one build.
	type report struct {
		Values []int `json:"values"`
	}
	runtime := New(Options{})
	started, release := make(chan struct{}), make(chan struct{})
	var calls atomic.Int32
	compute := func(context.Context) (report, error) {
		calls.Add(1)
		close(started)
		<-release
		return report{Values: []int{1, 2}}, nil
	}
	results := make(chan report, 2)
	errs := make(chan error, 2)
	req := MemoRequest{Namespace: "concurrent", DataScope: "tenant"}
	go func() { v, err := MemoizeJSON(t.Context(), runtime, req, compute); results <- v; errs <- err }()
	<-started
	joined := &memoJoinedContext{Context: t.Context(), joined: make(chan struct{})}
	go func() { v, err := MemoizeJSON(joined, runtime, req, compute); results <- v; errs <- err }()
	<-joined.joined
	close(release)
	one, two := <-results, <-results
	require.NoError(t, <-errs)
	require.NoError(t, <-errs)
	require.EqualValues(t, 1, calls.Load())
	one.Values[0] = 99
	require.Equal(t, []int{1, 2}, two.Values)
}

type hostileMemoInput struct{}

func (hostileMemoInput) MarshalJSON() ([]byte, error) { panic("identity encoder must not run") }
func TestMemoIdentityRejectsUnsafeAndLargeInputs(t *testing.T) {
	// Validating only computed output leaves the identity encoder as an allocation bypass.
	runtime := New(Options{})
	_, err := memoIdentity(runtime, MemoRequest{Namespace: "safe", DataScope: "tenant", Input: hostileMemoInput{}})
	require.Error(t, err)
	_, err = memoIdentity(runtime, MemoRequest{Namespace: "safe", DataScope: "tenant", Input: strings.Repeat("x", 64000)})
	require.ErrorIs(t, err, resource.ErrLimit)
	status, ok := resource.StatusCode(err)
	require.True(t, ok)
	require.Equal(t, 413, status)
}

func TestMemoizeJSONAcceptsNativeUUIDIdentityAndResult(t *testing.T) {
	t.Parallel()
	id := uuid.New()
	nullable := uuid.NullUUID{UUID: id, Valid: true}
	type report struct {
		ID       uuid.UUID     `json:"id"`
		Optional uuid.NullUUID `json:"optional"`
	}
	runtime := New(Options{})
	var calls atomic.Int32
	req := MemoRequest{Namespace: "native-uuid", DataScope: "tenant", Input: map[string]any{"id": id, "pointer": &id, "optional": nullable}}
	compute := func(context.Context) (report, error) { calls.Add(1); return report{ID: id, Optional: nullable}, nil }
	first, err := MemoizeJSON(t.Context(), runtime, req, compute)
	require.NoError(t, err)
	second, err := MemoizeJSON(t.Context(), runtime, req, compute)
	require.NoError(t, err)
	require.Equal(t, id, first.ID)
	require.Equal(t, nullable, second.Optional)
	require.EqualValues(t, 1, calls.Load())
}
