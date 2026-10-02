//go:build !unix

package process

import (
	"os"
	"os/exec"
)

func ownProcessGroup(command *exec.Cmd)      {}
func interruptOwned(command *exec.Cmd) error { return command.Process.Signal(os.Interrupt) }
func killOwned(command *exec.Cmd) error      { return command.Process.Kill() }
