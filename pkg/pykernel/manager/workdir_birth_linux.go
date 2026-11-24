//go:build linux

package manager

import "golang.org/x/sys/unix"

// dirBirth returns the birth time of path in Unix nanoseconds, or 0 when the
// file system does not record one.
func dirBirth(path string) int64 {
	var st unix.Statx_t
	if err := unix.Statx(unix.AT_FDCWD, path, unix.AT_SYMLINK_NOFOLLOW, unix.STATX_BTIME, &st); err != nil {
		return 0
	}
	if st.Mask&unix.STATX_BTIME == 0 {
		return 0
	}
	return st.Btime.Sec*1e9 + int64(st.Btime.Nsec)
}
