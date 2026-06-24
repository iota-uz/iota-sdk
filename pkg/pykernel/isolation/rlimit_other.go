//go:build unix && !linux

package isolation

// applyRlimits is a no-op on non-Linux unix systems (darwin/bsd), which the
// project uses only for development. rlimit enforcement is a Linux-only control
// (applied via prlimit in production); there is no other limit enforcement on
// these platforms.
func applyRlimits(_ int, _ ResourceLimits) error { return nil }
