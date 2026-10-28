//go:build !linux

package sandbox

// applyLimits is a no-op outside Linux, which is used only for development;
// the memory watchdog and the supervisor deadline still apply.
func applyLimits() error { return nil }
