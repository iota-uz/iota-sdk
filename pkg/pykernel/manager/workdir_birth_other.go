//go:build !linux && !darwin

package manager

// dirBirth is unavailable here; device and inode alone identify a directory.
func dirBirth(string) int64 { return 0 }
