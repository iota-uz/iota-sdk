package execute

import (
	"context"
	"testing"

	"github.com/iota-uz/iota-sdk/pkg/pykernel"
	"github.com/stretchr/testify/assert"
)

type infoKernel struct{ info pykernel.KernelInfo }

func (k *infoKernel) Exec(context.Context, pykernel.ExecRequest) (<-chan pykernel.ExecEvent, error) {
	return nil, pykernel.ErrKernelDisposed
}
func (k *infoKernel) Info() pykernel.KernelInfo     { return k.info }
func (k *infoKernel) Dispose(context.Context) error { return nil }

func TestNamespace_ReportsFilesContinuityIndependently(t *testing.T) {
	r := &Runner{generations: map[string]generation{}}
	first := &infoKernel{}
	ns := r.namespace("s", "id", first)
	assert.False(t, ns.Reset)
	assert.False(t, ns.FilesPreserved, "a brand-new session has no earlier files")

	same := r.namespace("s", "id", first)
	assert.Equal(t, ns.Generation, same.Generation)
	assert.True(t, same.FilesPreserved, "the same kernel keeps its workdir")

	// Evicted, respawned on the same workdir directory.
	kept := &infoKernel{info: pykernel.KernelInfo{NamespaceReset: true, FilesPreserved: true}}
	ns = r.namespace("s", "id", kept)
	assert.True(t, ns.Reset)
	assert.True(t, ns.FilesPreserved)

	// Evicted, and the workdir was replaced: the reset is reported, files are not.
	replaced := &infoKernel{info: pykernel.KernelInfo{NamespaceReset: true, FilesPreserved: false}}
	ns = r.namespace("s", "id", replaced)
	assert.True(t, ns.Reset)
	assert.False(t, ns.FilesPreserved)
}
