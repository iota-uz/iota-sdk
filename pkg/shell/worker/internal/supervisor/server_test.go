package supervisor

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/iota-uz/iota-sdk/pkg/shell/worker/internal/protocol"
	"github.com/iota-uz/iota-sdk/pkg/shell/worker/internal/sandbox"
)

const (
	testToken   = "test-token"
	crashMarker = "#crash-child"
	// awk loops never yield to the shell, so only the supervisor can stop them.
	spinScript = `awk 'BEGIN{while(1){}}'`
)

// TestMain doubles as the child binary: the supervisor re-executes the test
// binary with the "child" argument.
func TestMain(m *testing.M) {
	if len(os.Args) > 1 && os.Args[1] == "child" {
		payload, _ := io.ReadAll(os.Stdin)
		if bytes.Contains(payload, []byte(crashMarker)) {
			panic("simulated worker crash")
		}
		os.Exit(sandbox.ServeChild(bytes.NewReader(payload), os.Stdout, os.Stderr, sandbox.ChildConfig{
			MemoryBytes: 64 << 20,
			Budgets:     sandbox.DefaultBudgets,
		}))
	}
	os.Exit(m.Run())
}

func newTestServer(t *testing.T, queueSize int) *httptest.Server {
	t.Helper()
	sup, err := New(Config{
		Token:        testToken,
		ChildCommand: []string{os.Args[0], "child"},
		QueueSize:    queueSize,
		RetryAfter:   1500 * time.Millisecond,
		KillGrace:    200 * time.Millisecond,
	}, slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(sup.Handler())
	t.Cleanup(srv.Close)
	return srv
}

type response struct {
	status int
	result protocol.Result
	err    protocol.Error
}

func post(ctx context.Context, t *testing.T, srv *httptest.Server, token string, req protocol.Request) (*response, error) {
	t.Helper()
	body, err := json.Marshal(req)
	if err != nil {
		t.Fatal(err)
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, srv.URL+RunsPath, bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	httpReq.Header.Set("Authorization", "Bearer "+token)
	resp, err := srv.Client().Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	out := &response{status: resp.StatusCode}
	target := any(&out.err)
	if resp.StatusCode == http.StatusOK {
		target = &out.result
	}
	if err := json.NewDecoder(resp.Body).Decode(target); err != nil {
		t.Errorf("decode %d response: %v", resp.StatusCode, err)
	}
	return out, nil
}

func mustPost(t *testing.T, srv *httptest.Server, req protocol.Request) *response {
	t.Helper()
	resp, err := post(context.Background(), t, srv, testToken, req)
	if err != nil {
		t.Fatal(err)
	}
	return resp
}

func script(s string, timeoutMS int) protocol.Request {
	return protocol.Request{Script: s, Cwd: protocol.TempDir, TimeoutMS: timeoutMS, MaxOutputBytes: 32 << 10}
}

// assertHealthy checks that the next run is served and starts from an empty
// sandbox, whatever happened to the previous one.
func assertHealthy(t *testing.T, srv *httptest.Server) {
	t.Helper()
	resp := mustPost(t, srv, script("ls -A /tmp /output", 5000))
	if resp.status != http.StatusOK || resp.result.ExitCode == nil || *resp.result.ExitCode != 0 {
		t.Fatalf("next run failed: status=%d result=%+v", resp.status, resp.result)
	}
	if want := "/tmp:\n\n/output:\n"; resp.result.Stdout != want {
		t.Fatalf("next run sees foreign files: %q", resp.result.Stdout)
	}
}

func TestServer_Run(t *testing.T) {
	srv := newTestServer(t, 2)
	resp := mustPost(t, srv, protocol.Request{
		Script:         "wc -l < /input/a.csv; cp /input/a.csv /output/copy.csv; exit 3",
		Cwd:            "/input",
		TimeoutMS:      5000,
		MaxOutputBytes: 1024,
		Files:          []protocol.InputFile{{Path: "a.csv", Content: []byte("a\nb\n")}},
	})
	res := resp.result
	if resp.status != http.StatusOK || res.ExitCode == nil || *res.ExitCode != 3 || res.Stdout != "2\n" || res.LimitHit != nil {
		t.Fatalf("status=%d result=%+v", resp.status, res)
	}
	if res.RunID == "" || len(res.Outputs) != 1 || res.Outputs[0].Path != "/output/copy.csv" || string(res.Outputs[0].Content) != "a\nb\n" {
		t.Fatalf("result=%+v", res)
	}
}

func TestServer_Terminations(t *testing.T) {
	tests := []struct {
		name      string
		script    string
		timeoutMS int
		wantLimit protocol.LimitHit
		maxWait   time.Duration
	}{
		{"hard timeout kills a non-cooperative run", "echo secret > /tmp/s; " + spinScript, 300, protocol.LimitTimeout, 3 * time.Second},
		{"memory exhaustion", "echo secret > /tmp/s; s=xxxxxxxx; while :; do s=$s$s; done", 10000, protocol.LimitMemory, 10 * time.Second},
		{"worker crash", crashMarker, 5000, protocol.LimitExecution, 5 * time.Second},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := newTestServer(t, 1)
			started := time.Now()
			resp := mustPost(t, srv, script(tt.script, tt.timeoutMS))
			elapsed := time.Since(started)
			res := resp.result
			if resp.status != http.StatusOK || res.LimitHit == nil || *res.LimitHit != tt.wantLimit || res.ExitCode != nil {
				t.Fatalf("status=%d result=%+v", resp.status, res)
			}
			if elapsed > tt.maxWait {
				t.Fatalf("run took %s", elapsed)
			}
			assertHealthy(t, srv)
		})
	}
}

func TestServer_Busy(t *testing.T) {
	srv := newTestServer(t, 1)
	var wg sync.WaitGroup
	results := make([]*response, 2)
	for i := range results {
		wg.Add(1)
		go func() {
			defer wg.Done()
			results[i], _ = post(context.Background(), t, srv, testToken, script(spinScript, 1000))
		}()
		time.Sleep(200 * time.Millisecond)
	}

	busy := mustPost(t, srv, script("echo must-not-run > /output/x", 1000))
	if busy.status != http.StatusTooManyRequests || busy.err.Code != protocol.CodeBusy || busy.err.RetryAfterMS != 1500 {
		t.Fatalf("status=%d error=%+v", busy.status, busy.err)
	}

	wg.Wait()
	for _, r := range results {
		if r == nil || r.status != http.StatusOK || r.result.LimitHit == nil || *r.result.LimitHit != protocol.LimitTimeout {
			t.Fatalf("queued run: %+v", r)
		}
	}
	assertHealthy(t, srv)
}

func TestServer_AbandonedClientReleasesQueue(t *testing.T) {
	srv := newTestServer(t, 1)
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	if _, err := post(ctx, t, srv, testToken, script(spinScript, 10000)); err == nil {
		t.Fatal("abandoned request unexpectedly completed")
	}

	started := time.Now()
	assertHealthy(t, srv)
	if elapsed := time.Since(started); elapsed > 3*time.Second {
		t.Fatalf("next run waited %s for the abandoned one", elapsed)
	}
}

func TestServer_RejectsBeforeRun(t *testing.T) {
	srv := newTestServer(t, 1)
	tests := []struct {
		name       string
		token      string
		req        protocol.Request
		wantStatus int
		wantCode   protocol.ErrorCode
	}{
		{"wrong token", "nope", script("true", 1000), http.StatusUnauthorized, protocol.CodeUnauthorized},
		{"traversal", testToken, withFiles(script("true", 1000), "../etc/passwd"), http.StatusBadRequest, protocol.CodeInvalidInput},
		{"absolute path", testToken, withFiles(script("true", 1000), "/etc/passwd"), http.StatusBadRequest, protocol.CodeInvalidInput},
		{"non-canonical path", testToken, withFiles(script("true", 1000), "a/./b"), http.StatusBadRequest, protocol.CodeInvalidInput},
		{"duplicate path", testToken, withFiles(script("true", 1000), "a.csv", "a.csv"), http.StatusBadRequest, protocol.CodeInvalidInput},
		{"file used as directory", testToken, withFiles(script("true", 1000), "a", "a/b"), http.StatusBadRequest, protocol.CodeInvalidInput},
		{"cwd outside sandbox", testToken, protocol.Request{Script: "true", Cwd: "/etc", TimeoutMS: 1000, MaxOutputBytes: 10}, http.StatusBadRequest, protocol.CodeInvalidInput},
		{"timeout above max", testToken, script("true", 20000), http.StatusBadRequest, protocol.CodeInvalidInput},
		{"script too large", testToken, script(strings.Repeat("x", protocol.MaxScriptBytes+1), 1000), http.StatusRequestEntityTooLarge, protocol.CodeLimitExceeded},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := post(context.Background(), t, srv, tt.token, tt.req)
			if err != nil {
				t.Fatal(err)
			}
			if resp.status != tt.wantStatus || resp.err.Code != tt.wantCode {
				t.Fatalf("status=%d error=%+v", resp.status, resp.err)
			}
		})
	}
}

func withFiles(req protocol.Request, paths ...string) protocol.Request {
	for _, p := range paths {
		req.Files = append(req.Files, protocol.InputFile{Path: p, Content: []byte("x")})
	}
	return req
}
