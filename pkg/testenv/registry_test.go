package testenv

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

const testToken = "test-control-token-32-characters-long"

func testInput(scope string) Input {
	return Input{Name: "invoice", Version: "1", ScopeID: scope, Seed: "seed", Now: "2026-10-02T00:00:00Z", Params: map[string]any{"amount": float64(10)}}
}
func testDefinition() Definition {
	return Definition{Name: "invoice", Version: "1", Isolation: "dedicated", InputSchema: map[string]any{"type": "object", "required": []any{"amount"}, "properties": map[string]any{"amount": map[string]any{"type": "number", "minimum": 1}}, "additionalProperties": false}, OutputSchema: map[string]any{"type": "object", "required": []any{"id"}, "properties": map[string]any{"id": map[string]any{"type": "string"}}}}
}
func request(t *testing.T, handler http.Handler, method, path string, input any, token string) *httptest.ResponseRecorder {
	t.Helper()
	body, err := json.Marshal(input)
	require.NoError(t, err)
	req := httptest.NewRequest(method, path, bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	return w
}
func TestScenarioHTTPValidationAndReplay(t *testing.T) {
	// Falsely green if handlers are called directly and bypass credential/schema checks.
	r := NewRegistry(nil, true)
	var prepares, cleanups atomic.Int32
	require.NoError(t, r.Register(testDefinition(), func(_ context.Context, i Input) (Result, error) {
		prepares.Add(1)
		return Result{Data: map[string]any{"id": i.ScopeID}}, nil
	}, func(context.Context, string) error { cleanups.Add(1); return nil }))
	require.NoError(t, r.AllowScope("scope"))
	handler, err := NewHandler(r, testToken)
	require.NoError(t, err)
	require.Equal(t, 401, request(t, handler, "POST", "/__test__/scenarios/prepare", testInput("scope"), "wrong").Code)
	invalid := testInput("scope")
	invalid.Params["amount"] = float64(0)
	require.Equal(t, 400, request(t, handler, "POST", "/__test__/scenarios/prepare", invalid, testToken).Code)
	require.Zero(t, prepares.Load())
	first := request(t, handler, "POST", "/__test__/scenarios/prepare", testInput("scope"), testToken)
	require.Equal(t, 200, first.Code)
	replay := request(t, handler, "POST", "/__test__/scenarios/prepare", testInput("scope"), testToken)
	require.JSONEq(t, first.Body.String(), replay.Body.String())
	require.EqualValues(t, 1, prepares.Load())
	conflict := testInput("scope")
	conflict.Seed = "another"
	require.Equal(t, 409, request(t, handler, "POST", "/__test__/scenarios/prepare", conflict, testToken).Code)
	for range 2 {
		require.Equal(t, 200, request(t, handler, "DELETE", "/__test__/scopes/scope", nil, testToken).Code)
	}
	require.EqualValues(t, 1, cleanups.Load())
	require.Equal(t, 409, request(t, handler, "POST", "/__test__/scenarios/prepare", testInput("scope"), testToken).Code)
	require.Equal(t, 409, request(t, handler, "POST", "/__test__/scenarios/prepare", testInput("foreign"), testToken).Code)
}
func TestConcurrentPrepareAndCleanup(t *testing.T) {
	// Falsely green if concurrent requests are serialized by the test instead of the registry.
	r := NewRegistry(nil, true)
	var calls atomic.Int32
	gate := NewGate()
	require.NoError(t, r.Register(testDefinition(), func(ctx context.Context, i Input) (Result, error) {
		calls.Add(1)
		if err := gate.Wait(ctx); err != nil {
			return Result{}, err
		}
		return Result{Data: map[string]any{"id": i.ScopeID}}, nil
	}, func(context.Context, string) error { return nil }))
	require.NoError(t, r.AllowScope("one"))
	var wg sync.WaitGroup
	errs := make(chan error, 16)
	for range 16 {
		wg.Add(1)
		go func() { defer wg.Done(); _, err := r.Prepare(context.Background(), testInput("one")); errs <- err }()
	}
	<-gate.Entered()
	gate.Release()
	wg.Wait()
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}
	require.EqualValues(t, 1, calls.Load())
	result, err := r.Prepare(context.Background(), testInput("one"))
	require.NoError(t, err)
	result.Data["id"] = "mutated"
	replay, err := r.Prepare(context.Background(), testInput("one"))
	require.NoError(t, err)
	require.Equal(t, "one", replay.Data["id"])
}
func TestPartialFailureNeverBecomesReady(t *testing.T) {
	// Falsely green if only the preparation error is asserted and compensation is not observed.
	r := NewRegistry(nil, true)
	var cleanups atomic.Int32
	require.NoError(t, r.Register(testDefinition(), func(context.Context, Input) (Result, error) { return Result{}, errors.New("partial write") }, func(ctx context.Context, _ string) error { require.NoError(t, ctx.Err()); cleanups.Add(1); return nil }))
	require.NoError(t, r.AllowScope("failed"))
	_, err := r.Prepare(context.Background(), testInput("failed"))
	var control *Error
	require.ErrorAs(t, err, &control)
	require.Equal(t, "execution_failed", control.Code)
	require.EqualValues(t, 1, cleanups.Load())
	_, err = r.Prepare(context.Background(), testInput("failed"))
	require.ErrorAs(t, err, &control)
	require.Equal(t, "execution_failed", control.Code)
	require.NoError(t, r.Dispose(context.Background(), "failed"))
	require.EqualValues(t, 2, cleanups.Load())
}
func TestClockAndGateCancellation(t *testing.T) {
	// Falsely green if a wall-clock sleep determines when the boundary is crossed.
	start := time.Date(2026, 10, 2, 0, 0, 0, 0, time.UTC)
	clock := NewManualClock(start)
	require.NoError(t, clock.Advance(time.Hour))
	require.Equal(t, start.Add(time.Hour), clock.Now())
	require.Error(t, clock.Advance(-time.Second))
	gate := NewGate()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	require.ErrorIs(t, gate.Wait(ctx), context.Canceled)
	gate.Release()
	gate.Release()
	require.NoError(t, gate.Wait(context.Background()))
}

func TestNaturalGoSchemaAndValues(t *testing.T) {
	// Falsely green if every Go schema and value is preconverted to JSON by the caller.
	r := NewRegistry(nil, true)
	d := testDefinition()
	d.InputSchema = map[string]any{"type": "object", "required": []string{"tags"}, "properties": map[string]any{"tags": map[string]any{"type": "array", "items": map[string]any{"type": "string"}}}}
	d.OutputSchema = map[string]any{"type": "object", "required": []string{"count"}, "properties": map[string]any{"count": map[string]any{"type": "integer"}}}
	require.NoError(t, r.Register(d, func(context.Context, Input) (Result, error) { return Result{Data: map[string]any{"count": 2}}, nil }, func(context.Context, string) error { return nil }))
	require.NoError(t, r.AllowScope("native"))
	i := testInput("native")
	i.Params = map[string]any{"tags": []string{"one", "two"}}
	result, err := r.Prepare(context.Background(), i)
	require.NoError(t, err)
	require.Equal(t, float64(2), result.Data["count"])
}
