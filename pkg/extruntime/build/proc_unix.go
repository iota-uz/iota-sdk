//go:build !windows

package build

import (
	"os/exec"
	"syscall"
)

// ownProcessGroup runs cmd in its own process group and, on cancellation or
// timeout, kills the whole group: the worker's extraction and test children
// must not outlive an abandoned operation.
func ownProcessGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		if cmd.Process == nil {
			return nil
		}
		return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}
}
