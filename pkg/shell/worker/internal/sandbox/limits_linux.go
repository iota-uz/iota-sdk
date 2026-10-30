//go:build linux

package sandbox

import (
	"os"
	"syscall"
)

// applyLimits makes the child the first victim of the container OOM killer and
// caps the process itself: no host file can be written, no core dump is
// produced and file descriptors are scarce. RLIMIT_AS is not used: the Go
// runtime reserves address space far beyond its heap and fails under it.
func applyLimits() error {
	if err := os.WriteFile("/proc/self/oom_score_adj", []byte("1000"), 0); err != nil {
		return err
	}
	for resource, value := range map[int]uint64{
		syscall.RLIMIT_FSIZE:  0,
		syscall.RLIMIT_CORE:   0,
		syscall.RLIMIT_NOFILE: 64,
	} {
		if err := syscall.Setrlimit(resource, &syscall.Rlimit{Cur: value, Max: value}); err != nil {
			return err
		}
	}
	return nil
}
