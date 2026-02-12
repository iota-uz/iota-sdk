package runtime

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	appletenginerpc "github.com/iota-uz/iota-sdk/pkg/appletengine/rpc"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testFileStore struct {
	files map[string]map[string]any
}

func newTestFileStore() *testFileStore {
	return &testFileStore{files: make(map[string]map[string]any)}
}

func (s *testFileStore) Store(_ context.Context, name, contentType string, data []byte) (map[string]any, error) {
	id := "f1"
	record := map[string]any{
		"id":          id,
		"name":        name,
		"contentType": contentType,
		"size":        len(data),
		"path":        "/tmp/" + name,
	}
	s.files[id] = record
	return record, nil
}

func (s *testFileStore) Get(_ context.Context, id string) (map[string]any, bool, error) {
	record, ok := s.files[id]
	return record, ok, nil
}

func (s *testFileStore) Delete(_ context.Context, id string) (bool, error) {
	_, ok := s.files[id]
	if ok {
		delete(s.files, id)
	}
	return ok, nil
}

func requireBun(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("bun"); err != nil {
		t.Skip("bun is not available in PATH")
	}
}

func TestManager_RequiresEntryPoint(t *testing.T) {
	manager := NewManager(t.TempDir(), appletenginerpc.NewDispatcher(appletenginerpc.NewRegistry(), nil, logrus.New()), logrus.New())

	proc, err := manager.EnsureStarted(context.Background(), "bichat", "")
	require.Error(t, err)
	assert.Nil(t, proc)
	assert.Contains(t, err.Error(), "entry point is required")
}

func TestManager_EngineSocketUnavailablePath(t *testing.T) {
	baseDir := t.TempDir()
	blockedPath := filepath.Join(baseDir, "blocked")
	require.NoError(t, os.WriteFile(blockedPath, []byte("x"), 0o644))

	manager := NewManager(blockedPath, appletenginerpc.NewDispatcher(appletenginerpc.NewRegistry(), nil, logrus.New()), logrus.New())
	proc, err := manager.EnsureStarted(context.Background(), "bichat", "modules/bichat/runtime/index.ts")
	require.Error(t, err)
	assert.Nil(t, proc)
	assert.Contains(t, err.Error(), "create runtime directory")
}

func TestManager_SpawnAndHealthSuccess(t *testing.T) {
	requireBun(t)

	manager := NewManager(t.TempDir(), appletenginerpc.NewDispatcher(appletenginerpc.NewRegistry(), nil, logrus.New()), logrus.New())
	entrypoint := writeRuntimeEntry(t, t.TempDir())

	proc, err := manager.EnsureStarted(context.Background(), "bichat", entrypoint)
	require.NoError(t, err)
	require.NotNil(t, proc)
	require.NotNil(t, proc.Cmd)
	require.NotNil(t, proc.Cmd.Process)
	assert.NotEmpty(t, manager.EngineSocketPath())

	again, err := manager.EnsureStarted(context.Background(), "bichat", entrypoint)
	require.NoError(t, err)
	require.NotNil(t, again)
	assert.Equal(t, proc.Cmd.Process.Pid, again.Cmd.Process.Pid)

	t.Cleanup(func() {
		_ = manager.Shutdown(context.Background())
	})
}

func TestManager_CrashAndRestartBackoff(t *testing.T) {
	requireBun(t)
	t.Setenv("IOTA_TEST_CRASH_ONCE", "1")
	crashMarker := filepath.Join(t.TempDir(), "crashed-once.marker")
	t.Setenv("IOTA_TEST_CRASH_MARKER", crashMarker)

	manager := NewManager(t.TempDir(), appletenginerpc.NewDispatcher(appletenginerpc.NewRegistry(), nil, logrus.New()), logrus.New())
	entrypoint := writeRuntimeEntry(t, t.TempDir())

	proc, err := manager.EnsureStarted(context.Background(), "bichat", entrypoint)
	require.NoError(t, err)
	require.NotNil(t, proc)
	firstPID := proc.Cmd.Process.Pid

	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		manager.mu.Lock()
		current := manager.processes["bichat"]
		manager.mu.Unlock()
		if current != nil && current.Cmd != nil && current.Cmd.Process != nil && current.Cmd.Process.Pid != firstPID {
			t.Cleanup(func() {
				_ = manager.Shutdown(context.Background())
			})
			return
		}
		time.Sleep(200 * time.Millisecond)
	}

	_ = manager.Shutdown(context.Background())
	t.Fatalf("expected bun process restart with new pid (initial=%d)", firstPID)
}

func TestManager_DispatchJob(t *testing.T) {
	requireBun(t)

	manager := NewManager(t.TempDir(), appletenginerpc.NewDispatcher(appletenginerpc.NewRegistry(), nil, logrus.New()), logrus.New())
	entrypoint := writeRuntimeEntry(t, t.TempDir())
	manager.RegisterApplet("bichat", entrypoint)

	_, err := manager.EnsureStarted(context.Background(), "bichat", "")
	require.NoError(t, err)

	err = manager.DispatchJob(context.Background(), "bichat", "tenant-1", "job-1", "bichat.test", map[string]any{"x": 1})
	require.NoError(t, err)

	err = manager.DispatchWebsocketEvent(context.Background(), "bichat", "tenant-1", "conn-1", "message", []byte("hi"))
	require.NoError(t, err)

	t.Cleanup(func() {
		_ = manager.Shutdown(context.Background())
	})
}

func TestManager_CallPublicMethod_ForwardsHeaders(t *testing.T) {
	requireBun(t)

	manager := NewManager(t.TempDir(), appletenginerpc.NewDispatcher(appletenginerpc.NewRegistry(), nil, logrus.New()), logrus.New())
	entrypoint := writeRuntimeEntry(t, t.TempDir())
	manager.RegisterApplet("bichat", entrypoint)

	_, err := manager.EnsureStarted(context.Background(), "bichat", "")
	require.NoError(t, err)

	result, err := manager.CallPublicMethod(
		context.Background(),
		"bichat",
		"bichat.ping",
		json.RawMessage(`{"value":"ok"}`),
		http.Header{
			"X-Iota-Tenant-Id":  []string{"tenant-1"},
			"X-Iota-User-Id":    []string{"user-1"},
			"X-Iota-Request-Id": []string{"req-1"},
			"Cookie":            []string{"sid=abc123"},
			"Authorization":     []string{"Bearer token-1"},
		},
	)
	require.NoError(t, err)

	payload, ok := result.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "bichat.ping", payload["method"])
	assert.Equal(t, "tenant-1", payload["tenantId"])
	assert.Equal(t, "user-1", payload["userId"])
	assert.Equal(t, "req-1", payload["requestId"])
	assert.Equal(t, "sid=abc123", payload["cookie"])
	assert.Equal(t, "Bearer token-1", payload["authorization"])

	t.Cleanup(func() {
		_ = manager.Shutdown(context.Background())
	})
}

func TestManager_EngineSocketFilesEndpoints(t *testing.T) {
	manager := NewManager(t.TempDir(), appletenginerpc.NewDispatcher(appletenginerpc.NewRegistry(), nil, logrus.New()), logrus.New())
	manager.RegisterFileStore("bichat", newTestFileStore())
	require.NoError(t, manager.ensureEngineSocket())
	require.NotEmpty(t, manager.EngineSocketPath())

	storeReq, err := unixRequest(
		manager.EngineSocketPath(),
		http.MethodPost,
		"/files/store",
		[]byte("hello"),
		map[string]string{
			"X-Iota-Applet-Id":    "bichat",
			"X-Iota-Tenant-Id":    "tenant-1",
			"X-Iota-File-Name":    "greet.txt",
			"X-Iota-Content-Type": "text/plain",
		},
	)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, storeReq.StatusCode)
	storeBody, err := io.ReadAll(storeReq.Body)
	require.NoError(t, err)
	_ = storeReq.Body.Close()

	var stored map[string]any
	require.NoError(t, json.Unmarshal(storeBody, &stored))
	assert.Equal(t, "greet.txt", stored["name"])

	getReq, err := unixRequest(
		manager.EngineSocketPath(),
		http.MethodGet,
		"/files/get?id=f1&applet=bichat",
		nil,
		map[string]string{"X-Iota-Tenant-Id": "tenant-1"},
	)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, getReq.StatusCode)
	_ = getReq.Body.Close()

	deleteReq, err := unixRequest(
		manager.EngineSocketPath(),
		http.MethodDelete,
		"/files/delete?id=f1&applet=bichat",
		nil,
		map[string]string{"X-Iota-Tenant-Id": "tenant-1"},
	)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, deleteReq.StatusCode)
	_ = deleteReq.Body.Close()

	require.NoError(t, manager.Shutdown(context.Background()))
}

func writeRuntimeEntry(t *testing.T, dir string) string {
	t.Helper()
	entry := filepath.Join(dir, "runtime.ts")
	source := `
import { existsSync, writeFileSync } from "node:fs"

const crashOnce = process.env.IOTA_TEST_CRASH_ONCE === "1"
const markerPath = process.env.IOTA_TEST_CRASH_MARKER
if (crashOnce && markerPath && !existsSync(markerPath)) {
  writeFileSync(markerPath, "1")
  setTimeout(() => process.exit(1), 2000)
}

Bun.serve({
  unix: process.env.IOTA_APPLET_SOCKET!,
  fetch(request: Request) {
    const pathname = new URL(request.url).pathname
    if (pathname === "/__health") {
      return new Response("ok", { status: 200 })
    }
    if (pathname === "/__public_rpc" && request.method === "POST") {
      return request.json().then((payload: any) =>
        new Response(
          JSON.stringify({
            jsonrpc: "2.0",
            id: payload?.id ?? null,
            result: {
              method: payload?.method ?? "",
              tenantId: request.headers.get("x-iota-tenant-id") ?? "",
              userId: request.headers.get("x-iota-user-id") ?? "",
              requestId: request.headers.get("x-iota-request-id") ?? "",
              cookie: request.headers.get("cookie") ?? "",
              authorization: request.headers.get("authorization") ?? "",
            },
          }),
          { status: 200, headers: { "content-type": "application/json" } },
        ),
      )
    }
    if (pathname === "/__job" && request.method === "POST") {
      return new Response(JSON.stringify({ ok: true }), { status: 200, headers: { "content-type": "application/json" } })
    }
    if (pathname === "/__ws" && request.method === "POST") {
      return new Response(JSON.stringify({ ok: true }), { status: 200, headers: { "content-type": "application/json" } })
    }
    return new Response("not found", { status: 404 })
  },
})
`
	require.NoError(t, os.WriteFile(entry, []byte(source), 0o644))
	return entry
}

func unixRequest(socketPath, method, path string, body []byte, headers map[string]string) (*http.Response, error) {
	dialer := &net.Dialer{}
	transport := &http.Transport{
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			return dialer.DialContext(ctx, "unix", socketPath)
		},
	}
	client := &http.Client{Transport: transport}
	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}
	req, err := http.NewRequest(method, "http://unix"+path, reader)
	if err != nil {
		return nil, err
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	return client.Do(req)
}
