package runtime

import (
	"context"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMemoizeJSONReusesCloneSafeScopedValue(t *testing.T) {
	t.Parallel()
	runtime := New(Options{})
	var calls atomic.Int32
	type report struct {
		Values []int `json:"values"`
	}
	load := func(scope string) (report, error) {
		return MemoizeJSON(context.Background(), runtime, MemoRequest{Namespace: "report", DataScope: scope, Input: map[string]any{"year": 2026}}, func(context.Context) (report, error) { calls.Add(1); return report{Values: []int{1, 2}}, nil })
	}
	one, err := load("a")
	require.NoError(t, err)
	one.Values[0] = 99
	two, err := load("a")
	require.NoError(t, err)
	require.Equal(t, []int{1, 2}, two.Values)
	_, err = load("b")
	require.NoError(t, err)
	require.Equal(t, int32(2), calls.Load())
}
