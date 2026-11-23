//go:build !linux

package isolation

// ReapGroup is a no-op outside Linux.
func ReapGroup(int) int { return 0 }

func emptyGroup(int) {}
