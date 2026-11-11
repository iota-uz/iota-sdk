//go:build windows

package build

import "os/exec"

func ownProcessGroup(*exec.Cmd) {}
