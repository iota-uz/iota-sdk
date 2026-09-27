//go:build unix

package isolation_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/iota-uz/iota-sdk/pkg/pykernel/isolation"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLauncher_WorkdirAndEnvScrub(t *testing.T) {
	// No t.Parallel(): t.Setenv mutates process-wide state.
	workdir := t.TempDir()
	// Set a host env var that must NOT leak into the child.
	t.Setenv("PYKERNEL_SECRET", "leaked")

	l := isolation.NewLauncher()
	// The child writes "$ALLOWED:$PYKERNEL_SECRET" into a file in its cwd. If
	// the workdir is honoured the file lands in workdir; if the env is scrubbed
	// PYKERNEL_SECRET is empty in the child.
	proc, err := l.Launch(context.Background(), isolation.SandboxSpec{
		Command: []string{"/bin/sh", "-c", `printf '%s' "$ALLOWED:$PYKERNEL_SECRET" > marker.txt`},
		Workdir: workdir,
		Env:     []string{"ALLOWED=ok"},
	})
	require.NoError(t, err)
	require.NoError(t, proc.Wait())

	got, err := os.ReadFile(filepath.Join(workdir, "marker.txt"))
	require.NoError(t, err)
	assert.Equal(t, "ok:", string(got), "ALLOWED should pass through; the host secret must be scrubbed")
}

func TestLauncher_NilEnvIsEmptyNotHost(t *testing.T) {
	// No t.Parallel(): t.Setenv mutates process-wide state.
	workdir := t.TempDir()
	t.Setenv("PYKERNEL_SECRET", "leaked")

	l := isolation.NewLauncher()
	proc, err := l.Launch(context.Background(), isolation.SandboxSpec{
		Command: []string{"/bin/sh", "-c", `printf '%s' "${PYKERNEL_SECRET}" > marker.txt`},
		Workdir: workdir,
		Env:     nil, // must yield an EMPTY child env, not the host's
	})
	require.NoError(t, err)
	require.NoError(t, proc.Wait())

	got, err := os.ReadFile(filepath.Join(workdir, "marker.txt"))
	require.NoError(t, err)
	assert.Empty(t, string(got), "nil Env must not inherit the host environment")
}

func TestLauncher_KillReapsProcessGroup(t *testing.T) {
	t.Parallel()

	l := isolation.NewLauncher()
	proc, err := l.Launch(context.Background(), isolation.SandboxSpec{
		Command: []string{"/bin/sh", "-c", "sleep 30"},
		Workdir: t.TempDir(),
		Env:     []string{},
	})
	require.NoError(t, err)

	require.NoError(t, proc.Kill())

	done := make(chan struct{})
	go func() {
		_ = proc.Wait() // returns a non-nil "signal: killed" error
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("killed process did not exit promptly")
	}
}
