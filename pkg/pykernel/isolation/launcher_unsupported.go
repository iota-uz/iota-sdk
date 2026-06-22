//go:build !unix

package isolation

import "context"

func newLauncher() Launcher { return unsupportedLauncher{} }

type unsupportedLauncher struct{}

func (unsupportedLauncher) Launch(context.Context, SandboxSpec) (Process, error) {
	return nil, ErrUnsupported
}
