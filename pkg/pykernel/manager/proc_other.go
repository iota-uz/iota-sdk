//go:build !linux

package manager

// atRest cannot be observed here; only supervised (Linux) kernels settle.
func atRest(int) bool { return true }

func groupStopped(int) bool { return true }
