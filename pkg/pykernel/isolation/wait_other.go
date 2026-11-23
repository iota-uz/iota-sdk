//go:build unix && !linux

package isolation

const canWaitWithoutReap = false

// waitExited is a no-op outside Linux; Wait then reaps directly.
func waitExited(int) {}
