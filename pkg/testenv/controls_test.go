package testenv

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func controlCode(t *testing.T, err error) string {
	t.Helper()
	var control *Error
	require.ErrorAs(t, err, &control)
	return control.Code
}

func echoScenario(t *testing.T, r *Registry, isolation string) {
	t.Helper()
	d := testDefinition()
	d.Isolation = isolation
	require.NoError(t, r.Register(d, func(_ context.Context, i Input) (Result, error) {
		return Result{Refs: map[string]EntityRef{"invoice": {Kind: "invoice", ID: i.Seed}}, Data: map[string]any{"id": i.Seed}}, nil
	}, func(context.Context, string) error { return nil }))
}

func TestJobsRunRealHandlerWaitAndCancel(t *testing.T) {
	// Falsely green if Wait returned a scripted result instead of observing the
	// handler's own durable effect, or if a canceled wait hid a running job.
	r := NewRegistry(nil, false)
	echoScenario(t, r, "shared")
	jobs, err := NewJobs(r)
	require.NoError(t, err)
	var mu sync.Mutex
	posted := map[string]string{}
	gate := NewGate()
	require.NoError(t, jobs.Register(JobDefinition{Name: "post"}, func(ctx context.Context, req JobRequest) error {
		correlation, ok := CorrelationFrom(ctx)
		if !ok || correlation.ExecutionID != req.ExecutionID {
			return errors.New("missing correlation")
		}
		if err := gate.Wait(ctx); err != nil {
			return err
		}
		mu.Lock()
		defer mu.Unlock()
		posted[req.Refs["invoice"].ID] = correlation.ScopeID
		return nil
	}))
	require.NoError(t, jobs.Register(JobDefinition{Name: "broken"}, func(context.Context, JobRequest) error { return errors.New("ledger closed") }))
	require.NoError(t, jobs.Register(JobDefinition{Name: "global", Dedicated: true}, func(context.Context, JobRequest) error { return nil }))
	require.NoError(t, jobs.RegisterCondition("posted", func(_ context.Context, req JobRequest) (bool, error) {
		mu.Lock()
		defer mu.Unlock()
		return posted[req.Refs["invoice"].ID] == req.ScopeID, nil
	}))
	for _, id := range []string{"a", "b"} {
		require.NoError(t, r.AllowScope(id))
	}
	ctx := context.Background()
	refs := map[string]EntityRef{"invoice": {Kind: "invoice", ID: "42"}}

	_, err = jobs.Run(ctx, "global-scope", "post", refs)
	require.Equal(t, "scope_conflict", controlCode(t, err))
	_, err = jobs.Run(ctx, "a", "missing", refs)
	require.Equal(t, "unknown_scenario", controlCode(t, err))
	_, err = jobs.Run(ctx, "a", "global", refs)
	require.Equal(t, "missing_capability", controlCode(t, err), "shared environments refuse environment-wide jobs")

	execution, err := jobs.Run(ctx, "a", "post", refs)
	require.NoError(t, err)
	for _, timeout := range []int{0, 120001} {
		_, err = jobs.Wait(ctx, "a", execution, "posted", timeout)
		require.Equal(t, "invalid_input", controlCode(t, err))
	}
	_, err = jobs.Wait(ctx, "b", execution, "posted", 10)
	require.Equal(t, "scope_conflict", controlCode(t, err), "another scope cannot observe this execution")
	_, err = jobs.Wait(ctx, "a", execution, "nope", 10)
	require.Equal(t, "unknown_scenario", controlCode(t, err))

	canceled, cancel := context.WithCancel(ctx)
	cancel()
	result, err := jobs.Wait(canceled, "a", execution, "posted", 1000)
	require.NoError(t, err)
	require.Equal(t, "canceled", result.Outcome)
	require.Equal(t, "running", result.Execution.State, "cancellation reports the execution state")

	result, err = jobs.Wait(ctx, "a", execution, "posted", 5)
	require.Equal(t, "timeout", controlCode(t, err))
	require.Equal(t, "timeout", result.Outcome)
	require.Positive(t, result.Checks)

	gate.Release()
	result, err = jobs.Wait(ctx, "a", execution, "posted", 5000)
	require.NoError(t, err)
	require.Equal(t, "reached", result.Outcome)
	result, err = jobs.Wait(ctx, "a", execution, ConditionCompleted, 5000)
	require.NoError(t, err)
	require.Equal(t, "succeeded", result.Execution.State)

	failedID, err := jobs.Run(ctx, "b", "broken", refs)
	require.NoError(t, err)
	result, err = jobs.Wait(ctx, "b", failedID, "", 5000)
	require.Equal(t, "execution_failed", controlCode(t, err))
	require.Equal(t, "failed", result.Outcome)
	require.Equal(t, "ledger closed", result.Execution.Error)
}

func TestDisposeCancelsAndJoinsScopeJobs(t *testing.T) {
	// Falsely green if dispose returned while the job could still write after cleanup.
	r := NewRegistry(nil, true)
	echoScenario(t, r, "shared")
	jobs, err := NewJobs(r)
	require.NoError(t, err)
	exited := make(chan struct{})
	entered := make(chan struct{})
	require.NoError(t, jobs.Register(JobDefinition{Name: "long"}, func(ctx context.Context, _ JobRequest) error {
		defer close(exited)
		close(entered)
		<-ctx.Done()
		return ctx.Err()
	}))
	require.NoError(t, r.AllowScope("a"))
	id, err := jobs.Run(context.Background(), "a", "long", nil)
	require.NoError(t, err)
	<-entered
	require.NoError(t, r.Dispose(context.Background(), "a"))
	select {
	case <-exited:
	default:
		t.Fatal("dispose returned before the job exited")
	}
	_, err = jobs.Status("a", id)
	require.Equal(t, "scope_conflict", controlCode(t, err))
}

type paymentFake struct{ providers *Providers }

// Charge is what a consumer's fake provider adapter looks like: it asks the
// controls for the scripted step and maps outcomes onto its own protocol.
func (f paymentFake) Charge(ctx context.Context, key string) (string, error) {
	step, err := f.providers.Call(ctx, "payments", "charge", key)
	if err != nil {
		return "", err
	}
	if err := step.Err(); err != nil {
		return "", err
	}
	return step.Response["receipt"].(string), nil
}

func paymentProviders(t *testing.T, r *Registry) *Providers {
	t.Helper()
	providers, err := NewProviders(r)
	require.NoError(t, err)
	require.NoError(t, providers.Register(ProviderDefinition{Name: "payments", ResponseSchema: map[string]any{
		"type": "object", "required": []any{"receipt"}, "properties": map[string]any{"receipt": map[string]any{"type": "string"}}, "additionalProperties": false,
	}}))
	return providers
}

func TestProviderGatesReproduceRaceDeterministically(t *testing.T) {
	// Falsely green if the race depended on goroutine scheduling or sleeps:
	// the second charge must complete while the first is provably in flight.
	r := NewRegistry(nil, false)
	echoScenario(t, r, "shared")
	providers := paymentProviders(t, r)
	fake := paymentFake{providers}
	for _, id := range []string{"a", "b"} {
		require.NoError(t, r.AllowScope(id))
	}
	handler, err := NewHandler(r, testToken)
	require.NoError(t, err)
	invalid := map[string]any{"steps": []any{map[string]any{"outcome": "success", "response": map[string]any{"wrong": 1}}}}
	require.Equal(t, 400, request(t, handler, "POST", "/__test__/scopes/a/providers/payments/scripts", invalid, testToken).Code)
	script := ProviderScript{Steps: []ProviderStep{
		{Operation: "charge", Outcome: OutcomeSuccess, Hold: "first", Response: map[string]any{"receipt": "r-1"}},
		{Operation: "charge", Outcome: OutcomeRefusal},
		{Operation: "charge", Outcome: OutcomeTimeout},
	}}
	require.Equal(t, 200, request(t, handler, "POST", "/__test__/scopes/a/providers/payments/scripts", script, testToken).Code)
	require.NoError(t, providers.Script(context.Background(), "b", "payments", ProviderScript{Steps: []ProviderStep{{Outcome: OutcomeSuccess, Response: map[string]any{"receipt": "b-1"}}}}))

	scopeA := WithCorrelation(context.Background(), Correlation{ScopeID: "a", ExecutionID: "x1"})
	first := make(chan error, 1)
	var receipt string
	go func() {
		var err error
		receipt, err = fake.Charge(scopeA, "idem-1")
		first <- err
	}()
	require.Equal(t, 200, request(t, handler, "POST", "/__test__/scopes/a/providers/payments/gates/first/wait", map[string]int{"timeoutMs": 5000}, testToken).Code)
	_, err = fake.Charge(scopeA, "idem-1")
	require.ErrorIs(t, err, ErrProviderRefused, "second charge resolves while the first is held")
	_, err = fake.Charge(scopeA, "idem-1")
	require.ErrorIs(t, err, ErrProviderTimeout)
	_, err = fake.Charge(scopeA, "idem-1")
	require.ErrorIs(t, err, ErrUnscripted, "exhausted scripts fail instead of inventing success")
	_, err = fake.Charge(context.Background(), "idem-1")
	require.ErrorIs(t, err, ErrUncorrelated)

	calls, err := providers.Calls("a", "payments")
	require.NoError(t, err)
	outcomes := []string{}
	for _, call := range calls {
		outcomes = append(outcomes, call.Outcome)
		require.Equal(t, "a/x1", call.Correlation)
		require.Equal(t, "idem-1", call.Key)
	}
	require.Equal(t, []string{"held", OutcomeRefusal, OutcomeTimeout, "unscripted"}, outcomes)
	require.Equal(t, 200, request(t, handler, "POST", "/__test__/scopes/a/providers/payments/gates/first/release", nil, testToken).Code)
	require.NoError(t, <-first)
	require.Equal(t, "r-1", receipt)
	w := request(t, handler, "GET", "/__test__/scopes/a/providers/payments/calls", nil, testToken)
	require.Equal(t, 200, w.Code)
	var reply struct{ Calls []ProviderCall }
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &reply))
	require.Equal(t, OutcomeSuccess, reply.Calls[0].Outcome)
	require.Equal(t, 1, reply.Calls[0].Sequence)

	// Forget ends held calls and removes only this scope's scripts and calls.
	require.NoError(t, providers.Script(context.Background(), "a", "payments", ProviderScript{Steps: []ProviderStep{{Outcome: OutcomeSuccess, Hold: "never", Response: map[string]any{"receipt": "r-2"}}}}))
	held := make(chan error, 1)
	go func() { _, err := fake.Charge(scopeA, "idem-2"); held <- err }()
	require.NoError(t, providers.WaitHeld(context.Background(), "a", "payments", "never"))
	require.Equal(t, 200, request(t, handler, "DELETE", "/__test__/scopes/a/providers", nil, testToken).Code)
	require.Equal(t, "scope_conflict", controlCode(t, <-held))
	calls, err = providers.Calls("a", "payments")
	require.NoError(t, err)
	require.Empty(t, calls)
	receipt, err = fake.Charge(WithCorrelation(context.Background(), Correlation{ScopeID: "b"}), "other")
	require.NoError(t, err)
	require.Equal(t, "b-1", receipt, "another scope's script survives Forget")
}

func TestClockBoundaryWithoutWallClockSleep(t *testing.T) {
	// Falsely green if only browser time moved: the condition reads server time
	// through the injected clock as an explicit asOf, and no sleep is involved.
	r := NewRegistry(nil, true)
	clock := NewManualClock(time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC))
	r.UseClock(clock)
	echoScenario(t, r, "dedicated")
	jobs, err := NewJobs(r)
	require.NoError(t, err)
	var dueAt time.Time
	require.NoError(t, jobs.Register(JobDefinition{Name: "schedule"}, func(context.Context, JobRequest) error {
		dueAt = clock.Now().Add(24 * time.Hour)
		return nil
	}))
	require.NoError(t, jobs.RegisterCondition("overdue", func(context.Context, JobRequest) (bool, error) {
		asOf := clock.Now()
		return !asOf.Before(dueAt), nil
	}))
	for _, id := range []string{"lease", "other"} {
		require.NoError(t, r.AllowScope(id))
	}
	handler, err := NewHandler(r, testToken)
	require.NoError(t, err)
	require.Equal(t, 200, request(t, handler, "POST", "/__test__/scenarios/prepare", testInput("lease"), testToken).Code)
	require.Equal(t, "2026-10-02T00:00:00Z", clock.Now().Format(time.RFC3339), "dedicated preparation sets scenario time")
	id, err := jobs.Run(context.Background(), "lease", "schedule", nil)
	require.NoError(t, err)
	_, err = jobs.Wait(context.Background(), "lease", id, ConditionCompleted, 5000)
	require.NoError(t, err)
	_, err = jobs.Wait(context.Background(), "lease", id, "overdue", 1)
	require.Equal(t, "timeout", controlCode(t, err))

	require.Equal(t, 409, request(t, handler, "POST", "/__test__/scopes/other/clock/advance", map[string]int64{"durationMs": 1}, testToken).Code, "only the lease holder moves the clock")
	require.Equal(t, 400, request(t, handler, "POST", "/__test__/scopes/lease/clock/advance", map[string]int64{"durationMs": -1}, testToken).Code)
	w := request(t, handler, "POST", "/__test__/scopes/lease/clock/advance", map[string]int64{"durationMs": (24 * time.Hour).Milliseconds()}, testToken)
	require.Equal(t, 200, w.Code)
	require.JSONEq(t, `{"now":"2026-10-03T00:00:00Z"}`, w.Body.String())
	_, err = jobs.Wait(context.Background(), "lease", id, "overdue", 1000)
	require.NoError(t, err)

	shared := NewRegistry(nil, false)
	shared.UseClock(NewManualClock(time.Now()))
	echoScenario(t, shared, "shared")
	require.NoError(t, shared.AllowScope("s"))
	_, err = shared.AdvanceClock("s", time.Hour)
	require.Equal(t, "missing_capability", controlCode(t, err), "a shared environment refuses global clock control")
}

func TestScopeCleanupCoversFilesQueuesAndFakes(t *testing.T) {
	// Falsely green if cleanup were asserted only on DB rows: files, queue
	// namespaces and fake scripts would leak into the next scope.
	r := NewRegistry(nil, false)
	echoScenario(t, r, "shared")
	files, err := NewScopedFiles(r, t.TempDir())
	require.NoError(t, err)
	providers := paymentProviders(t, r)
	queues := map[string][]string{}
	var queueMu sync.Mutex
	failOnce := true
	require.NoError(t, r.OnDispose("queues", func(_ context.Context, scopeID string) error {
		queueMu.Lock()
		defer queueMu.Unlock()
		if failOnce && scopeID == "a" {
			failOnce = false
			return errors.New("stream busy")
		}
		delete(queues, QueueNamespace("jobs", scopeID))
		return nil
	}))
	for _, id := range []string{"a", "b"} {
		require.NoError(t, r.AllowScope(id))
		_, err := r.Prepare(context.Background(), testInput(id))
		require.NoError(t, err)
		dir, err := files.Dir(id)
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(filepath.Join(dir, "export.pdf"), []byte("%PDF"), 0o600))
		queues[QueueNamespace("jobs", id)] = []string{"msg"}
		require.NoError(t, providers.Script(context.Background(), id, "payments", ProviderScript{Steps: []ProviderStep{{Outcome: OutcomeSuccess, Response: map[string]any{"receipt": id}}}}))
	}
	_, err = files.Dir("../escape")
	require.Error(t, err)

	err = r.Dispose(context.Background(), "a")
	require.Equal(t, "cleanup_failed", controlCode(t, err))
	require.NoError(t, r.owned("a"), "a failed cleanup leaves the scope retryable, not disposed")
	require.NoError(t, r.Dispose(context.Background(), "a"))
	require.NoError(t, r.Dispose(context.Background(), "a"))

	dirA, _ := files.path("a")
	_, err = os.Stat(dirA)
	require.True(t, os.IsNotExist(err))
	dirB, err := files.Dir("b")
	require.NoError(t, err)
	_, err = os.Stat(filepath.Join(dirB, "export.pdf"))
	require.NoError(t, err)
	require.NotContains(t, queues, QueueNamespace("jobs", "a"))
	require.Contains(t, queues, QueueNamespace("jobs", "b"))
	_, err = providers.Calls("a", "payments")
	require.Equal(t, "scope_conflict", controlCode(t, err))
	snapshot, err := providers.SnapshotScope("b")
	require.NoError(t, err)
	require.NotEmpty(t, snapshot)
}

func TestManifestReplayRestoresScenarioAndControls(t *testing.T) {
	// Falsely green if replay only re-ran the scenario: provider scripts and the
	// advanced clock must be reproduced in a fresh environment as well.
	build := BuildInfo{BuildRevision: "rev-1", SchemaFingerprint: "schema-1", BaselineFingerprint: "base-1"}
	newEnvironment := func() (*Registry, *Providers, *ManualClock, http.Handler) {
		r := NewEnvironmentRegistry(nil, true, "env")
		r.SetBuildInfo(build)
		clock := NewManualClock(time.Unix(0, 0).UTC())
		r.UseClock(clock)
		echoScenario(t, r, "dedicated")
		providers := paymentProviders(t, r)
		handler, err := NewHandler(r, testToken)
		require.NoError(t, err)
		return r, providers, clock, handler
	}
	source, _, _, sourceHandler := newEnvironment()
	ctx := context.Background()
	require.NoError(t, source.ReserveScope("env-1"))
	original, err := source.Prepare(ctx, testInput("env-1"))
	require.NoError(t, err)
	require.Equal(t, 200, request(t, sourceHandler, "POST", "/__test__/scopes/env-1/providers/payments/scripts", ProviderScript{Steps: []ProviderStep{{Outcome: OutcomeRefusal}}}, testToken).Code)
	_, err = source.AdvanceClock("env-1", 90*time.Minute)
	require.NoError(t, err)
	w := request(t, sourceHandler, "GET", "/__test__/scopes/env-1/manifest", nil, testToken)
	require.Equal(t, 200, w.Code)
	require.NotContains(t, w.Body.String(), testToken, "manifests never carry the control credential")
	var manifest Manifest
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &manifest))
	require.Equal(t, "seed", manifest.Scenario.Seed)
	require.Equal(t, "2026-10-02T01:30:00Z", manifest.Controls.Clock)
	require.Equal(t, original.Refs, manifest.Result.Refs)

	fresh, providers, clock, handler := newEnvironment()
	require.Equal(t, 200, request(t, handler, "POST", "/__test__/scopes", map[string]string{"scopeId": "env-2"}, testToken).Code)
	for range 2 { // a repeated replay after a lost response re-applies nothing
		w = request(t, handler, "POST", "/__test__/scopes/env-2/replay", manifest, testToken)
		require.Equal(t, 200, w.Code, w.Body.String())
	}
	var replayed Result
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &replayed))
	require.Equal(t, "env-2", replayed.ScopeID)
	require.Equal(t, original.Data, replayed.Data)
	require.Equal(t, original.Refs, replayed.Refs)
	require.Equal(t, "2026-10-02T01:30:00Z", clock.Now().Format(time.RFC3339))
	fake := paymentFake{providers}
	scoped := WithCorrelation(ctx, Correlation{ScopeID: "env-2"})
	_, err = fake.Charge(scoped, "k")
	require.ErrorIs(t, err, ErrProviderRefused)
	_, err = fake.Charge(scoped, "k")
	require.ErrorIs(t, err, ErrUnscripted, "scripts were restored exactly once")

	drift := manifest
	drift.Environment.BaselineFingerprint = "base-2"
	require.NoError(t, fresh.ReserveScope("env-3"))
	_, err = fresh.Replay(ctx, "env-3", drift)
	require.Equal(t, "version_mismatch", controlCode(t, err))
	unknown := NewEnvironmentRegistry(nil, true, "env")
	echoScenario(t, unknown, "dedicated")
	require.NoError(t, unknown.ReserveScope("env-4"))
	_, err = unknown.Replay(ctx, "env-4", manifest)
	require.Equal(t, "version_mismatch", controlCode(t, err), "an environment that cannot prove its build refuses replay")
}
