//go:build unix && !linux

package supervisor

import "syscall"

func childProcAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{Setpgid: true}
}
