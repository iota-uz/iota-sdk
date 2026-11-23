// Package runtime carries the embedded Python kernel shim and materializes it
// at spawn time. Shipping the shim inside the Go binary (rather than pip-
// installing it) keeps a single source of truth and guarantees the shim and the
// host bridge never disagree on the wire protocol.
package runtime

import (
	_ "embed"

	"github.com/iota-uz/iota-sdk/pkg/pykernel/isolation"
)

//go:embed bootstrap.py
var bootstrapPy []byte

// ShimFilename is the materialized shim's filename inside the runtime directory.
const ShimFilename = "bootstrap.py"

// Write materializes the embedded shim and the sandbox module it loads into
// dir (created 0700 if absent) and returns the shim's path. dir must be a
// host-owned directory outside every kernel workdir: a confined kernel can
// read it but never write it, so code from one exec cannot plant anything
// that runs before the sandbox of the next kernel.
func Write(dir string) (string, error) {
	if _, err := isolation.WriteShim(dir); err != nil {
		return "", err
	}
	return isolation.WriteFileAtomic(dir, ShimFilename, bootstrapPy)
}
