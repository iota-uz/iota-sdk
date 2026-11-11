// Package pooltest builds the host worker and immutable artifact directories
// for tests of the pool and the invocation host.
package pooltest

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/iota-uz/iota-sdk/pkg/extruntime/pool"
)

// RepoRoot returns the repository root.
func RepoRoot(tb testing.TB) string {
	tb.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		tb.Fatal("cannot locate repository root")
	}
	return filepath.Join(filepath.Dir(file), "..", "..", "..", "..")
}

// BuildWorker builds web/runtime/src/host into a temp dir and returns the
// path of worker.mjs. It skips the test when node is unavailable.
func BuildWorker(tb testing.TB) string {
	tb.Helper()
	node, err := exec.LookPath("node")
	if err != nil {
		tb.Skip("node is not installed")
	}
	out := filepath.Join(tb.TempDir(), "host")
	script := filepath.Join(RepoRoot(tb), "web", "runtime", "scripts", "build-host.mjs")
	cmd := exec.Command(node, "--no-warnings", script, out)
	if b, err := cmd.CombinedOutput(); err != nil {
		tb.Fatalf("build host worker: %v\n%s", err, b)
	}
	return filepath.Join(out, "worker.mjs")
}

// Artifact writes source as <dir>/server.mjs in a directory addressed by the
// content hash and returns it.
func Artifact(tb testing.TB, source string) pool.Artifact {
	tb.Helper()
	sum := sha256.Sum256([]byte(source))
	hash := hex.EncodeToString(sum[:])
	dir := filepath.Join(tb.TempDir(), hash)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		tb.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "server.mjs"), []byte(source), 0o644); err != nil {
		tb.Fatal(err)
	}
	return pool.Artifact{Hash: hash, Dir: dir}
}
