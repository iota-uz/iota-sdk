package resource

import (
	"encoding/json"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"sync"
	"testing"
	"time"
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
	require.ErrorIs(t, err, ErrLimit)
}

type hostileText struct{}

func (hostileText) MarshalText() ([]byte, error) { panic("must reject before encoding") }
func TestEstimateRejectsCustomTextMarshal(t *testing.T) {
	// encoding/json can invoke TextMarshaler even when JSONMarshaler is absent.
	_, err := Estimate(hostileText{}, 10000)
	require.Error(t, err)
}

func TestEstimateJSONSkipsOpaqueNonSerializedState(t *testing.T) {
	// Serialization preflight may skip opaque private/ignored state; ownership preflight may not.
	value := struct {
		Public  string
		ignored func() `json:"-"`
		private map[string]string
	}{Public: "wire", ignored: func() {}, private: map[string]string{"private": "value"}}
	_, err := EstimateJSON(value, 10000)
	require.NoError(t, err)
	_, err = Estimate(value, 10000)
	require.Error(t, err)
}

func TestEstimateAcceptsBoundedNativeTransportValues(t *testing.T) {
	t.Parallel()
	id := uuid.New()
	nullable := uuid.NullUUID{UUID: id, Valid: true}
	values := []any{id, &id, nullable, &nullable, uuid.NullUUID{}, json.RawMessage(`{"ok":true}`), json.Number("123.5"), time.Now()}
	for _, value := range values {
		_, err := Estimate(value, 10000)
		require.NoError(t, err)
		_, err = EstimateJSON(value, 10000)
		require.NoError(t, err)
	}
	_, err := Estimate(hostile{}, 10000)
	require.Error(t, err)
	_, err = EstimateJSON(hostileText{}, 10000)
	require.Error(t, err)
}
