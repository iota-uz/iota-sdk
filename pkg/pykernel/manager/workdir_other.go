//go:build !unix

package manager

import (
	"os"
	"path/filepath"

	"github.com/iota-uz/iota-sdk/pkg/pykernel/isolation"
)

// prepareState is the unconfined fallback for platforms without the sandbox.
func prepareState(workdir string, _ *isolation.Credential, writeMarker bool) (bool, error) {
	ctrl := filepath.Join(workdir, stateDir)
	if err := os.MkdirAll(filepath.Join(ctrl, "tmp"), 0o700); err != nil {
		return false, err
	}
	_, err := os.Lstat(filepath.Join(ctrl, stateFile))
	preserved := err == nil
	if writeMarker {
		if _, err := isolation.WriteFileAtomic(ctrl, stateFile, markerJSON()); err != nil {
			return false, err
		}
	}
	return preserved, nil
}
