//go:build unix

package isolation

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"syscall"
)

func newLauncher() Launcher { return &unixLauncher{} }

type unixLauncher struct{}

func (l *unixLauncher) Launch(ctx context.Context, spec SandboxSpec) (Process, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if len(spec.Command) == 0 {
		return nil, fmt.Errorf("pykernel/isolation: empty command")
	}

	cmd := exec.Command(spec.Command[0], spec.Command[1:]...) //nolint:gosec // argv is built host-side, never from kernel input
	cmd.Dir = spec.Workdir
	cmd.Stdout = spec.Stdout
	cmd.Stderr = spec.Stderr
	cmd.ExtraFiles = spec.ExtraFiles // inherited starting at fd 3

	// Force an explicit environment. A nil cmd.Env would inherit the host's
	// environment (and its secrets); an empty non-nil slice yields none.
	cmd.Env = spec.Env
	if cmd.Env == nil {
		cmd.Env = []string{}
	}

	// Own process group so a timeout or kill reaps the whole subtree.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("pykernel/isolation: start: %w", err)
	}

	// Apply rlimits to the started child via prlimit (Linux only). There is a
	// small window between Start and this call during which the child runs
	// without limits; the shim does not yet self-apply limits, so this window
	// is currently unguarded (tracked follow-up). On non-Linux this is a no-op.
	if err := applyRlimits(cmd.Process.Pid, spec.Limits); err != nil {
		_ = killGroup(cmd.Process.Pid)
		_ = cmd.Wait()
		return nil, fmt.Errorf("pykernel/isolation: apply rlimits: %w", err)
	}
	if spec.Nice != 0 {
		_ = syscall.Setpriority(syscall.PRIO_PROCESS, cmd.Process.Pid, spec.Nice)
	}

	// Honor a mid-launch cancellation so we don't leak a just-started subprocess.
	// We do NOT use exec.CommandContext: the kernel must outlive the Acquire
	// call's ctx, so the only ctx coupling is this one-shot pre-handoff check.
	if err := ctx.Err(); err != nil {
		_ = killGroup(cmd.Process.Pid)
		_ = cmd.Wait()
		return nil, err
	}

	return &unixProcess{cmd: cmd}, nil
}

type unixProcess struct{ cmd *exec.Cmd }

func (p *unixProcess) PID() int { return p.cmd.Process.Pid }

func (p *unixProcess) Wait() error { return p.cmd.Wait() }

// Signal sends sig to the process. For a syscall.Signal it targets the whole
// process group (negative pid); any other os.Signal falls back to
// cmd.Process.Signal, which targets only the leader process.
func (p *unixProcess) Signal(sig os.Signal) error {
	ssig, ok := sig.(syscall.Signal)
	if !ok {
		return p.cmd.Process.Signal(sig)
	}
	// Negative pid targets the process group.
	if err := syscall.Kill(-p.cmd.Process.Pid, ssig); err != nil && err != syscall.ESRCH {
		return err
	}
	return nil
}

func (p *unixProcess) Kill() error { return killGroup(p.cmd.Process.Pid) }

func killGroup(pid int) error {
	if err := syscall.Kill(-pid, syscall.SIGKILL); err != nil && err != syscall.ESRCH {
		return err
	}
	return nil
}
