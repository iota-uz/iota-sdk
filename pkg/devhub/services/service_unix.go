//go:build !windows

package services

import (
	"syscall"
)

func setSysProcAttr(attr *syscall.SysProcAttr) {
	attr.Setpgid = true
}

func killProcessGroup(pid int) error {
	return syscall.Kill(-pid, syscall.SIGKILL)
}
