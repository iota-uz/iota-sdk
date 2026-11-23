package isolation

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

//go:embed sandbox.py
var sandboxPy []byte

// ShimFilename is the sandbox module's filename. The kernel bootstrap loads it
// from its own directory, and Check runs it with --probe.
const ShimFilename = "sandbox.py"

// EnvSandbox carries the JSON-encoded Confinement to the child.
const EnvSandbox = "PYKERNEL_SANDBOX"

// MinLandlockABI is the lowest Landlock ABI that provides every required
// boundary: ABI 4 adds TCP rules, ABI 6 scopes abstract unix sockets and
// signals.
const MinLandlockABI = 6

// WriteShim materializes the embedded sandbox module into dir (created 0700
// if absent) and returns its path. The write is atomic so concurrent spawns
// never read a partial file.
func WriteShim(dir string) (string, error) {
	return WriteFileAtomic(dir, ShimFilename, sandboxPy)
}

// WriteFileAtomic writes data to dir/name through a temporary file and a
// rename, creating dir 0700 if absent, and returns the final path.
func WriteFileAtomic(dir, name string, data []byte) (string, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("pykernel/isolation: mkdir %q: %w", dir, err)
	}
	tmp, err := os.CreateTemp(dir, "."+name+".*")
	if err != nil {
		return "", fmt.Errorf("pykernel/isolation: write %q: %w", name, err)
	}
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmp.Name())
		return "", fmt.Errorf("pykernel/isolation: write %q: %w", name, err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmp.Name())
		return "", fmt.Errorf("pykernel/isolation: write %q: %w", name, err)
	}
	if err := os.Chmod(tmp.Name(), 0o644); err != nil {
		_ = os.Remove(tmp.Name())
		return "", fmt.Errorf("pykernel/isolation: chmod %q: %w", name, err)
	}
	path := filepath.Join(dir, name)
	if err := os.Rename(tmp.Name(), path); err != nil {
		_ = os.Remove(tmp.Name())
		return "", fmt.Errorf("pykernel/isolation: rename %q: %w", name, err)
	}
	return path, nil
}

// Mode selects how strictly a kernel is confined.
type Mode int

const (
	// ModeRequired confines every kernel and refuses to run one that is
	// missing any boundary in RequiredBoundaries (fail closed). It is the
	// zero value, so a Manager is confined unless it opts out explicitly.
	ModeRequired Mode = iota
	// ModeInsecureDisabled runs kernels with only the environment scrub,
	// rlimits and the process group. It exists for development on hosts
	// without Landlock (macOS, old kernels) and for tests with fake
	// launchers; never use it for code a model or a user wrote.
	ModeInsecureDisabled
)

func (m Mode) String() string {
	switch m {
	case ModeRequired:
		return "required"
	case ModeInsecureDisabled:
		return "insecure-disabled"
	default:
		return fmt.Sprintf("Mode(%d)", int(m))
	}
}

// Credential is the unprivileged identity a kernel runs as. A host running as
// root must set one: RLIMIT_NPROC is not enforced for uid 0, so a root kernel
// could exhaust the host's process table.
type Credential struct {
	UID uint32
	GID uint32
}

// Profile is the host's sandbox configuration for every kernel of a Manager.
type Profile struct {
	// Mode is ModeRequired unless set otherwise.
	Mode Mode
	// ReadPaths are extra read-only paths (fonts, reference data) on top of
	// DefaultReadPaths and the interpreter's own stdlib and site-packages.
	ReadPaths []string
	// RunAs, if set, is the uid/gid the kernel runs as. The Manager hands the
	// session workdir to it before each lease.
	RunAs *Credential
	// StartTimeout bounds how long a spawned kernel may take to confine
	// itself and report ready (default 20s).
	StartTimeout time.Duration
}

// DefaultReadPaths are the shared-library and timezone paths a CPython kernel
// reads after the restriction. Paths that do not exist are skipped.
func DefaultReadPaths() []string {
	return []string{
		"/lib", "/lib64", "/usr/lib", "/usr/lib64", "/usr/local/lib",
		"/etc/ld.so.cache", "/etc/localtime", "/usr/share/zoneinfo",
		"/sys/devices/system/cpu",
	}
}

// DefaultLimits are the resource caps a confined kernel gets for every field a
// Manager leaves zero.
func DefaultLimits() ResourceLimits {
	return ResourceLimits{
		WallClock:         30 * time.Minute,
		CPUSeconds:        600,
		AddressSpaceBytes: 4 << 30,
		FileSizeBytes:     512 << 20,
		ProcHeadroom:      64,
		MaxOpenFiles:      256,
		WorkdirBytes:      1 << 30,
		WorkdirEntries:    100_000,
	}
}

// WithDefaults fills every zero field of l from DefaultLimits.
func (l ResourceLimits) WithDefaults() ResourceLimits {
	d := DefaultLimits()
	if l.WallClock == 0 {
		l.WallClock = d.WallClock
	}
	if l.CPUSeconds == 0 {
		l.CPUSeconds = d.CPUSeconds
	}
	if l.AddressSpaceBytes == 0 {
		l.AddressSpaceBytes = d.AddressSpaceBytes
	}
	if l.FileSizeBytes == 0 {
		l.FileSizeBytes = d.FileSizeBytes
	}
	if l.MaxProcs == 0 && l.ProcHeadroom == 0 {
		l.ProcHeadroom = d.ProcHeadroom
	}
	if l.MaxOpenFiles == 0 {
		l.MaxOpenFiles = d.MaxOpenFiles
	}
	if l.WorkdirBytes == 0 {
		l.WorkdirBytes = d.WorkdirBytes
	}
	if l.WorkdirEntries == 0 {
		l.WorkdirEntries = d.WorkdirEntries
	}
	return l
}

// Confinement is the per-launch sandbox the child applies to itself before it
// runs any untrusted code. The launcher sends it in EnvSandbox.
type Confinement struct {
	// Workdir is the only read-write path.
	Workdir string
	// ReadPaths are read-only; ExecPaths are read-only and executable.
	ReadPaths []string
	ExecPaths []string
	// RunAs is the identity the child is started with (nil: the host's).
	RunAs *Credential
}

type confinementWire struct {
	Workdir        string            `json:"workdir"`
	ReadPaths      []string          `json:"read_paths"`
	ExecPaths      []string          `json:"exec_paths,omitempty"`
	MinLandlockABI int               `json:"min_landlock_abi"`
	Rlimits        map[string]uint64 `json:"rlimits"`
}

func (c *Confinement) env(lim ResourceLimits) (string, error) {
	raw, err := json.Marshal(confinementWire{
		Workdir:        c.Workdir,
		ReadPaths:      c.ReadPaths,
		ExecPaths:      c.ExecPaths,
		MinLandlockABI: MinLandlockABI,
		Rlimits: map[string]uint64{
			"cpu":    lim.CPUSeconds,
			"as":     lim.AddressSpaceBytes,
			"fsize":  lim.FileSizeBytes,
			"nproc":  lim.MaxProcs,
			"nofile": lim.MaxOpenFiles,
		},
	})
	if err != nil {
		return "", err
	}
	return EnvSandbox + "=" + string(raw), nil
}

// Boundary names one isolation guarantee.
type Boundary string

const (
	BoundaryNoNewPrivs     Boundary = "no_new_privs"
	BoundaryNoCapabilities Boundary = "no_capabilities"
	// BoundaryLandlockFS: read-only runtime, read-write session workdir,
	// nothing else (no /proc, $HOME, /etc beyond the loader, app binary).
	BoundaryLandlockFS Boundary = "landlock_fs"
	// BoundaryLandlockNet: TCP bind and connect denied.
	BoundaryLandlockNet Boundary = "landlock_net"
	// BoundaryLandlockScope: abstract unix sockets and signals scoped to the
	// sandbox (no connecting out, no signalling the host).
	BoundaryLandlockScope Boundary = "landlock_scope"
	// BoundarySeccomp: no new sockets of any family, no ptrace, namespaces,
	// mounts, bpf, keyrings, perf, io_uring, process_vm_* or setsid/setpgid.
	BoundarySeccomp Boundary = "seccomp"
	// BoundaryRlimits: AS, CPU, FSIZE, NPROC and NOFILE set before user code.
	BoundaryRlimits Boundary = "rlimits"
	// BoundaryNPROC: the kernel runs as a uid for which RLIMIT_NPROC holds.
	BoundaryNPROC Boundary = "nproc"
	// BoundaryWorkdirQuota: every file write, extension and entry creation is
	// approved by the host against the workdir quota before it happens
	// (seccomp user notification), counting unlinked files still held open.
	BoundaryWorkdirQuota Boundary = "workdir_quota"
)

// RequiredBoundaries is the full set a confined kernel must have.
var RequiredBoundaries = []Boundary{
	BoundaryNoNewPrivs, BoundaryNoCapabilities, BoundaryLandlockFS, BoundaryLandlockNet,
	BoundaryLandlockScope, BoundarySeccomp, BoundaryRlimits, BoundaryNPROC, BoundaryWorkdirQuota,
}

// BoundaryStatus is whether one boundary is in force, with the reason.
type BoundaryStatus struct {
	Boundary Boundary `json:"boundary"`
	Enabled  bool     `json:"enabled"`
	Detail   string   `json:"detail,omitempty"`
}

// Report states which boundaries are really in force.
type Report struct {
	// Platform is GOOS/GOARCH of the host.
	Platform string `json:"platform"`
	// LandlockABI is the kernel's Landlock ABI version (0: unavailable).
	LandlockABI int `json:"landlock_abi"`
	// Boundaries lists every required boundary in RequiredBoundaries order.
	Boundaries []BoundaryStatus `json:"boundaries"`
	// Verified is true when a real child applied the sandbox and probed it,
	// not only the host kernel's support for it.
	Verified bool `json:"verified"`
}

// Enabled reports whether boundary b is in force.
func (r Report) Enabled(b Boundary) bool {
	for _, s := range r.Boundaries {
		if s.Boundary == b {
			return s.Enabled
		}
	}
	return false
}

// Missing lists the required boundaries that are not in force.
func (r Report) Missing() []BoundaryStatus {
	var out []BoundaryStatus
	for _, b := range RequiredBoundaries {
		found := false
		for _, s := range r.Boundaries {
			if s.Boundary == b {
				found = true
				if !s.Enabled {
					out = append(out, s)
				}
			}
		}
		if !found {
			out = append(out, BoundaryStatus{Boundary: b, Detail: "not reported"})
		}
	}
	return out
}

// Complete reports whether every required boundary is in force.
func (r Report) Complete() bool { return len(r.Missing()) == 0 }

// Err is nil when the report is complete and verified by a child, otherwise an
// *IncompleteError listing what is missing.
func (r Report) Err() error {
	missing := r.Missing()
	if len(missing) == 0 && r.Verified {
		return nil
	}
	return &IncompleteError{Missing: missing, Unverified: !r.Verified}
}

// ErrIsolationUnavailable matches every *IncompleteError.
var ErrIsolationUnavailable = errors.New("pykernel/isolation: required sandbox boundary unavailable")

// IncompleteError lists the required boundaries that are not in force.
type IncompleteError struct {
	Missing    []BoundaryStatus
	Unverified bool
}

func (e *IncompleteError) Error() string {
	parts := make([]string, 0, len(e.Missing)+1)
	for _, m := range e.Missing {
		parts = append(parts, fmt.Sprintf("%s (%s)", m.Boundary, m.Detail))
	}
	if e.Unverified {
		parts = append(parts, "not verified by a confined child")
	}
	return ErrIsolationUnavailable.Error() + ": " + strings.Join(parts, "; ")
}

func (e *IncompleteError) Is(target error) bool { return target == ErrIsolationUnavailable }

// childReport is the JSON the sandbox module prints with --probe and sends in
// the kernel.ready handshake.
type childReport struct {
	Boundaries map[string]struct {
		Enabled bool   `json:"enabled"`
		Detail  string `json:"detail"`
	} `json:"boundaries"`
	LandlockABI int    `json:"landlock_abi"`
	Error       string `json:"error"`
}

// ParseChildReport decodes the sandbox module's report into a Report.
func ParseChildReport(raw []byte) (Report, error) {
	var cr childReport
	if err := json.Unmarshal(raw, &cr); err != nil {
		return Report{}, fmt.Errorf("pykernel/isolation: decode sandbox report: %w", err)
	}
	r := Report{Platform: runtime.GOOS + "/" + runtime.GOARCH, LandlockABI: cr.LandlockABI, Verified: true}
	for _, b := range RequiredBoundaries {
		s, ok := cr.Boundaries[string(b)]
		st := BoundaryStatus{Boundary: b, Enabled: ok && s.Enabled, Detail: s.Detail}
		if !ok {
			st.Detail = "not reported by the child"
			if cr.Error != "" {
				st.Detail = cr.Error
			}
		}
		r.Boundaries = append(r.Boundaries, st)
	}
	return r, nil
}

// CheckOptions configures Check.
type CheckOptions struct {
	// PythonPath, if set, makes Check start a real confined child with it and
	// report what the child actually applied (Verified). Without it only the
	// host kernel's support is reported.
	PythonPath string
	// Profile is the sandbox profile the child is confined with.
	Profile Profile
	// Limits are the resource caps (zero fields take DefaultLimits).
	Limits ResourceLimits
	// Launcher starts the child (default NewLauncher()).
	Launcher Launcher
}

// Check reports which isolation boundaries are really in force on this host.
// With a PythonPath it confines a real child exactly like a kernel and returns
// the child's verified report; a consumer enables untrusted execution only when
// Report.Err() is nil.
func Check(ctx context.Context, opts CheckOptions) Report {
	host := hostReport(opts.Profile)
	if opts.PythonPath == "" || !host.Complete() {
		return host
	}
	child, err := probeChild(ctx, opts)
	if err != nil {
		for i := range host.Boundaries {
			host.Boundaries[i].Enabled = false
			host.Boundaries[i].Detail = err.Error()
		}
		return host
	}
	return child
}

func probeChild(ctx context.Context, opts CheckOptions) (Report, error) {
	dir, err := os.MkdirTemp("", "pykernel-check-")
	if err != nil {
		return Report{}, fmt.Errorf("probe workdir: %w", err)
	}
	defer func() { _ = os.RemoveAll(dir) }()
	rt := filepath.Join(dir, "runtime")
	shim, err := WriteShim(rt)
	if err != nil {
		return Report{}, err
	}
	work := filepath.Join(dir, "work")
	if err := os.MkdirAll(work, 0o700); err != nil {
		return Report{}, fmt.Errorf("probe workdir: %w", err)
	}
	if opts.Profile.RunAs != nil {
		if err := os.Chmod(dir, 0o755); err != nil {
			return Report{}, err
		}
		if err := os.Chmod(rt, 0o755); err != nil {
			return Report{}, err
		}
		// A fresh, empty directory nobody else writes.
		if err := os.Lchown(work, int(opts.Profile.RunAs.UID), int(opts.Profile.RunAs.GID)); err != nil {
			return Report{}, err
		}
	}
	l := opts.Launcher
	if l == nil {
		l = NewLauncher()
	}
	timeout := opts.Profile.StartTimeout
	if timeout <= 0 {
		timeout = 20 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	var stdout, stderr bytes.Buffer
	proc, err := l.Launch(ctx, SandboxSpec{
		Command: []string{opts.PythonPath, "-s", shim, "--probe"},
		Workdir: work,
		Env:     AllowedEnv("PATH", "LANG", "LC_ALL"),
		Limits:  opts.Limits.WithDefaults(),
		Stdout:  &stdout,
		Stderr:  &stderr,
		Confinement: &Confinement{
			Workdir:   work,
			ReadPaths: append(append(DefaultReadPaths(), rt), opts.Profile.ReadPaths...),
			RunAs:     opts.Profile.RunAs,
		},
	})
	if err != nil {
		return Report{}, fmt.Errorf("start probe: %w", err)
	}
	waitErr := make(chan error, 1)
	go func() { waitErr <- proc.Wait() }()
	select {
	case <-waitErr:
	case <-ctx.Done():
		_ = proc.Kill()
		<-waitErr
		return Report{}, fmt.Errorf("probe did not finish: %w", ctx.Err())
	}
	line := bytes.TrimSpace(stdout.Bytes())
	if i := bytes.LastIndexByte(line, '\n'); i >= 0 {
		line = line[i+1:]
	}
	if len(line) == 0 {
		return Report{}, fmt.Errorf("probe printed no report: %s", strings.TrimSpace(stderr.String()))
	}
	return ParseChildReport(line)
}
