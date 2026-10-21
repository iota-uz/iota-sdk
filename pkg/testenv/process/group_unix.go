//go:build unix

package process

import (
	"os/exec"
	"syscall"
)

func ownProcessGroup(command *exec.Cmd) { command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true} }
func interruptOwned(command *exec.Cmd) error {
	return syscall.Kill(-command.Process.Pid, syscall.SIGINT)
}
func killOwned(command *exec.Cmd) error { return syscall.Kill(-command.Process.Pid, syscall.SIGKILL) }
