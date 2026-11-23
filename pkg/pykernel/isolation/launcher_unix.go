//go:build unix

package isolation

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
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
	if spec.Confinement != nil {
		// Never hand a host descriptor (a TTY, a log file) to a confined
		// child: os/exec passes an *os.File through as is, so hide it behind
		// a plain writer and the child gets a pipe instead.
		cmd.Stdout = pipeOnly(spec.Stdout)
		cmd.Stderr = pipeOnly(spec.Stderr)
	}
	cmd.ExtraFiles = spec.ExtraFiles // inherited starting at fd 3

	// Force an explicit environment. A nil cmd.Env would inherit the host's
	// environment (and its secrets); an empty non-nil slice yields none.
	cmd.Env = append([]string{}, spec.Env...)

	// Own process group so a timeout or kill reaps the whole subtree. A
	// confined child cannot leave it: seccomp refuses setsid and setpgid.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	limits := spec.Limits
	if c := spec.Confinement; c != nil {
		uid := uint32(os.Getuid())
		if c.RunAs != nil {
			uid = c.RunAs.UID
			cmd.SysProcAttr.Credential = &syscall.Credential{Uid: c.RunAs.UID, Gid: c.RunAs.GID, NoSetGroups: true}
		}
		if limits.MaxProcs == 0 && limits.ProcHeadroom > 0 {
			limits.MaxProcs = countTasks(uid) + limits.ProcHeadroom
		}
		kv, err := c.env(limits)
		if err != nil {
			return nil, fmt.Errorf("pykernel/isolation: encode confinement: %w", err)
		}
		cmd.Env = append(cmd.Env, kv)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("pykernel/isolation: start: %w", err)
	}

	// Apply rlimits to the started child via prlimit (Linux only). A confined
	// child sets the same limits on itself before it reads any user code, so
	// the window between Start and this call never runs untrusted code; for an
	// unconfined child this is the only enforcement. On non-Linux it is a no-op.
	// A confined child that runs as another uid may not be prlimit-able by a
	// host without CAP_SYS_RESOURCE (root in a container); its own limits,
	// verified in the ready handshake, are the enforcement then.
	if err := applyRlimits(cmd.Process.Pid, limits); err != nil && spec.Confinement == nil {
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

	p := &unixProcess{cmd: cmd}
	if canWaitWithoutReap {
		p.exited = make(chan struct{})
		go func() {
			waitExited(cmd.Process.Pid)
			close(p.exited)
		}()
	}
	return p, nil
}

// unixProcess guards the group kill against pid reuse: once the leader is
// reaped its pid (and group id) may belong to an unrelated process, so Kill
// and Signal become no-ops then. Wait first waits for the exit without
// reaping, marks the process reaped under the lock, and only then reaps.
type unixProcess struct {
	cmd    *exec.Cmd
	mu     sync.Mutex
	reaped bool
	exited chan struct{}
}

// Exited is closed when the process has exited, before it is reaped; nil
// (never ready) where that cannot be observed.
func (p *unixProcess) Exited() <-chan struct{} { return p.exited }

func (p *unixProcess) PID() int { return p.cmd.Process.Pid }

func (p *unixProcess) Wait() error {
	waitExited(p.cmd.Process.Pid)
	p.mu.Lock()
	p.reaped = true
	p.mu.Unlock()
	return p.cmd.Wait()
}

// Signal sends sig to the process. For a syscall.Signal it targets the whole
// process group (negative pid); any other os.Signal falls back to
// cmd.Process.Signal, which targets only the leader process.
func (p *unixProcess) Signal(sig os.Signal) error {
	ssig, ok := sig.(syscall.Signal)
	if !ok {
		return p.cmd.Process.Signal(sig)
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.reaped {
		return nil
	}
	// Negative pid targets the process group.
	if err := syscall.Kill(-p.cmd.Process.Pid, ssig); err != nil && err != syscall.ESRCH {
		return err
	}
	return nil
}

func (p *unixProcess) Kill() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.reaped {
		return nil
	}
	return killGroup(p.cmd.Process.Pid)
}

func killGroup(pid int) error {
	if err := syscall.Kill(-pid, syscall.SIGKILL); err != nil && err != syscall.ESRCH {
		return err
	}
	return nil
}

type writerOnly struct{ w io.Writer }

func (w writerOnly) Write(p []byte) (int, error) { return w.w.Write(p) }

func pipeOnly(w io.Writer) io.Writer {
	if w == nil {
		return nil
	}
	if _, isFile := w.(*os.File); isFile {
		return writerOnly{w}
	}
	return w
}
