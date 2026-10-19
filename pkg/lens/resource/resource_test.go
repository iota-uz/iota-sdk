package resource

import (
	"errors"
	"github.com/stretchr/testify/require"
	"sync"
	"testing"
)

type hostile struct{}

func (hostile) MarshalJSON() ([]byte, error) { panic("must reject before marshal") }
func TestEstimateRejectsCyclesAndCustomMarshal(t *testing.T) {
	// A shallow-only estimator would miss both nested cycles and an encoder bypass.
	cyclic := map[string]any{}
	cyclic["self"] = cyclic
	_, err := Estimate(cyclic, 10000)
	require.Error(t, err)
	_, err = Estimate(hostile{}, 10000)
	require.Error(t, err)
	shared := map[string][]int{"x": {1, 2}}
	_, err = Estimate([]any{shared, shared}, 10000)
	require.NoError(t, err)
	_, err = Estimate(make([]int, 1000), 100)
	require.ErrorIs(t, err, ErrLimit)
}
func TestBudgetReplacementAndRelease(t *testing.T) {
	// Reservation updates must work at full capacity and cannot resurrect released bytes.
	b := NewBudget(100)
	r, err := b.Reserve(100)
	require.NoError(t, err)
	_, err = b.Replace(r, 100)
	require.NoError(t, err)
	require.EqualValues(t, 100, b.Used())
	_, err = NewBudget(100).Replace(r, 1)
	require.Error(t, err)
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); _, _ = b.Replace(r, 50); r.Release() }()
	}
	wg.Wait()
	require.Zero(t, b.Used())
	_, err = b.Replace(r, 10)
	require.Error(t, err)
	r.Release()
	require.Zero(t, b.Used())
	_, err = b.Reserve(101)
	require.True(t, errors.Is(err, ErrLimit))
}
