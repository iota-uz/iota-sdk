//go:build darwin

package manager

import "golang.org/x/sys/unix"

// dirBirth returns the birth time of path in Unix nanoseconds, or 0 when it
// cannot be read.
func dirBirth(path string) int64 {
	var st unix.Stat_t
	if err := unix.Lstat(path, &st); err != nil {
		return 0
	}
	return st.Btim.Nano()
}
