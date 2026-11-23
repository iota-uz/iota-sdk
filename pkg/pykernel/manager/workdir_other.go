//go:build !unix

package manager

import (
	"os"
	"path/filepath"

	"github.com/iota-uz/iota-sdk/pkg/pykernel/isolation"
)

// prepareState is the unconfined fallback for platforms without the sandbox.
func prepareState(workdir string, _ *isolation.Credential) error {
	return os.MkdirAll(filepath.Join(workdir, stateDir, "tmp"), 0o700)
}

// dirIdentity is unavailable here: every kernel looks like a new directory.
func dirIdentity(string) (uint64, uint64, error) { return 0, 0, nil }
