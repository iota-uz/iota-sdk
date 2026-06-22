// Package isolation launches the Python kernel subprocess under OS-level
// confinement: resource limits (rlimits on Linux), a dedicated process group,
// a jailed working directory, and a scrubbed environment that contains no
// database credentials or secrets.
//
// The environment scrub is the load-bearing control: because the child receives
// no DSNs and (in production) no network egress, importing a database driver
// inside the kernel is inert — the only path to data is a capability call over
// the bridge. rlimits and the wall-clock timeout bound resource use; the
// process group lets a timeout or kill reap the whole subtree.
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

// ResourceLimits are the per-kernel resource caps. A zero field means that
// limit is not applied. They are enforced as OS rlimits on Linux and are a
// no-op on other unixes (development only).
type ResourceLimits struct {
	// WallClock is the host-enforced timeout. It is NOT an rlimit (it catches
	// IO-bound hangs that never burn CPU); the launcher records it for context
	// while the Manager owns the actual timer.
	WallClock time.Duration
	// CPUSeconds maps to RLIMIT_CPU.
	CPUSeconds uint64
	// AddressSpaceBytes maps to RLIMIT_AS (virtual memory).
	AddressSpaceBytes uint64
	// FileSizeBytes maps to RLIMIT_FSIZE.
	FileSizeBytes uint64
	// MaxProcs maps to RLIMIT_NPROC (blocks fork bombs).
	MaxProcs uint64
	// MaxOpenFiles maps to RLIMIT_NOFILE.
	MaxOpenFiles uint64
}

// SandboxSpec is the concrete launch profile, built host-side from the Session
// and the lifecycle policy. Nothing here originates from kernel input.
type SandboxSpec struct {
	// Command is the argv to execute, e.g. ["python3", ".pykernel/bootstrap.py"].
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
