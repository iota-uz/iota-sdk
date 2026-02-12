package runtime

import (
	"context"
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

func requireBun(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("bun"); err != nil {
		t.Skip("bun is not available in PATH")
	}
}

func TestManager_DisabledFlagDoesNotSpawn(t *testing.T) {
	t.Setenv("IOTA_APPLET_ENGINE_BICHAT", "")
	manager := NewManager(t.TempDir(), appletenginerpc.NewDispatcher(appletenginerpc.NewRegistry(), nil, logrus.New()), logrus.New())

	proc, err := manager.EnsureStarted(context.Background(), "bichat", "modules/bichat/runtime/index.ts")
	require.NoError(t, err)
	assert.Nil(t, proc)
}

func TestManager_EngineSocketUnavailablePath(t *testing.T) {
	t.Setenv("IOTA_APPLET_ENGINE_BICHAT", "bun")

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
	t.Setenv("IOTA_APPLET_ENGINE_BICHAT", "bun")

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
	t.Setenv("IOTA_APPLET_ENGINE_BICHAT", "bun")
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
    if (new URL(request.url).pathname === "/__health") {
      return new Response("ok", { status: 200 })
    }
    return new Response("not found", { status: 404 })
  },
})
`
	require.NoError(t, os.WriteFile(entry, []byte(source), 0o644))
	return entry
}
