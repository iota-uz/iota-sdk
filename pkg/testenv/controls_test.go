package testenv

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
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
	require.NoError(t, providers.Register(ProviderDefinition{Name: "payments", CredentialFree: true, ResponseSchema: map[string]any{
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
	var reply struct {
		Calls []ProviderCall `json:"calls"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &reply))
	require.Equal(t, OutcomeSuccess, reply.Calls[0].Outcome)
	require.Equal(t, 1, reply.Calls[0].Sequence)

	// Forget ends held calls and removes only this scope's scripts and calls.
	require.NoError(t, providers.Script(context.Background(), "a", "payments", ProviderScript{Steps: []ProviderStep{{Outcome: OutcomeSuccess, Hold: "never", Response: map[string]any{"receipt": "r-2"}}}}))
	bounded, stop := context.WithTimeout(context.Background(), 5*time.Second)
	defer stop()
	held := make(chan error, 1)
	go func() {
		_, err := fake.Charge(WithCorrelation(bounded, Correlation{ScopeID: "a", ExecutionID: "x1"}), "idem-2")
		held <- err
	}()
	require.NoError(t, providers.WaitHeld(bounded, "a", "payments", "never"), "the call reaches the gate")
	require.Equal(t, 200, request(t, handler, "DELETE", "/__test__/scopes/a/providers", nil, testToken).Code)
	select {
	case err := <-held:
		require.Equal(t, "scope_conflict", controlCode(t, err), "Forget ends the held call")
	case <-bounded.Done():
		t.Fatal("Forget did not end the held call")
	}
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
	require.Equal(t, "scope_conflict", controlCode(t, r.owned("a")), "admission closes before cleanup and stays closed")
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
	w = request(t, handler, "POST", "/__test__/scopes/env-2/replay", manifest, testToken)
	require.Equal(t, 200, w.Code, w.Body.String())
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
	_, err = fresh.AdvanceClock("env-2", time.Hour)
	require.NoError(t, err)
	// A repeated replay after a lost response re-applies nothing: the consumed
	// script stays consumed and the advanced clock is not reset.
	retry := request(t, handler, "POST", "/__test__/scopes/env-2/replay", manifest, testToken)
	require.Equal(t, 200, retry.Code, retry.Body.String())
	require.JSONEq(t, w.Body.String(), retry.Body.String())
	_, err = fake.Charge(scoped, "k")
	require.ErrorIs(t, err, ErrUnscripted, "scripts were restored exactly once")
	require.Equal(t, "2026-10-02T02:30:00Z", clock.Now().Format(time.RFC3339))

	// Replay never lands in a scope that was prepared otherwise.
	require.NoError(t, fresh.ReserveScope("env-6"))
	_, err = fresh.Prepare(ctx, func() Input { i := manifest.Scenario; i.ScopeID = "env-6"; return i }())
	require.Equal(t, "scope_conflict", controlCode(t, err), "env-2 holds the dedicated lease")
	require.NoError(t, fresh.Dispose(ctx, "env-2"))
	_, err = fresh.Prepare(ctx, func() Input { i := manifest.Scenario; i.ScopeID = "env-6"; return i }())
	require.NoError(t, err)
	_, err = fresh.Replay(ctx, "env-6", manifest)
	require.Equal(t, "scope_conflict", controlCode(t, err))
	require.NoError(t, fresh.Dispose(ctx, "env-6"))

	// A control that cannot be restored compensates the replayed scope; it is
	// never handed out as ready afterwards.
	strict := NewEnvironmentRegistry(nil, true, "env")
	strict.SetBuildInfo(build)
	strict.UseClock(NewManualClock(time.Unix(0, 0).UTC()))
	// The before phase succeeds, the callback creates a fixture, and the after
	// phase fails: compensation must remove the fixture the callback created.
	fixtures := map[string]bool{}
	strictDefinition := testDefinition()
	require.NoError(t, strict.Register(strictDefinition, func(_ context.Context, i Input) (Result, error) {
		fixtures[i.ScopeID] = true
		return Result{Refs: map[string]EntityRef{"invoice": {Kind: "invoice", ID: i.Seed}}, Data: map[string]any{"id": i.Seed}}, nil
	}, func(_ context.Context, scope string) error { delete(fixtures, scope); return nil }))
	restore := &failingRestore{}
	require.NoError(t, strict.Attach("providers", restore))
	require.NoError(t, strict.ReserveScope("env-5"))
	_, err = strict.Replay(ctx, "env-5", manifest)
	require.Equal(t, "execution_failed", controlCode(t, err))
	require.Equal(t, []RestorePhase{RestoreBeforePrepare, RestoreAfterPrepare}, restore.phases, "the callback ran between the phases")
	require.Empty(t, fixtures, "records the callback prepared are compensated")
	_, err = strict.Prepare(ctx, func() Input { i := manifest.Scenario; i.ScopeID = "env-5"; return i }())
	require.Equal(t, "execution_failed", controlCode(t, err), "the half-restored scope is not ready")
	_, err = strict.Manifest(ctx, "env-5")
	require.Equal(t, "scope_conflict", controlCode(t, err))
	require.NoError(t, strict.Dispose(ctx, "env-5"))

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
	blank := manifest
	blank.Environment = BuildInfo{}
	require.NoError(t, fresh.ReserveScope("env-7"))
	_, err = fresh.Replay(ctx, "env-7", blank)
	require.Equal(t, "version_mismatch", controlCode(t, err), "an empty recorded fingerprint is not a wildcard")
}

func TestReplayRestoresScriptsTheScenarioConsumes(t *testing.T) {
	// Falsely green if replay restored every script after the callback: a
	// scenario that calls its provider during preparation would then fail.
	build := BuildInfo{BuildRevision: "r", SchemaFingerprint: "s", BaselineFingerprint: "b"}
	newEnvironment := func() (*Registry, *Providers) {
		r := NewEnvironmentRegistry(nil, false, "env")
		r.SetBuildInfo(build)
		providers := paymentProviders(t, r)
		d := testDefinition()
		d.Isolation = "shared"
		require.NoError(t, r.Register(d, func(ctx context.Context, i Input) (Result, error) {
			receipt, err := paymentFake{providers}.Charge(ctx, "setup")
			if err != nil {
				return Result{}, err
			}
			return Result{Data: map[string]any{"id": receipt}}, nil
		}, func(context.Context, string) error { return nil }))
		return r, providers
	}
	ctx := context.Background()
	source, providers := newEnvironment()
	require.NoError(t, source.ReserveScope("env-1"))
	require.NoError(t, providers.Script(ctx, "env-1", "payments", ProviderScript{Steps: []ProviderStep{{Outcome: OutcomeSuccess, Response: map[string]any{"receipt": "setup-1"}}}}))
	_, err := source.Prepare(ctx, testInput("env-1"))
	require.NoError(t, err)
	require.NoError(t, providers.Script(ctx, "env-1", "payments", ProviderScript{Steps: []ProviderStep{{Outcome: OutcomeRefusal}}}))
	manifest, err := source.Manifest(ctx, "env-1")
	require.NoError(t, err)

	fresh, freshProviders := newEnvironment()
	require.NoError(t, fresh.ReserveScope("env-2"))
	result, err := fresh.Replay(ctx, "env-2", manifest)
	require.NoError(t, err)
	require.Equal(t, "setup-1", result.Data["id"])
	_, err = paymentFake{freshProviders}.Charge(WithCorrelation(ctx, Correlation{ScopeID: "env-2"}), "later")
	require.ErrorIs(t, err, ErrProviderRefused, "post-preparation scripts are restored after the callback")
}

func TestReviewRegressions(t *testing.T) {
	ctx := context.Background()
	t.Run("success steps require a schema-valid response", func(t *testing.T) {
		// Falsely green if a success step without response reached a fake that
		// reads it: the fake would panic instead of the script being refused.
		r := NewRegistry(nil, false)
		providers := paymentProviders(t, r)
		require.NoError(t, r.AllowScope("a"))
		err := providers.Script(ctx, "a", "payments", ProviderScript{Steps: []ProviderStep{{Outcome: OutcomeSuccess}}})
		require.Equal(t, "invalid_input", controlCode(t, err))
		require.NoError(t, providers.Script(ctx, "a", "payments", ProviderScript{Steps: []ProviderStep{{Outcome: OutcomeTimeout}}}))
	})
	t.Run("discovery cannot mutate validation", func(t *testing.T) {
		r := NewRegistry(nil, false)
		providers := paymentProviders(t, r)
		definitions := providers.Definitions()
		definitions[0]["payloadSchema"].(map[string]any)["properties"] = nil
		require.NotNil(t, providers.Definitions()[0]["payloadSchema"].(map[string]any)["properties"])
	})
	t.Run("unclassified providers are never exported", func(t *testing.T) {
		// Falsely green if export were opt-out: an unmarked provider could
		// serialize a scripted access token into the manifest.
		r := NewRegistry(nil, false)
		echoScenario(t, r, "shared")
		providers, err := NewProviders(r)
		require.NoError(t, err)
		require.NoError(t, providers.Register(ProviderDefinition{Name: "auth"}))
		require.NoError(t, r.AllowScope("a"))
		_, err = r.Prepare(ctx, testInput("a"))
		require.NoError(t, err)
		require.NoError(t, providers.Script(ctx, "a", "auth", ProviderScript{Steps: []ProviderStep{{Outcome: OutcomeSuccess, Response: map[string]any{"accessToken": "secret-token"}}}}))
		_, err = r.Manifest(ctx, "a")
		require.Equal(t, "invalid_input", controlCode(t, err))
	})
	t.Run("empty typed responses survive encoding", func(t *testing.T) {
		r := NewRegistry(nil, false)
		providers, err := NewProviders(r)
		require.NoError(t, err)
		require.NoError(t, providers.Register(ProviderDefinition{Name: "any", CredentialFree: true}))
		require.NoError(t, r.AllowScope("a"))
		require.NoError(t, providers.Script(ctx, "a", "any", ProviderScript{Steps: []ProviderStep{{Outcome: OutcomeSuccess, Response: map[string]any{}}}}))
	})
	t.Run("a released gate is not reported as held", func(t *testing.T) {
		r := NewRegistry(nil, false)
		providers := paymentProviders(t, r)
		require.NoError(t, r.AllowScope("a"))
		require.NoError(t, providers.Script(ctx, "a", "payments", ProviderScript{Steps: []ProviderStep{{Outcome: OutcomeRefusal, Hold: "g"}}}))
		providers.mu.Lock() // freeze the call between release and decrement
		state := providers.stateLocked("a", "payments")
		state.holding["g"] = 1
		state.gates["g"] = NewGate()
		state.gates["g"].Release()
		providers.mu.Unlock()
		short, stop := context.WithTimeout(ctx, 20*time.Millisecond)
		defer stop()
		require.Equal(t, "timeout", controlCode(t, providers.WaitHeld(short, "a", "payments", "g")))
	})
	t.Run("scripts the scenario submits are recreated, not replayed", func(t *testing.T) {
		// Falsely green if replay restored the callback's own script on top of
		// the one the callback submits again: replay would have two refusals.
		build := BuildInfo{BuildRevision: "r", SchemaFingerprint: "s", BaselineFingerprint: "b"}
		newEnvironment := func() (*Registry, *Providers) {
			r := NewEnvironmentRegistry(nil, false, "env")
			r.SetBuildInfo(build)
			providers := paymentProviders(t, r)
			d := testDefinition()
			d.Isolation = "shared"
			require.NoError(t, r.Register(d, func(ctx context.Context, i Input) (Result, error) {
				err := providers.Script(ctx, i.ScopeID, "payments", ProviderScript{Steps: []ProviderStep{{Outcome: OutcomeRefusal}}})
				return Result{Data: map[string]any{"id": i.Seed}}, err
			}, func(context.Context, string) error { return nil }))
			return r, providers
		}
		source, _ := newEnvironment()
		require.NoError(t, source.ReserveScope("env-1"))
		_, err := source.Prepare(ctx, testInput("env-1"))
		require.NoError(t, err)
		manifest, err := source.Manifest(ctx, "env-1")
		require.NoError(t, err)
		require.Empty(t, manifest.Controls.Resources)
		fresh, providers := newEnvironment()
		require.NoError(t, fresh.ReserveScope("env-2"))
		_, err = fresh.Replay(ctx, "env-2", manifest)
		require.NoError(t, err)
		scoped := WithCorrelation(ctx, Correlation{ScopeID: "env-2"})
		_, err = paymentFake{providers}.Charge(scoped, "k")
		require.ErrorIs(t, err, ErrProviderRefused)
		_, err = paymentFake{providers}.Charge(scoped, "k")
		require.ErrorIs(t, err, ErrUnscripted, "same queue as the source")
	})
	t.Run("dispose unblocks gated jobs before joining them", func(t *testing.T) {
		// Falsely green if the job's fake call honored job cancellation: here
		// it uses a detached context, so only the gate interrupt frees it.
		r := NewRegistry(nil, false)
		providers := paymentProviders(t, r)
		jobs, err := NewJobs(r)
		require.NoError(t, err)
		require.NoError(t, jobs.Register(JobDefinition{Name: "charge"}, func(ctx context.Context, req JobRequest) error {
			detached := WithCorrelation(context.WithoutCancel(ctx), Correlation{ScopeID: req.ScopeID})
			_, err := paymentFake{providers}.Charge(detached, "k")
			return err
		}))
		require.NoError(t, r.AllowScope("a"))
		require.NoError(t, providers.Script(ctx, "a", "payments", ProviderScript{Steps: []ProviderStep{{Outcome: OutcomeRefusal, Hold: "g"}}}))
		_, err = jobs.Run(ctx, "a", "charge", nil)
		require.NoError(t, err)
		bounded, stop := context.WithTimeout(ctx, 5*time.Second)
		defer stop()
		require.NoError(t, providers.WaitHeld(bounded, "a", "payments", "g"))
		require.NoError(t, r.Dispose(bounded, "a"))
	})
	t.Run("a wait ends when its scope is disposed", func(t *testing.T) {
		r := NewRegistry(nil, false)
		jobs, err := NewJobs(r)
		require.NoError(t, err)
		entered := make(chan struct{})
		var once sync.Once
		require.NoError(t, jobs.Register(JobDefinition{Name: "idle"}, func(ctx context.Context, _ JobRequest) error { <-ctx.Done(); return nil }))
		require.NoError(t, jobs.RegisterCondition("paused", func(ctx context.Context, _ JobRequest) (bool, error) {
			once.Do(func() { close(entered) })
			<-ctx.Done() // a check paused on a record cleanup is removing
			return true, nil
		}))
		require.NoError(t, r.AllowScope("a"))
		id, err := jobs.Run(ctx, "a", "idle", nil)
		require.NoError(t, err)
		waited := make(chan error, 1)
		go func() { _, err := jobs.Wait(ctx, "a", id, "paused", 60000); waited <- err }()
		<-entered
		require.NoError(t, r.Dispose(ctx, "a"))
		select {
		case err := <-waited:
			require.Equal(t, "scope_conflict", controlCode(t, err), "never reached after disposal")
		case <-time.After(5 * time.Second):
			t.Fatal("wait outlived its scope")
		}
	})
	t.Run("caller cancellation is not reported as timeout", func(t *testing.T) {
		r := NewRegistry(nil, false)
		jobs, err := NewJobs(r)
		require.NoError(t, err)
		require.NoError(t, jobs.Register(JobDefinition{Name: "idle"}, func(ctx context.Context, _ JobRequest) error { <-ctx.Done(); return nil }))
		require.NoError(t, r.AllowScope("a"))
		id, err := jobs.Run(ctx, "a", "idle", nil)
		require.NoError(t, err)
		for range 50 {
			canceled, cancel := context.WithCancel(ctx)
			go cancel()
			result, err := jobs.Wait(canceled, "a", id, ConditionCompleted, 1)
			if result.Outcome != "canceled" {
				require.Equal(t, "timeout", result.Outcome)
				require.Error(t, err)
				continue
			}
			require.NoError(t, err)
		}
		canceled, cancel := context.WithCancel(ctx)
		cancel()
		result, err := jobs.Wait(canceled, "a", id, ConditionCompleted, 1)
		require.NoError(t, err)
		require.Equal(t, "canceled", result.Outcome)
		require.NoError(t, r.Dispose(ctx, "a"))
	})
	t.Run("no job starts during failed-preparation cleanup", func(t *testing.T) {
		// Falsely green if admission stayed open during compensation: a job
		// started from inside cleanup could write after the records are gone.
		r := NewRegistry(nil, false)
		jobs, err := NewJobs(r)
		require.NoError(t, err)
		require.NoError(t, jobs.Register(JobDefinition{Name: "late"}, func(context.Context, JobRequest) error { return nil }))
		var lateErr error
		d := testDefinition()
		d.Isolation = "shared"
		require.NoError(t, r.Register(d, func(context.Context, Input) (Result, error) { return Result{}, errors.New("boom") },
			func(ctx context.Context, scope string) error {
				_, lateErr = jobs.Run(ctx, scope, "late", nil)
				return nil
			}))
		require.NoError(t, r.AllowScope("a"))
		_, err = r.Prepare(ctx, testInput("a"))
		require.Equal(t, "execution_failed", controlCode(t, err))
		require.Equal(t, "scope_conflict", controlCode(t, lateErr))
	})
	t.Run("parent cancellation between checks is canceled", func(t *testing.T) {
		r := NewRegistry(nil, false)
		jobs, err := NewJobs(r)
		require.NoError(t, err)
		require.NoError(t, jobs.Register(JobDefinition{Name: "idle"}, func(ctx context.Context, _ JobRequest) error { <-ctx.Done(); return nil }))
		checked := make(chan struct{}, 1)
		require.NoError(t, jobs.RegisterCondition("never", func(context.Context, JobRequest) (bool, error) {
			select {
			case checked <- struct{}{}:
			default:
			}
			return false, nil
		}))
		require.NoError(t, r.AllowScope("a"))
		id, err := jobs.Run(ctx, "a", "idle", nil)
		require.NoError(t, err)
		parent, cancel := context.WithCancel(ctx)
		waited := make(chan WaitResult, 1)
		go func() { result, _ := jobs.Wait(parent, "a", id, "never", 60000); waited <- result }()
		<-checked // the wait is now idle between checks
		cancel()
		require.Equal(t, "canceled", (<-waited).Outcome)
		require.NoError(t, r.Dispose(ctx, "a"))
	})
	t.Run("scope directories stay short", func(t *testing.T) {
		r := NewEnvironmentRegistry(nil, false, "env")
		files, err := NewScopedFiles(r, t.TempDir())
		require.NoError(t, err)
		long := "env-" + strings.Repeat("x", 250)
		require.NoError(t, r.ReserveScope(long))
		dir, err := files.Dir(long)
		require.NoError(t, err)
		require.LessOrEqual(t, len(filepath.Base(dir)), 80)
	})
	t.Run("scenario cleanup is not repeated after it succeeded", func(t *testing.T) {
		r := NewRegistry(nil, false)
		var cleanups int
		d := testDefinition()
		d.Isolation = "shared"
		require.NoError(t, r.Register(d, func(_ context.Context, i Input) (Result, error) {
			return Result{Data: map[string]any{"id": i.Seed}}, nil
		}, func(context.Context, string) error {
			cleanups++
			if cleanups > 1 {
				return errors.New("fixture already removed")
			}
			return nil
		}))
		failOnce := true
		require.NoError(t, r.OnDispose("queues", func(context.Context, string) error {
			if failOnce {
				failOnce = false
				return errors.New("stream busy")
			}
			return nil
		}))
		require.NoError(t, r.AllowScope("a"))
		_, err := r.Prepare(ctx, testInput("a"))
		require.NoError(t, err)
		require.Equal(t, "cleanup_failed", controlCode(t, r.Dispose(ctx, "a")))
		require.NoError(t, r.Dispose(ctx, "a"))
		require.Equal(t, 1, cleanups)
	})
	t.Run("oversized manifests are refused at export", func(t *testing.T) {
		r := NewRegistry(nil, false)
		echoScenario(t, r, "shared")
		providers := paymentProviders(t, r)
		require.NoError(t, r.AllowScope("a"))
		_, err := r.Prepare(ctx, testInput("a"))
		require.NoError(t, err)
		big := strings.Repeat("x", 768<<10)
		for range 6 {
			require.NoError(t, providers.Script(ctx, "a", "payments", ProviderScript{Steps: []ProviderStep{{Outcome: OutcomeSuccess, Response: map[string]any{"receipt": big}}}}))
		}
		_, err = r.Manifest(ctx, "a")
		require.Equal(t, "invalid_input", controlCode(t, err))
	})
	t.Run("a canceled hold is no longer reported as held", func(t *testing.T) {
		r := NewRegistry(nil, false)
		providers := paymentProviders(t, r)
		require.NoError(t, r.AllowScope("a"))
		require.NoError(t, providers.Script(ctx, "a", "payments", ProviderScript{Steps: []ProviderStep{{Outcome: OutcomeRefusal, Hold: "g"}}}))
		callCtx, cancel := context.WithCancel(WithCorrelation(ctx, Correlation{ScopeID: "a"}))
		done := make(chan error, 1)
		go func() { _, err := providers.Call(callCtx, "payments", "charge", "k"); done <- err }()
		bounded, stop := context.WithTimeout(ctx, 5*time.Second)
		defer stop()
		require.NoError(t, providers.WaitHeld(bounded, "a", "payments", "g"))
		cancel()
		require.ErrorIs(t, <-done, context.Canceled)
		short, stopShort := context.WithTimeout(ctx, 20*time.Millisecond)
		defer stopShort()
		require.Equal(t, "timeout", controlCode(t, providers.WaitHeld(short, "a", "payments", "g")))
	})
	t.Run("released-before-arrival passes without holding", func(t *testing.T) {
		r := NewRegistry(nil, false)
		providers := paymentProviders(t, r)
		require.NoError(t, r.AllowScope("a"))
		require.NoError(t, providers.Release("a", "payments", "g"))
		require.NoError(t, providers.Script(ctx, "a", "payments", ProviderScript{Steps: []ProviderStep{{Outcome: OutcomeRefusal, Hold: "g"}}}))
		_, err := paymentFake{providers}.Charge(WithCorrelation(ctx, Correlation{ScopeID: "a"}), "k")
		require.ErrorIs(t, err, ErrProviderRefused)
		calls, err := providers.Calls("a", "payments")
		require.NoError(t, err)
		require.Equal(t, OutcomeRefusal, calls[0].Outcome)
	})
	t.Run("queue namespaces are lossless", func(t *testing.T) {
		require.NotEqual(t, QueueNamespace("jobs", "a:b"), QueueNamespace("jobs", "a_b"))
		require.NotEqual(t, QueueNamespace("jobs", "a/b"), QueueNamespace("jobs", "a%2Fb"))
	})
	t.Run("case variants get distinct directories", func(t *testing.T) {
		r := NewRegistry(nil, false)
		files, err := NewScopedFiles(r, t.TempDir())
		require.NoError(t, err)
		require.NoError(t, r.AllowScope("case-a"))
		require.NoError(t, r.AllowScope("CASE-A"))
		lower, err := files.Dir("case-a")
		require.NoError(t, err)
		upper, err := files.Dir("CASE-A")
		require.NoError(t, err)
		require.NotEqual(t, strings.ToLower(lower), strings.ToLower(upper))
	})
	t.Run("resources attach before scopes", func(t *testing.T) {
		r := NewRegistry(nil, false)
		require.NoError(t, r.AllowScope("a"))
		require.Equal(t, "scope_conflict", controlCode(t, r.OnDispose("late", func(context.Context, string) error { return nil })))
	})
	t.Run("clock advance overflow is refused", func(t *testing.T) {
		r := NewRegistry(nil, true)
		r.UseClock(NewManualClock(time.Unix(0, 0)))
		echoScenario(t, r, "dedicated")
		require.NoError(t, r.AllowScope("a"))
		handler, err := NewHandler(r, testToken)
		require.NoError(t, err)
		require.Equal(t, 200, request(t, handler, "POST", "/__test__/scenarios/prepare", testInput("a"), testToken).Code)
		before, _ := r.ClockNow()
		require.Equal(t, 400, request(t, handler, "POST", "/__test__/scopes/a/clock/advance", map[string]int64{"durationMs": -18446744073709}, testToken).Code)
		after, _ := r.ClockNow()
		require.Equal(t, before, after)
	})
	t.Run("failed dedicated preparation restores server time", func(t *testing.T) {
		r := NewRegistry(nil, true)
		start := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
		r.UseClock(NewManualClock(start))
		require.NoError(t, r.Register(testDefinition(), func(context.Context, Input) (Result, error) { return Result{}, errors.New("boom") }, func(context.Context, string) error { return nil }))
		require.NoError(t, r.AllowScope("a"))
		_, err := r.Prepare(ctx, testInput("a"))
		require.Error(t, err)
		now, _ := r.ClockNow()
		require.Equal(t, start, now)
	})
	t.Run("dedicated jobs need their scope's lease", func(t *testing.T) {
		// Falsely green if a scope without the lease could run environment-wide work.
		r := NewRegistry(nil, true)
		echoScenario(t, r, "dedicated")
		jobs, err := NewJobs(r)
		require.NoError(t, err)
		require.NoError(t, jobs.Register(JobDefinition{Name: "global", Dedicated: true}, func(context.Context, JobRequest) error { return nil }))
		for _, id := range []string{"owner", "other"} {
			require.NoError(t, r.AllowScope(id))
		}
		_, err = jobs.Run(ctx, "owner", "global", nil)
		require.Equal(t, "scope_conflict", controlCode(t, err), "no lease before preparation")
		_, err = r.Prepare(ctx, testInput("owner"))
		require.NoError(t, err)
		_, err = jobs.Run(ctx, "other", "global", nil)
		require.Equal(t, "scope_conflict", controlCode(t, err))
		_, err = jobs.Run(ctx, "owner", "global", nil)
		require.NoError(t, err)
	})
	t.Run("a job failing during a check is not reached", func(t *testing.T) {
		r := NewRegistry(nil, false)
		jobs, err := NewJobs(r)
		require.NoError(t, err)
		fail := make(chan struct{})
		require.NoError(t, jobs.Register(JobDefinition{Name: "partial"}, func(ctx context.Context, _ JobRequest) error {
			<-fail
			return errors.New("failed after partial output")
		}))
		var once sync.Once
		require.NoError(t, jobs.RegisterCondition("partial-output", func(ctx context.Context, _ JobRequest) (bool, error) {
			once.Do(func() { close(fail) })
			// Wait for the job to record its failure before answering true.
			for {
				if status, _ := jobs.Status("a", currentExecution(jobs)); status.State == "failed" {
					return true, nil
				}
				select {
				case <-ctx.Done():
					return false, ctx.Err()
				case <-time.After(time.Millisecond):
				}
			}
		}))
		require.NoError(t, r.AllowScope("a"))
		id, err := jobs.Run(ctx, "a", "partial", nil)
		require.NoError(t, err)
		result, err := jobs.Wait(ctx, "a", id, "partial-output", 5000)
		require.Equal(t, "execution_failed", controlCode(t, err))
		require.Equal(t, "failed", result.Outcome)
	})
	t.Run("conditions receive the wait deadline", func(t *testing.T) {
		r := NewRegistry(nil, false)
		jobs, err := NewJobs(r)
		require.NoError(t, err)
		require.NoError(t, jobs.Register(JobDefinition{Name: "idle"}, func(ctx context.Context, _ JobRequest) error { <-ctx.Done(); return nil }))
		require.NoError(t, jobs.RegisterCondition("blocking", func(ctx context.Context, _ JobRequest) (bool, error) {
			<-ctx.Done() // a context-aware query that never answers
			return false, ctx.Err()
		}))
		require.NoError(t, r.AllowScope("a"))
		id, err := jobs.Run(ctx, "a", "idle", nil)
		require.NoError(t, err)
		result, err := jobs.Wait(ctx, "a", id, "blocking", 20)
		require.Equal(t, "timeout", controlCode(t, err))
		require.Equal(t, "timeout", result.Outcome)
		require.NoError(t, r.Dispose(ctx, "a"))
	})
	t.Run("handlers and conditions get their own refs", func(t *testing.T) {
		r := NewRegistry(nil, false)
		jobs, err := NewJobs(r)
		require.NoError(t, err)
		require.NoError(t, jobs.Register(JobDefinition{Name: "derive"}, func(_ context.Context, req JobRequest) error {
			for i := range 1000 {
				req.Refs[fmt.Sprint("derived-", i)] = EntityRef{Kind: "x", ID: "y"}
			}
			return nil
		}))
		require.NoError(t, jobs.RegisterCondition("reads", func(_ context.Context, req JobRequest) (bool, error) {
			_, ok := req.Refs["derived-999"]
			return !ok && len(req.Refs) == 1, nil
		}))
		require.NoError(t, r.AllowScope("a"))
		id, err := jobs.Run(ctx, "a", "derive", map[string]EntityRef{"invoice": {Kind: "invoice", ID: "1"}})
		require.NoError(t, err)
		_, err = jobs.Wait(ctx, "a", id, "reads", 5000)
		require.NoError(t, err)
	})
	t.Run("stuck jobs defer destructive cleanup", func(t *testing.T) {
		// Falsely green if records were deleted while a job could still write.
		r := NewRegistry(nil, false)
		var cleaned int
		d := testDefinition()
		d.Isolation = "shared"
		require.NoError(t, r.Register(d, func(_ context.Context, i Input) (Result, error) {
			return Result{Data: map[string]any{"id": i.Seed}}, nil
		}, func(context.Context, string) error { cleaned++; return nil }))
		jobs, err := NewJobs(r)
		require.NoError(t, err)
		unstick := make(chan struct{})
		require.NoError(t, jobs.Register(JobDefinition{Name: "stuck"}, func(context.Context, JobRequest) error { <-unstick; return nil }))
		require.NoError(t, r.AllowScope("a"))
		_, err = r.Prepare(ctx, testInput("a"))
		require.NoError(t, err)
		_, err = jobs.Run(ctx, "a", "stuck", nil)
		require.NoError(t, err)
		short, stop := context.WithTimeout(ctx, 20*time.Millisecond)
		defer stop()
		require.Equal(t, "cleanup_failed", controlCode(t, r.Dispose(short, "a")))
		require.Zero(t, cleaned, "records stay until the job has exited")
		_, err = jobs.Run(ctx, "a", "stuck", nil)
		require.Equal(t, "scope_conflict", controlCode(t, err), "admission stays closed after a failed cleanup")
		close(unstick)
		require.NoError(t, r.Dispose(ctx, "a"))
		require.Equal(t, 1, cleaned)
	})
}

func currentExecution(j *Jobs) string {
	j.mu.Lock()
	defer j.mu.Unlock()
	for id := range j.executions {
		return id
	}
	return ""
}

// failingRestore succeeds before the callback and fails after it.
type failingRestore struct{ phases []RestorePhase }

func (*failingRestore) ForgetScope(context.Context, string) error     { return nil }
func (*failingRestore) SnapshotScope(string) (json.RawMessage, error) { return nil, nil }
func (f *failingRestore) RestoreScope(_ context.Context, _ string, _ json.RawMessage, phase RestorePhase) error {
	f.phases = append(f.phases, phase)
	if phase == RestoreAfterPrepare {
		return errors.New("provider sandbox unavailable")
	}
	return nil
}
