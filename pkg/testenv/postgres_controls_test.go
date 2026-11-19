package testenv

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"hash/fnv"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/dbconfig"
	"github.com/iota-uz/iota-sdk/pkg/dbctl/testdb"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"
)

// freshDatabase creates an owned UUID database on TESTENV_POSTGRES_DSN with
// the schema of a tiny consumer domain.
func freshDatabase(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("TESTENV_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("set TESTENV_POSTGRES_DSN to an isolated PostgreSQL cluster")
	}
	cfg, err := pgx.ParseConfig(dsn)
	require.NoError(t, err)
	dbcfg := dbconfig.Config{Host: cfg.Host, Port: strconv.Itoa(int(cfg.Port)), User: cfg.User, Password: cfg.Password}
	name := "testenv_controls_" + testdb.Name(uuid.NewString())
	ctx := context.Background()
	require.NoError(t, testdb.Create(ctx, name, dbcfg))
	db, err := sql.Open("postgres", testdb.ConnectionString(name, dbcfg))
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = db.Close()
		require.NoError(t, testdb.Drop(context.Background(), name, dbcfg))
	})
	_, err = db.ExecContext(ctx, `
CREATE TABLE customers(scope text NOT NULL, number text PRIMARY KEY, name text NOT NULL);
CREATE TABLE invoices(scope text NOT NULL, number text PRIMARY KEY, customer text NOT NULL REFERENCES customers(number), amount numeric NOT NULL CHECK (amount > 0), due_at timestamptz NOT NULL, status text NOT NULL DEFAULT 'open');`)
	require.NoError(t, err)
	return db
}

type pgEnvironment struct {
	registry  *Registry
	handler   http.Handler
	jobs      *Jobs
	providers *Providers
	files     *ScopedFiles
	clock     *ManualClock
	db        *sql.DB
}

// businessNumber derives a short business identifier from the logical seed.
// The space is deliberately small (1000), so hashing is not a uniqueness
// guarantee: the handler probes for the next free number inside its lock.
func businessNumber(ctx context.Context, tx *sql.Tx, prefix, table, seed string) (string, error) {
	h := fnv.New32a()
	_, _ = h.Write([]byte(seed))
	start := int(h.Sum32() % 1000)
	if _, err := tx.ExecContext(ctx, "SELECT pg_advisory_xact_lock(hashtext($1))", table); err != nil {
		return "", err
	}
	for probe := range 1000 {
		number := fmt.Sprintf("%s-%03d", prefix, (start+probe)%1000)
		var taken bool
		if err := tx.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM "+table+" WHERE number=$1)", number).Scan(&taken); err != nil {
			return "", err
		}
		if !taken {
			return number, nil
		}
	}
	return "", errors.New("business identifier space exhausted")
}

func newPGEnvironment(t *testing.T, db *sql.DB, environmentID string, dedicated bool, build BuildInfo) *pgEnvironment {
	t.Helper()
	env := &pgEnvironment{db: db, clock: NewManualClock(time.Unix(0, 0).UTC())}
	env.registry = NewEnvironmentRegistry([]string{"postgres"}, dedicated, environmentID)
	env.registry.SetBuildInfo(build)
	env.registry.UseClock(env.clock)
	var err error
	env.files, err = NewScopedFiles(env.registry, t.TempDir())
	require.NoError(t, err)
	env.providers, err = NewProviders(env.registry)
	require.NoError(t, err)
	require.NoError(t, env.providers.Register(ProviderDefinition{Name: "notifier", ResponseSchema: map[string]any{
		"type": "object", "required": []any{"messageId"}, "properties": map[string]any{"messageId": map[string]any{"type": "string"}},
	}}))
	env.jobs, err = NewJobs(env.registry)
	require.NoError(t, err)
	isolation := "shared"
	if dedicated {
		isolation = "dedicated"
	}
	cleanup := func(ctx context.Context, scope string) error {
		_, err := db.ExecContext(ctx, "DELETE FROM invoices WHERE scope=$1; DELETE FROM customers WHERE scope=$1", scope)
		if err != nil {
			// lib/pq rejects multiple statements with parameters; run them separately.
			if _, err = db.ExecContext(ctx, "DELETE FROM invoices WHERE scope=$1", scope); err == nil {
				_, err = db.ExecContext(ctx, "DELETE FROM customers WHERE scope=$1", scope)
			}
		}
		return err
	}
	require.NoError(t, env.registry.Register(Definition{
		Name: "overdue-invoice", Version: "3", Isolation: isolation, RequiredCapabilities: []string{"postgres"},
		InputSchema: map[string]any{"type": "object", "required": []any{"amount", "customer"}, "additionalProperties": false, "properties": map[string]any{
			"amount": map[string]any{"type": "number", "exclusiveMinimum": 0}, "customer": map[string]any{"type": "string", "minLength": 1},
			"failAt": map[string]any{"enum": []any{"files"}}, "drift": map[string]any{"type": "boolean"},
		}},
		OutputSchema: map[string]any{"type": "object", "required": []any{"invoiceNumber", "customerNumber"}, "properties": map[string]any{
			"invoiceNumber": map[string]any{"type": "string"}, "customerNumber": map[string]any{"type": "string"},
		}},
	}, func(ctx context.Context, input Input) (Result, error) {
		now, _ := time.Parse(time.RFC3339, input.Now)
		// Stage 1: the DB graph is published atomically.
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			return Result{}, err
		}
		defer func() { _ = tx.Rollback() }()
		customer, err := businessNumber(ctx, tx, "C", "customers", input.Seed)
		if err != nil {
			return Result{}, err
		}
		invoice, err := businessNumber(ctx, tx, "INV", "invoices", input.Seed)
		if err != nil {
			return Result{}, err
		}
		if _, err := tx.ExecContext(ctx, "INSERT INTO customers VALUES($1,$2,$3)", input.ScopeID, customer, input.Params["customer"]); err != nil {
			return Result{}, err
		}
		if _, err := tx.ExecContext(ctx, "INSERT INTO invoices(scope,number,customer,amount,due_at) VALUES($1,$2,$3,$4,$5)", input.ScopeID, invoice, customer, input.Params["amount"], now.Add(24*time.Hour)); err != nil {
			return Result{}, err
		}
		if err := tx.Commit(); err != nil {
			return Result{}, err
		}
		// Stage 2: files become ready only after the graph committed.
		if input.Params["failAt"] == "files" {
			return Result{}, errors.New("file stage failed after the DB graph committed")
		}
		dir, err := env.files.Dir(input.ScopeID)
		if err != nil {
			return Result{}, err
		}
		if err := os.WriteFile(filepath.Join(dir, invoice+".pdf"), []byte("%PDF"), 0o600); err != nil {
			return Result{}, err
		}
		data := map[string]any{"invoiceNumber": invoice, "customerNumber": customer}
		if input.Params["drift"] == true {
			delete(data, "invoiceNumber")
		}
		return Result{Refs: map[string]EntityRef{"invoice": {Kind: "invoice", ID: invoice}}, Data: data}, nil
	}, cleanup))
	// The real dunning handler: claim overdue invoices as of the injected
	// clock, notify through the provider, then mark them dunned.
	require.NoError(t, env.jobs.Register(JobDefinition{Name: "dun"}, func(ctx context.Context, req JobRequest) error {
		number := req.Refs["invoice"].ID
		var claimed string
		err := db.QueryRowContext(ctx, "UPDATE invoices SET status='dunning' WHERE scope=$1 AND number=$2 AND status='open' AND due_at <= $3 RETURNING number", req.ScopeID, number, env.clock.Now()).Scan(&claimed)
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		if err != nil {
			return err
		}
		step, err := env.providers.Call(ctx, "notifier", "send", number)
		if err == nil {
			err = step.Err()
		}
		if err != nil {
			_, _ = db.ExecContext(context.WithoutCancel(ctx), "UPDATE invoices SET status='open' WHERE number=$1", number)
			return err
		}
		_, err = db.ExecContext(ctx, "UPDATE invoices SET status='dunned' WHERE number=$1", number)
		return err
	}))
	require.NoError(t, env.jobs.RegisterCondition("dunned", func(ctx context.Context, req JobRequest) (bool, error) {
		var status string
		err := db.QueryRowContext(ctx, "SELECT status FROM invoices WHERE scope=$1 AND number=$2", req.ScopeID, req.Refs["invoice"].ID).Scan(&status)
		return status == "dunned", err
	}))
	env.handler, err = NewHandler(env.registry, testToken)
	require.NoError(t, err)
	return env
}

func (e *pgEnvironment) count(t *testing.T, scope string) int {
	t.Helper()
	var n int
	require.NoError(t, e.db.QueryRow("SELECT (SELECT count(*) FROM invoices WHERE scope=$1)+(SELECT count(*) FROM customers WHERE scope=$1)", scope).Scan(&n))
	return n
}

type invoiceRow struct {
	Number, Customer, Name string
	Amount                 float64
	DueAt                  time.Time
}

func (e *pgEnvironment) rows(t *testing.T, scope string) []invoiceRow {
	t.Helper()
	rows, err := e.db.Query("SELECT i.number, i.customer, c.name, i.amount, i.due_at FROM invoices i JOIN customers c ON c.number=i.customer WHERE i.scope=$1 ORDER BY i.number", scope)
	require.NoError(t, err)
	defer func() { _ = rows.Close() }()
	var out []invoiceRow
	for rows.Next() {
		var row invoiceRow
		require.NoError(t, rows.Scan(&row.Number, &row.Customer, &row.Name, &row.Amount, &row.DueAt))
		row.DueAt = row.DueAt.UTC()
		out = append(out, row)
	}
	require.NoError(t, rows.Err())
	return out
}

func pgInput(scope string) Input {
	return Input{Name: "overdue-invoice", Version: "3", ScopeID: scope, Seed: "acme-7", Now: "2026-10-02T09:00:00Z", Params: map[string]any{"amount": float64(120), "customer": "Acme"}}
}

func call(t *testing.T, handler http.Handler, method, path string, body any, target any) int {
	t.Helper()
	w := request(t, handler, method, path, body, testToken)
	if target != nil && w.Code < 300 {
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), target), w.Body.String())
	}
	return w.Code
}

func TestPostgresScenarioControlsEndToEnd(t *testing.T) {
	// Falsely green risks covered here: asserting only HTTP JSON instead of
	// durable rows; treating a committed first stage as ready after a later
	// stage failed; a race that passes only because of scheduling; a time
	// boundary crossed by sleeping or by moving only browser time; and a
	// "replay" that reruns the scenario but loses provider scripts and clock.
	build := BuildInfo{BuildRevision: "sdk-test", SchemaFingerprint: "schema-v1", BaselineFingerprint: "baseline-v1"}
	a := newPGEnvironment(t, freshDatabase(t), "envA", true, build)
	h := a.handler
	reserve := func(handler http.Handler, scope string) {
		require.Equal(t, 200, call(t, handler, "POST", "/__test__/scopes", map[string]string{"scopeId": scope}, nil))
	}

	// Version and schema drift are refused before any write.
	reserve(h, "envA-drift")
	versionDrift := pgInput("envA-drift")
	versionDrift.Version = "2"
	require.Equal(t, 409, call(t, h, "POST", "/__test__/scenarios/prepare", versionDrift, nil))
	inputDrift := pgInput("envA-drift")
	inputDrift.Params["amount"] = "ten"
	require.Equal(t, 400, call(t, h, "POST", "/__test__/scenarios/prepare", inputDrift, nil))
	require.Zero(t, a.count(t, "envA-drift"))

	// Output drift and a failed later stage compensate the committed graph and
	// the scope never becomes ready.
	for scope, param := range map[string]string{"envA-output": "drift", "envA-stage": "failAt"} {
		reserve(h, scope)
		input := pgInput(scope)
		if param == "drift" {
			input.Params["drift"] = true
		} else {
			input.Params["failAt"] = "files"
		}
		require.Equal(t, 500, call(t, h, "POST", "/__test__/scenarios/prepare", input, nil))
		require.Zero(t, a.count(t, scope), "committed rows are compensated")
		dir, _ := a.files.path(scope)
		_, err := os.Stat(dir)
		require.True(t, os.IsNotExist(err), "staged files are compensated")
		require.Equal(t, 500, call(t, h, "POST", "/__test__/scenarios/prepare", input, nil), "a failed scope is not handed out as ready")
		require.Equal(t, 409, call(t, h, "GET", "/__test__/scopes/"+scope+"/manifest", nil, nil))
		require.Equal(t, 200, call(t, h, "DELETE", "/__test__/scopes/"+scope, nil, nil))
	}
	require.Equal(t, 200, call(t, h, "DELETE", "/__test__/scopes/envA-drift", nil, nil))

	// Concurrent prepares and a retry after a lost response create one graph.
	reserve(h, "envA-1")
	var wg sync.WaitGroup
	encoded, err := json.Marshal(pgInput("envA-1"))
	require.NoError(t, err)
	bodies := make([]string, 8)
	for i := range bodies {
		wg.Add(1)
		go func() {
			defer wg.Done()
			req := httptest.NewRequest(http.MethodPost, "/__test__/scenarios/prepare", bytes.NewReader(encoded))
			req.Header.Set("Authorization", "Bearer "+testToken)
			w := httptest.NewRecorder()
			h.ServeHTTP(w, req)
			bodies[i] = fmt.Sprint(w.Code, w.Body.String())
		}()
	}
	wg.Wait()
	for _, body := range bodies {
		require.True(t, strings.HasPrefix(body, "200"), "every concurrent prepare succeeds: %s", body)
		require.Equal(t, bodies[0], body)
	}
	var prepared Result
	require.Equal(t, 200, call(t, h, "POST", "/__test__/scenarios/prepare", pgInput("envA-1"), &prepared))
	require.Equal(t, 2, a.count(t, "envA-1"))
	invoiceRef := map[string]EntityRef{"invoice": prepared.Refs["invoice"]}
	original := a.rows(t, "envA-1")

	script := ProviderScript{Steps: []ProviderStep{{Operation: "send", Outcome: OutcomeSuccess, Hold: "send", Response: map[string]any{"messageId": "m-1"}}}}
	require.Equal(t, 200, call(t, h, "POST", "/__test__/scopes/envA-1/providers/notifier/scripts", script, nil))
	manifestBefore := Manifest{}
	require.Equal(t, 200, call(t, h, "GET", "/__test__/scopes/envA-1/manifest", nil, &manifestBefore))

	runJob := func(env *pgEnvironment, scope string) string {
		var out struct {
			ExecutionID string `json:"executionId"`
		}
		require.Equal(t, 200, call(t, env.handler, "POST", "/__test__/scopes/"+scope+"/jobs", map[string]any{"job": "dun", "refs": invoiceRef}, &out))
		return out.ExecutionID
	}
	wait := func(env *pgEnvironment, scope, execution, condition string, timeoutMs int) (int, WaitResult) {
		w := request(t, env.handler, "POST", "/__test__/scopes/"+scope+"/executions/"+execution+"/wait", map[string]any{"condition": condition, "timeoutMs": timeoutMs}, testToken)
		var result WaitResult
		if w.Code == 200 {
			require.NoError(t, json.Unmarshal(w.Body.Bytes(), &result))
		} else {
			var failure struct {
				Details WaitResult `json:"details"`
			}
			require.NoError(t, json.Unmarshal(w.Body.Bytes(), &failure))
			result = failure.Details
		}
		return w.Code, result
	}

	// Before the due boundary the real job runs and changes nothing.
	early := runJob(a, "envA-1")
	code, _ := wait(a, "envA-1", early, ConditionCompleted, 5000)
	require.Equal(t, 200, code)
	code, result := wait(a, "envA-1", early, "dunned", 30)
	require.Equal(t, 504, code)
	require.Equal(t, "timeout", result.Outcome)
	require.Equal(t, "succeeded", result.Execution.State)

	// Cross the boundary by advancing server time; no sleep is involved.
	require.Equal(t, 200, call(t, h, "POST", "/__test__/scopes/envA-1/clock/advance", map[string]int64{"durationMs": (24 * time.Hour).Milliseconds()}, nil))

	// Controlled race: the first dunning run is held inside the provider call
	// while a second run completes; exactly one notification is sent.
	first := runJob(a, "envA-1")
	require.Equal(t, 200, call(t, h, "POST", "/__test__/scopes/envA-1/providers/notifier/gates/send/wait", map[string]int{"timeoutMs": 5000}, nil))
	second := runJob(a, "envA-1")
	code, _ = wait(a, "envA-1", second, ConditionCompleted, 5000)
	require.Equal(t, 200, code)
	var calls struct {
		Calls []ProviderCall `json:"calls"`
	}
	require.Equal(t, 200, call(t, h, "GET", "/__test__/scopes/envA-1/providers/notifier/calls", nil, &calls))
	require.Len(t, calls.Calls, 1)
	require.Equal(t, "held", calls.Calls[0].Outcome)
	require.Equal(t, "envA-1/"+first, calls.Calls[0].Correlation, "the async job carries scope correlation to the fake")
	require.Equal(t, 200, call(t, h, "POST", "/__test__/scopes/envA-1/providers/notifier/gates/send/release", nil, nil))
	code, result = wait(a, "envA-1", first, "dunned", 5000)
	require.Equal(t, 200, code)
	require.Equal(t, "reached", result.Outcome)

	// Replay the manifest on a fresh database and environment.
	var manifest Manifest
	require.Equal(t, 200, call(t, h, "GET", "/__test__/scopes/envA-1/manifest", nil, &manifest))
	require.Equal(t, manifestBefore.Controls.Resources, manifest.Controls.Resources, "scripts are recorded as submitted, not as consumed")
	require.Equal(t, "2026-10-03T09:00:00Z", manifest.Controls.Clock)
	require.Equal(t, build, manifest.Environment)
	b := newPGEnvironment(t, freshDatabase(t), "envB", true, build)
	reserve(b.handler, "envB-1")
	var replayed Result
	require.Equal(t, 200, call(t, b.handler, "POST", "/__test__/scopes/envB-1/replay", manifest, &replayed))
	require.Equal(t, prepared.Refs, replayed.Refs, "same logical seed reproduces business identifiers")
	require.Equal(t, prepared.Data, replayed.Data)
	require.Equal(t, original, b.rows(t, "envB-1"), "DB state is reproduced")
	require.Equal(t, a.clock.Now(), b.clock.Now(), "clock is reproduced")
	replayedRun := runJob(b, "envB-1")
	require.Equal(t, 200, call(t, b.handler, "POST", "/__test__/scopes/envB-1/providers/notifier/gates/send/wait", map[string]int{"timeoutMs": 5000}, nil), "provider scripts are reproduced")
	require.Equal(t, 200, call(t, b.handler, "POST", "/__test__/scopes/envB-1/providers/notifier/gates/send/release", nil, nil))
	code, _ = wait(b, "envB-1", replayedRun, "dunned", 5000)
	require.Equal(t, 200, code)

	drifted := manifest
	drifted.Environment.SchemaFingerprint = "schema-v2"
	reserve(b.handler, "envB-2")
	require.Equal(t, 409, call(t, b.handler, "POST", "/__test__/scopes/envB-2/replay", drifted, nil))
	require.Zero(t, b.count(t, "envB-2"))

	// Repeated dispose is idempotent and removes rows, files and fake state.
	for range 2 {
		require.Equal(t, 200, call(t, h, "DELETE", "/__test__/scopes/envA-1", nil, nil))
	}
	require.Zero(t, a.count(t, "envA-1"))
	dir, _ := a.files.path("envA-1")
	_, err = os.Stat(dir)
	require.True(t, os.IsNotExist(err))
	require.Equal(t, 409, call(t, h, "GET", "/__test__/scopes/envA-1/providers/notifier/calls", nil, nil))

	// In a shared environment, scopes with the same logical seed get distinct
	// business identifiers because the handler checks occupancy.
	shared := newPGEnvironment(t, a.db, "envS", false, build)
	numbers := map[string]bool{}
	for _, scope := range []string{"envS-1", "envS-2"} {
		reserve(shared.handler, scope)
		var out Result
		require.Equal(t, 412, call(t, shared.handler, "POST", "/__test__/scopes/"+scope+"/clock/advance", map[string]int64{"durationMs": 1}, nil), "shared environments refuse clock control")
		require.Equal(t, 200, call(t, shared.handler, "POST", "/__test__/scenarios/prepare", pgInput(scope), &out))
		numbers[out.Refs["invoice"].ID] = true
	}
	require.Len(t, numbers, 2)
	for _, scope := range []string{"envS-1", "envS-2"} {
		require.Equal(t, 200, call(t, shared.handler, "DELETE", "/__test__/scopes/"+scope, nil, nil))
		require.Zero(t, shared.count(t, scope))
	}
}
