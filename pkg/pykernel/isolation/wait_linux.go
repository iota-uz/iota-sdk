//go:build linux

package isolation

import "golang.org/x/sys/unix"

const canWaitWithoutReap = true

// waitExited blocks until pid has exited without reaping it (WNOWAIT), so the
// zombie still holds the pid and its process group id.
func waitExited(pid int) {
	var info unix.Siginfo
	for {
		err := unix.Waitid(unix.P_PID, pid, &info, unix.WEXITED|unix.WNOWAIT, nil)
		if err != unix.EINTR {
			return
		}
	}
}
