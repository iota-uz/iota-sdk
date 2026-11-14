// Package buildtest builds the Node build worker (and the SDK runtime it
// serves) for tests of the build adapter and of its users.
package buildtest

import (
	"crypto/sha256"
	"encoding/hex"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"

	"github.com/iota-uz/iota-sdk/pkg/extruntime/build"
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

var (
	once    sync.Once
	cached  string
	cacheOK bool
	cacheEr string
)

// Config returns a build.Config for the worker built from web/runtime. The
// build (tsc for the SDK, esbuild for the worker) is cached per source hash in
// the OS temp directory, so it runs once across test processes. The test is
// skipped when node or web/runtime's node_modules (pnpm install) are missing.
func Config(tb testing.TB) build.Config {
	tb.Helper()
	node, err := exec.LookPath("node")
	if err != nil {
		tb.Skip("node is not installed")
	}
	root := filepath.Join(RepoRoot(tb), "web", "runtime")
	modules := filepath.Join(root, "node_modules")
	if _, err := os.Stat(filepath.Join(modules, "esbuild")); err != nil {
		tb.Skip("web/runtime dependencies are not installed (run `pnpm install`)")
	}
	once.Do(func() {
		cached, cacheOK, cacheEr = ensure(root, node)
	})
	if !cacheOK {
		tb.Fatalf("build the build worker: %s", cacheEr)
	}
	return build.Config{
		Node:         node,
		WorkerScript: filepath.Join(cached, "build", "worker.mjs"),
		NodePath:     modules,
		WorkDir:      tb.TempDir(),
	}
}

func ensure(root, node string) (string, bool, string) {
	stamp := sourceHash(root)
	dir := filepath.Join(os.TempDir(), "iota-extruntime-build-"+stamp[:16])
	if _, err := os.Stat(filepath.Join(dir, "build", "worker.mjs")); err == nil {
		return dir, true, ""
	}
	staging, err := os.MkdirTemp(os.TempDir(), "iota-extruntime-build-staging-")
	if err != nil {
		return "", false, err.Error()
	}
	cmd := exec.Command(node, "--no-warnings", filepath.Join(root, "scripts", "build-worker.mjs"), "--emit-sdk", staging)
	cmd.Dir = root
	if out, err := cmd.CombinedOutput(); err != nil {
		_ = os.RemoveAll(staging)
		return "", false, err.Error() + "\n" + string(out)
	}
	if err := os.Rename(staging, dir); err != nil {
		// Another process won the race; its build is equivalent.
		_ = os.RemoveAll(staging)
		if _, statErr := os.Stat(filepath.Join(dir, "build", "worker.mjs")); statErr != nil {
			return "", false, err.Error()
		}
	}
	return dir, true, ""
}

// sourceHash covers everything the worker build depends on: the runtime
// sources, the SDK theme the package stylesheets are compiled with
// (tailwind.config.js) and the UI kit whose export names and declarations the
// worker ships (web/solid-ui/src).
func sourceHash(root string) string {
	h := sha256.New()
	for _, sub := range []string{"src", "scripts", filepath.Join("..", "solid-ui", "src")} {
		_ = filepath.WalkDir(filepath.Join(root, sub), func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				// An unreadable directory contributes nothing to the hash; for a
				// failed root WalkDir treats SkipDir as the end of the walk.
				return fs.SkipDir
			}
			if d.IsDir() {
				return nil
			}
			if strings.HasSuffix(path, ".ts") || strings.HasSuffix(path, ".tsx") || strings.HasSuffix(path, ".mjs") || strings.HasSuffix(path, ".json") {
				b, _ := os.ReadFile(path)
				h.Write([]byte(path))
				h.Write(b)
			}
			return nil
		})
	}
	for _, f := range []string{"package.json", "tsconfig.json", "tsconfig.build.json", filepath.Join("..", "..", "tailwind.config.js"), filepath.Join("..", "solid-ui", "tsconfig.package.json")} {
		b, _ := os.ReadFile(filepath.Join(root, f))
		h.Write(b)
	}
	return hex.EncodeToString(h.Sum(nil))
}
