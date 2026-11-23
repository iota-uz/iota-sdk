// Package isolation launches the Python kernel subprocess under OS-level
// confinement and reports which boundaries are really in force.
//
// A confined kernel (Mode ModeRequired, the default) applies, before the first
// instruction of user code and irreversibly for itself and its children:
//
//   - Landlock (ABI >= 6): read-only runtime paths, a read-write session
//     workdir and nothing else; TCP bind/connect denied; abstract unix sockets
//     and signals scoped to the sandbox;
//   - seccomp: no new sockets of any family, ptrace, namespaces, mounts, bpf,
//     keyrings, perf, io_uring, process_vm_* or setsid/setpgid;
//   - no_new_privs and an empty capability set;
//   - rlimits (AS, CPU, FSIZE, NPROC, NOFILE) set soft == hard by the child
//     itself, so there is no window between spawn and prlimit.
//
// The host adds an explicit environment allowlist (no DSNs, keys or tokens),
// no inherited descriptors but stdio and the bridge, a process group reaped on
// timeout, a wall-clock timeout and a workdir quota. The only path to data is a
// capability call over the bridge.
//
// Check reports what the host kernel supports and, given an interpreter, what a
// real confined child actually applied. A consumer runs untrusted code only
// when Report.Err() is nil; pykernel/manager refuses to start otherwise.
package isolation

import (
	"context"
	"errors"
	"io"
	"os"
	"strings"
	"time"
)

// ErrUnsupported is returned by Launch on platforms without sandbox support.
var ErrUnsupported = errors.New("pykernel/isolation: sandbox launch not supported on this platform")

// ResourceLimits are the per-kernel resource caps. For a confined kernel every
// zero field takes its DefaultLimits value; for an unconfined one a zero field
// means that limit is not applied. rlimits are enforced on Linux and are a
// no-op on other unixes (development only).
type ResourceLimits struct {
	// WallClock is the host-enforced per-exec timeout: the default for an exec
	// that sets none and the cap for one that asks for more. It is NOT an
	// rlimit (it catches IO-bound hangs that never burn CPU); the Manager owns
	// the timer.
	WallClock time.Duration
	// CPUSeconds maps to RLIMIT_CPU.
	CPUSeconds uint64
	// AddressSpaceBytes maps to RLIMIT_AS (virtual memory).
	AddressSpaceBytes uint64
	// FileSizeBytes maps to RLIMIT_FSIZE.
	FileSizeBytes uint64
	// MaxProcs maps to RLIMIT_NPROC (blocks fork bombs). The kernel counts it
	// against every task of the kernel's uid, so prefer ProcHeadroom.
	MaxProcs uint64
	// ProcHeadroom, when MaxProcs is zero, sets RLIMIT_NPROC at launch to the
	// number of tasks the kernel's uid already runs plus this headroom.
	ProcHeadroom uint64
	// MaxOpenFiles maps to RLIMIT_NOFILE.
	MaxOpenFiles uint64
	// WorkdirBytes is the session workdir quota. The Manager measures the
	// workdir while an exec runs and stops an exec that grows it past the
	// quota; FileSizeBytes bounds the overshoot of a single file.
	WorkdirBytes int64
	// WorkdirEntries caps the number of files and directories in the workdir
	// (inode exhaustion, and the cost of measuring it).
	WorkdirEntries int64
}

// SandboxSpec is the concrete launch profile, built host-side from the Session
// and the lifecycle policy. Nothing here originates from kernel input.
type SandboxSpec struct {
	// Command is the argv to execute, e.g. ["python3", "-s", "/run/pykernel/bootstrap.py"].
	Command []string
	// Workdir is the child's working directory (a 0700 directory on the
	// persistent volume).
	Workdir string
	// Env is the EXACT environment handed to the child — an allowlist with no
	// DB DSNs or secrets. The launcher never appends the host environment; a
	// nil Env yields an empty child environment (not the host's).
	Env []string
	// Limits are the resource caps applied to the child.
	Limits ResourceLimits
	// Nice is an optional scheduling niceness for the child (0 = default).
	Nice int
	// Stdout and Stderr, if set, receive the child's standard streams; nil
	// discards them. (The control protocol uses a separate channel, so stdout
	// stays a clean user-output stream.)
	Stdout, Stderr io.Writer
	// ExtraFiles are additional open files passed to the child, inherited
	// starting at fd 3 (matching os/exec.Cmd.ExtraFiles). The kernel control
	// socket is passed this way so it needs no filesystem path.
	ExtraFiles []*os.File
	// Confinement, if set, is the sandbox the child applies to itself before
	// running untrusted code (sent in EnvSandbox), and the identity it runs as.
	// The child refuses to continue, and the host refuses the child, when a
	// required boundary is missing.
	Confinement *Confinement
}

// Process is a handle to a launched sandbox subprocess.
type Process interface {
	// PID is the subprocess id.
	PID() int
	// Wait blocks until the process exits and returns its exit error.
	Wait() error
	// Signal sends sig to the process group.
	Signal(sig os.Signal) error
	// Kill SIGKILLs the whole process group; it is idempotent.
	Kill() error
}

// Launcher starts sandboxed subprocesses.
type Launcher interface {
	Launch(ctx context.Context, spec SandboxSpec) (Process, error)
}

// NewLauncher returns the platform Launcher.
func NewLauncher() Launcher { return newLauncher() }

// AllowedEnv returns the os.Environ entries whose key is in keys, for building
// an explicit allowlist (e.g. PATH, LANG). It never returns a variable that is
// not named in keys, so secrets stay out unless explicitly allow-listed.
func AllowedEnv(keys ...string) []string {
	allow := make(map[string]struct{}, len(keys))
	for _, k := range keys {
		allow[k] = struct{}{}
	}
	var out []string
	for _, kv := range os.Environ() {
		if eq := strings.IndexByte(kv, '='); eq >= 0 {
			if _, ok := allow[kv[:eq]]; ok {
				out = append(out, kv)
			}
		}
	}
	return out
}
