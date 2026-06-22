//go:build unix && !linux

package isolation

// applyRlimits is a no-op on non-Linux unix systems (darwin/bsd), which the
// project uses only for development. rlimit enforcement is a Linux-only control
// in production; the kernel shim additionally self-applies limits via Python's
// resource module where supported.
func applyRlimits(_ int, _ ResourceLimits) error { return nil }
