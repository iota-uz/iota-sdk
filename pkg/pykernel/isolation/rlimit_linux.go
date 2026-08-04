//go:build linux

package isolation

import "golang.org/x/sys/unix"

// applyRlimits sets each non-zero limit on the child by pid via prlimit(2).
// Setting on the running child (rather than the parent) keeps the host process
// unconstrained.
func applyRlimits(pid int, lim ResourceLimits) error {
	set := func(resource int, val uint64) error {
		if val == 0 {
			return nil
		}
		rl := &unix.Rlimit{Cur: val, Max: val}
		return unix.Prlimit(pid, resource, rl, nil)
	}
	if err := set(unix.RLIMIT_CPU, lim.CPUSeconds); err != nil {
		return err
	}
	if err := set(unix.RLIMIT_AS, lim.AddressSpaceBytes); err != nil {
		return err
	}
	if err := set(unix.RLIMIT_FSIZE, lim.FileSizeBytes); err != nil {
		return err
	}
	if err := set(unix.RLIMIT_NPROC, lim.MaxProcs); err != nil {
		return err
	}
	if err := set(unix.RLIMIT_NOFILE, lim.MaxOpenFiles); err != nil {
		return err
	}
	return nil
}
