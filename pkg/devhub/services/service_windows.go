//go:build windows

package services

import (
	"fmt"
	"syscall"
)

func setSysProcAttr(attr *syscall.SysProcAttr) {
	// Windows doesn't support Setpgid
}

func killProcessGroup(pid int) error {
	// On Windows, we can't kill a process group the same way
	// This is a simplified version - in production you might want to use Windows Job Objects
	return fmt.Errorf("process group kill not implemented on Windows")
}
