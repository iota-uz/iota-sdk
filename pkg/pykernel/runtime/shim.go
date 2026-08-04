// Package runtime carries the embedded Python kernel shim and materializes it
// at spawn time. Shipping the shim inside the Go binary (rather than pip-
// installing it) keeps a single source of truth and guarantees the shim and the
// host bridge never disagree on the wire protocol.
package runtime

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
)

//go:embed bootstrap.py
var bootstrapPy []byte

// ShimFilename is the materialized shim's filename inside the control directory.
const ShimFilename = "bootstrap.py"

// Write materializes the embedded shim into dir (created 0700 if absent) and
// returns the shim's path. It is written fresh on every spawn.
func Write(dir string) (string, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("pykernel/runtime: mkdir %q: %w", dir, err)
	}
	path := filepath.Join(dir, ShimFilename)
	if err := os.WriteFile(path, bootstrapPy, 0o600); err != nil {
		return "", fmt.Errorf("pykernel/runtime: write shim %q: %w", path, err)
	}
	return path, nil
}
