//go:build unix && !linux

package isolation

// waitExited is a no-op outside Linux; Wait then reaps directly.
func waitExited(int) {}
