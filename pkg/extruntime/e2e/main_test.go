package e2e_test

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
)

// repoRoot is the working directory of every test: migrations, locales and
// web/runtime resolve relative to it.
var repoRoot string

var (
	hostOnce   sync.Once
	hostDir    string
	hostErr    error
	hostScript string
)

// TestMain moves to the repository root and, afterwards, removes the host
// worker build shared by the tests of this process.
func TestMain(m *testing.M) {
	if err := os.Chdir("../../.."); err != nil {
		panic(err)
	}
	wd, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	repoRoot = wd
	code := m.Run()
	if hostDir != "" {
		_ = os.RemoveAll(hostDir)
	}
	os.Exit(code)
}

// hostWorker builds the Node host worker (web/runtime/src/host) once per test
// process and returns the path of worker.mjs.
func hostWorker(t testing.TB) string {
	t.Helper()
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node is not installed")
	}
	hostOnce.Do(func() {
		hostDir, hostErr = os.MkdirTemp("", "iota-e2e-host-")
		if hostErr != nil {
			return
		}
		script := filepath.Join(repoRoot, "web", "runtime", "scripts", "build-host.mjs")
		out := filepath.Join(hostDir, "host")
		if b, err := exec.Command(node, "--no-warnings", script, out).CombinedOutput(); err != nil {
			hostErr = fmt.Errorf("build host worker: %w\n%s", err, b)
			return
		}
		hostScript = filepath.Join(out, "worker.mjs")
	})
	if hostErr != nil {
		t.Fatal(hostErr)
	}
	return hostScript
}
