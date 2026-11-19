package cli

import (
	"context"
	"encoding/json"
	"io"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/iota-uz/iota-sdk/pkg/testenv"
	"github.com/stretchr/testify/require"
)

const token = "test-control-token-32-characters-long"

func input() testenv.Input {
	return testenv.Input{Name: "invoice", Version: "1", Seed: "seed-7", Now: "2026-10-02T00:00:00Z", Params: map[string]any{"amount": float64(10)}}
}

func TestGoCLIAndHTTPProduceEquivalentResults(t *testing.T) {
	// Falsely green if each path were compared with itself: one scenario runs
	// through the Go API, the CLI over Local, and the CLI over real HTTP.
	r := testenv.NewEnvironmentRegistry(nil, false, "env")
	r.SetBuildInfo(testenv.BuildInfo{BuildRevision: "r", SchemaFingerprint: "s", BaselineFingerprint: "b"})
	prepares := 0
	require.NoError(t, r.Register(testenv.Definition{
		Name: "invoice", Version: "1", Isolation: "shared",
		InputSchema:  map[string]any{"type": "object", "required": []any{"amount"}},
		OutputSchema: map[string]any{"type": "object", "required": []any{"number"}},
	}, func(_ context.Context, i testenv.Input) (testenv.Result, error) {
		prepares++
		number := "INV-" + i.Seed
		return testenv.Result{Refs: map[string]testenv.EntityRef{"invoice": {Kind: "invoice", ID: number}}, Data: map[string]any{"number": number, "amount": i.Params["amount"]}}, nil
	}, func(context.Context, string) error { return nil }))
	handler, err := testenv.NewHandler(r, token)
	require.NoError(t, err)
	server := httptest.NewServer(handler)
	defer server.Close()
	client, err := testenv.NewClient(server.URL, token, "env")
	require.NoError(t, err)
	ctx := context.Background()
	normalize := func(result testenv.Result) testenv.Result { result.ScopeID = ""; return result }

	require.NoError(t, r.ReserveScope("env-go"))
	goInput := input()
	goInput.ScopeID = "env-go"
	viaGo, err := r.Prepare(ctx, goInput)
	require.NoError(t, err)

	encoded, err := json.Marshal(input())
	require.NoError(t, err)
	manifestPath := filepath.Join(t.TempDir(), "manifest.json")
	paths := []struct {
		scope   string
		control testenv.Control
	}{{"env-cli-local", testenv.Local(r)}, {"env-cli-http", client}}
	for _, path := range paths {
		var out strings.Builder
		require.NoError(t, Run(ctx, path.control, []string{"prepare", "--scope", path.scope, "--manifest", manifestPath}, strings.NewReader(string(encoded)), &out))
		var result testenv.Result
		require.NoError(t, json.Unmarshal([]byte(out.String()), &result))
		require.Equal(t, path.scope, result.ScopeID)
		require.Equal(t, normalize(viaGo), normalize(result))
	}
	retryInput := input()
	retryInput.ScopeID = "env-cli-http"
	viaHTTP, err := client.Prepare(ctx, retryInput)
	require.NoError(t, err)
	require.Equal(t, normalize(viaGo), normalize(viaHTTP))
	require.Equal(t, 3, prepares, "a retried prepare after a lost response does not run the handler again")

	saved, err := os.ReadFile(manifestPath)
	require.NoError(t, err)
	require.NotContains(t, string(saved), token)
	var out strings.Builder
	require.NoError(t, Run(ctx, client, []string{"replay", "--scope", "env-replay", "--manifest", manifestPath}, nil, &out))
	var replayed testenv.Result
	require.NoError(t, json.Unmarshal([]byte(out.String()), &replayed))
	require.Equal(t, normalize(viaGo), normalize(replayed))

	for range 2 {
		require.NoError(t, Run(ctx, client, []string{"dispose", "--scope", "env-cli-http"}, nil, &strings.Builder{}))
	}
	err = Run(ctx, client, []string{"prepare", "--scope", "foreign-1"}, strings.NewReader(string(encoded)), &strings.Builder{})
	var control *testenv.Error
	require.ErrorAs(t, err, &control)
	require.Equal(t, "scope_conflict", control.Code)
	versionDrift := input()
	versionDrift.Version = "2"
	drifted, err := json.Marshal(versionDrift)
	require.NoError(t, err)
	err = Run(ctx, client, []string{"prepare", "--scope", "env-drift"}, strings.NewReader(string(drifted)), &strings.Builder{})
	require.ErrorAs(t, err, &control)
	require.Equal(t, "version_mismatch", control.Code)

	// Exactly one JSON document: a trailing second document is refused.
	err = Run(ctx, client, []string{"prepare", "--scope", "env-two"}, strings.NewReader(string(encoded)+"\n"+string(encoded)), &strings.Builder{})
	require.ErrorAs(t, err, &control)
	require.Equal(t, "invalid_input", control.Code)
	require.NoError(t, Run(ctx, client, []string{"prepare", "--scope", "env-ws"}, strings.NewReader(string(encoded)+"\n \n"), &strings.Builder{}))

	// Cancellation ends a command whose input never completes.
	pending, writer := io.Pipe()
	defer func() { _ = writer.Close() }()
	canceled, cancel := context.WithCancel(ctx)
	finished := make(chan error, 1)
	go func() {
		finished <- Run(canceled, client, []string{"prepare", "--scope", "env-open"}, pending, &strings.Builder{})
	}()
	_, _ = writer.Write([]byte("{"))
	cancel()
	require.ErrorAs(t, <-finished, &control)
	require.Equal(t, "timeout", control.Code)

	foreign, err := testenv.NewClient(server.URL, token, "another")
	require.NoError(t, err)
	_, err = foreign.Definitions(ctx)
	require.ErrorAs(t, err, &control)
	require.Equal(t, "scope_conflict", control.Code, "the client checks environment ownership")
}
