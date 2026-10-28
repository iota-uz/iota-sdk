//go:build linux

package supervisor

import "syscall"

// childProcAttr puts the child in its own process group and kills it if the
// supervisor dies, so a crashed supervisor never leaves a run behind.
func childProcAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{Setpgid: true, Pdeathsig: syscall.SIGKILL}
}
