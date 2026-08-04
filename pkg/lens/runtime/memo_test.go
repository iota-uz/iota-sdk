package runtime

import (
	"context"
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
