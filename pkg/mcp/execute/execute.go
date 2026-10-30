// Package execute runs model-authored Python against the shared tool registry
// on the SDK pykernel runtime. Every tools.<name>() call inside the script goes
// through the same Engine as a direct call, so policy is enforced per call and
// the script cannot reach anything the caller could not reach directly.
package execute

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/google/uuid"

	"github.com/iota-uz/iota-sdk/pkg/mcp"
	"github.com/iota-uz/iota-sdk/pkg/pykernel"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

const (
	ToolName = "execute"

	StatusSucceeded = "succeeded"
	StatusFailed    = "failed"
	StatusCancelled = "cancelled"
	StatusTimedOut  = "timed_out"

	ErrorPython        = "python_error"
	ErrorCancelled     = "cancelled"
	ErrorTimeout       = "timeout"
	ErrorLimitExceeded = "limit_exceeded"
	ErrorRuntimeLost   = "runtime_lost"

	busyRetryAfter = time.Second
	outputDir      = "out"
)

// Limits come from trusted configuration and are published in the tool
// schema; call arguments may only lower them.
type Limits struct {
	DefaultTimeout     time.Duration
	MaxTimeout         time.Duration
	DefaultOutputBytes int
	MaxOutputBytes     int
	MaxCodeBytes       int
	// MemoryBytes is enforced by the pykernel isolation limits and only
	// published here.
	MemoryBytes int64
}

type Config struct {
	Engine *mcp.Engine
	// Manager is the local Python runtime. Nil means the runtime is disabled:
	// execute answers dependency_unavailable and never falls back elsewhere.
	Manager  pykernel.Manager
	WorkRoot string
	Limits   Limits
	// Artifacts receives files the script leaves in ./out; nil disables them.
	Artifacts        ArtifactStore
	MaxArtifacts     int
	MaxArtifactBytes int64
	// Workspace scopes warm sessions below the account; empty means "default".
	Workspace func(ctx context.Context) string
	Policy    mcp.Policy
}

type Input struct {
	Code           string `json:"code"`
	TimeoutMS      int64  `json:"timeout_ms,omitempty"`
	MaxOutputBytes int    `json:"max_output_bytes,omitempty"`
}

type Truncated struct {
	Emitted bool `json:"emitted"`
	Stdout  bool `json:"stdout"`
	Stderr  bool `json:"stderr"`
}

type Namespace struct {
	ID         string `json:"id"`
	Generation string `json:"generation"`
	Reset      bool   `json:"reset"`
}

type RunError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type Result struct {
	RunID        string            `json:"run_id"`
	Status       string            `json:"status"`
	Emitted      []json.RawMessage `json:"emitted"`
	Stdout       string            `json:"stdout"`
	Stderr       string            `json:"stderr"`
	Truncated    Truncated         `json:"truncated"`
	Artifacts    []Artifact        `json:"artifacts"`
	OperationIDs []string          `json:"operation_ids"`
	DurationMS   int64             `json:"duration_ms"`
	Namespace    Namespace         `json:"namespace"`
	Error        *RunError         `json:"error"`
}

type Runner struct {
	cfg Config

	mu          sync.Mutex
	generations map[string]generation
}

type generation struct {
	kernel pykernel.Kernel
	id     string
}

func NewRunner(cfg Config) (*Runner, error) {
	const op serrors.Op = "execute.NewRunner"
	if cfg.Engine == nil {
		return nil, serrors.New(serrors.Invalid, "engine is required").WithOp(op)
	}
	if cfg.Manager != nil && cfg.WorkRoot == "" {
		return nil, serrors.New(serrors.Invalid, "work root is required with a runtime").WithOp(op)
	}
	l := cfg.Limits
	if l.DefaultTimeout <= 0 || l.MaxTimeout < l.DefaultTimeout || l.DefaultOutputBytes <= 0 ||
		l.MaxOutputBytes < l.DefaultOutputBytes || l.MaxCodeBytes <= 0 {
		return nil, serrors.New(serrors.Invalid, "runtime limits need positive defaults not above their maxima").WithOp(op)
	}
	return &Runner{cfg: cfg, generations: make(map[string]generation)}, nil
}

// Tool registers execute itself; it is host-only so isolated code cannot
// call execute recursively.
func (r *Runner) Tool() mcp.Tool {
	l := r.cfg.Limits
	return &tool{runner: r, spec: mcp.Spec{
		Name: ToolName,
		Description: "Run Python with a typed `tools` namespace generated from the same tools you can call directly. " +
			"Top-level await is supported: `r = await tools.<name>(...)` returns {ok, data} or {ok: false, error}. " +
			"Only values passed to emit(value) reach you as data; stdout and stderr are returned truncated. " +
			"Files written to ./out are returned as artifacts. Writes inside the script are separate operations, not one transaction.",
		Input: &jsonschema.Schema{
			Type:                 "object",
			AdditionalProperties: &jsonschema.Schema{Not: &jsonschema.Schema{}},
			Required:             []string{"code"},
			Properties: map[string]*jsonschema.Schema{
				"code": {Type: "string", MinLength: jsonschema.Ptr(1), MaxLength: jsonschema.Ptr(l.MaxCodeBytes)},
				"timeout_ms": {Type: "integer", Minimum: jsonschema.Ptr(1.0),
					Maximum: jsonschema.Ptr(float64(l.MaxTimeout.Milliseconds())), Default: rawInt(l.DefaultTimeout.Milliseconds())},
				"max_output_bytes": {Type: "integer", Minimum: jsonschema.Ptr(1.0),
					Maximum: jsonschema.Ptr(float64(l.MaxOutputBytes)), Default: rawInt(int64(l.DefaultOutputBytes))},
			},
		},
		Annotations: mcp.Annotations{OpenWorld: false, Destructive: true},
		Policy:      r.cfg.Policy,
		Meta: map[string]any{"limits": map[string]any{
			"timeout_ms":       map[string]int64{"default": l.DefaultTimeout.Milliseconds(), "max": l.MaxTimeout.Milliseconds()},
			"max_output_bytes": map[string]int{"default": l.DefaultOutputBytes, "max": l.MaxOutputBytes},
			"code_bytes":       map[string]int{"max": l.MaxCodeBytes},
			"memory_bytes":     map[string]int64{"max": l.MemoryBytes},
		}},
		HostOnly: true,
	}}
}

func rawInt(n int64) json.RawMessage {
	return json.RawMessage(strconv.FormatInt(n, 10))
}

type tool struct {
	runner *Runner
	spec   mcp.Spec
}

func (t *tool) Spec() mcp.Spec { return t.spec }

func (t *tool) Call(ctx context.Context, input json.RawMessage) (any, error) {
	var in Input
	if err := json.Unmarshal(input, &in); err != nil {
		return nil, mcp.Fail(mcp.CodeInvalidInput, mcp.RecoveryFixInput, "input does not match the tool schema")
	}
	return t.runner.Execute(ctx, in)
}

// Execute runs one bounded script synchronously. Busy rejects before running;
// cancellation and timeouts never undo commits already made by the script,
// whose operation ids are returned so they can be checked instead of rerun.
func (r *Runner) Execute(ctx context.Context, in Input) (Result, error) {
	if r.cfg.Manager == nil {
		return Result{}, mcp.Fail(mcp.CodeDependencyUnavailable, mcp.RecoveryRetryLater, "the Python runtime is not available")
	}
	p, ok := mcp.PrincipalFrom(ctx)
	if !ok {
		return Result{}, mcp.Fail(mcp.CodeForbidden, mcp.RecoveryReauthenticate, "the caller is not authenticated")
	}
	timeout, outputCap, err := r.limits(in)
	if err != nil {
		return Result{}, err
	}

	workspace := "default"
	if r.cfg.Workspace != nil {
		if w := r.cfg.Workspace(ctx); w != "" {
			workspace = w
		}
	}
	key := p.TenantID.String() + "/" + p.Subject() + "/" + workspace
	sum := sha256.Sum256([]byte(key))
	nsID := hex.EncodeToString(sum[:16])
	workdir := filepath.Join(r.cfg.WorkRoot, nsID)
	outDir := filepath.Join(workdir, outputDir)
	if err := os.RemoveAll(outDir); err != nil {
		return Result{}, mcp.Fail(mcp.CodeDependencyUnavailable, mcp.RecoveryRetryLater, "the run workspace is not available")
	}
	if err := os.MkdirAll(outDir, 0o700); err != nil {
		return Result{}, mcp.Fail(mcp.CodeDependencyUnavailable, mcp.RecoveryRetryLater, "the run workspace is not available")
	}

	caps, err := r.capabilities()
	if err != nil {
		return Result{}, err
	}
	k, err := r.cfg.Manager.Acquire(ctx, session{key: key, tenant: p.TenantID, caps: caps, workdir: workdir})
	if errors.Is(err, pykernel.ErrPoolExhausted) {
		return Result{}, mcp.Fail(mcp.CodeBusy, mcp.RecoveryRetryLater, "the runtime is at capacity").
			WithRetryAfter(busyRetryAfter.Milliseconds())
	}
	if err != nil {
		return Result{}, mcp.Fail(mcp.CodeDependencyUnavailable, mcp.RecoveryRetryLater, "the Python runtime is not available")
	}
	defer func() { _ = r.cfg.Manager.Release(k) }()

	res := Result{
		RunID:     uuid.NewString(),
		Emitted:   []json.RawMessage{},
		Artifacts: []Artifact{},
		Namespace: r.namespace(key, nsID, k),
	}
	state := &runState{maxEmit: outputCap}
	started := time.Now()
	events, err := k.Exec(withRun(ctx, state), pykernel.ExecRequest{
		Code: in.Code, Timeout: timeout, OutputCap: outputCap, Label: res.RunID,
		SeparateStderr: true, BindContext: true,
	})
	if errors.Is(err, pykernel.ErrKernelBusy) {
		return Result{}, mcp.Fail(mcp.CodeBusy, mcp.RecoveryRetryLater, "another run is in progress in this session").
			WithRetryAfter(busyRetryAfter.Milliseconds())
	}
	if err != nil {
		return Result{}, mcp.Fail(mcp.CodeDependencyUnavailable, mcp.RecoveryRetryLater, "the Python runtime is not available")
	}

	var stdout, stderr strings.Builder
	var execErr *pykernel.ExecError
	for ev := range events {
		switch ev.Kind {
		case pykernel.EventStdout:
			if ev.Stderr {
				res.Truncated.Stderr = appendCapped(&stderr, ev.Stdout, outputCap) || res.Truncated.Stderr
			} else {
				res.Truncated.Stdout = appendCapped(&stdout, ev.Stdout, outputCap) || res.Truncated.Stdout
			}
		case pykernel.EventError:
			execErr = ev.Err
		case pykernel.EventDone:
			if ev.Done != nil {
				res.Truncated.Stdout = res.Truncated.Stdout || ev.Done.StdoutTruncated
				res.Truncated.Stderr = res.Truncated.Stderr || ev.Done.StderrTruncated
			}
		case pykernel.EventResult, pykernel.EventMetric, pykernel.EventLog, pykernel.EventTruncated:
		}
	}
	res.DurationMS = time.Since(started).Milliseconds()
	res.Stdout, res.Stderr = stdout.String(), stderr.String()
	res.Emitted, res.Truncated.Emitted, res.OperationIDs = state.snapshot()
	res.Status, res.Error = classify(ctx, execErr, time.Since(started) >= timeout)
	if execErr != nil && res.Status == StatusFailed && execErr.Traceback != "" {
		res.Truncated.Stderr = appendCapped(&stderr, []byte(execErr.Traceback), outputCap) || res.Truncated.Stderr
		res.Stderr = stderr.String()
	}
	res.Artifacts = r.collectArtifacts(ctx, outDir)
	return res, nil
}

func (r *Runner) limits(in Input) (time.Duration, int, error) {
	l := r.cfg.Limits
	if len(in.Code) > l.MaxCodeBytes {
		return 0, 0, mcp.Fail(mcp.CodeLimitExceeded, mcp.RecoveryFixInput, "the code exceeds the size limit")
	}
	timeout := l.DefaultTimeout
	if in.TimeoutMS != 0 {
		timeout = time.Duration(in.TimeoutMS) * time.Millisecond
	}
	outputCap := l.DefaultOutputBytes
	if in.MaxOutputBytes != 0 {
		outputCap = in.MaxOutputBytes
	}
	if timeout <= 0 || timeout > l.MaxTimeout || outputCap <= 0 || outputCap > l.MaxOutputBytes {
		return 0, 0, mcp.Fail(mcp.CodeLimitExceeded, mcp.RecoveryFixInput, "limits may only be lowered below the published maxima")
	}
	return timeout, outputCap, nil
}

// namespace reports whether this run sees the same interpreter state as the
// previous run of the session in this process; a new generation means
// variables were lost while files in the workdir survived.
func (r *Runner) namespace(key, id string, k pykernel.Kernel) Namespace {
	r.mu.Lock()
	defer r.mu.Unlock()
	prev, seen := r.generations[key]
	if seen && prev.kernel == k && !k.Info().NamespaceReset {
		return Namespace{ID: id, Generation: prev.id}
	}
	gen := generation{kernel: k, id: uuid.NewString()}
	r.generations[key] = gen
	return Namespace{ID: id, Generation: gen.id, Reset: seen}
}

func classify(ctx context.Context, execErr *pykernel.ExecError, pastDeadline bool) (string, *RunError) {
	switch {
	case execErr == nil:
		return StatusSucceeded, nil
	case ctx.Err() != nil:
		return StatusCancelled, &RunError{Code: ErrorCancelled, Message: "the run was cancelled"}
	case pastDeadline && (execErr.Type == "KernelCancelled" || execErr.Type == "KernelExited"):
		return StatusTimedOut, &RunError{Code: ErrorTimeout, Message: "the run exceeded its time limit"}
	case execErr.Type == "KernelExited":
		return StatusFailed, &RunError{Code: ErrorRuntimeLost, Message: "the Python runtime stopped during the run"}
	case execErr.Type == "MemoryError":
		return StatusFailed, &RunError{Code: ErrorLimitExceeded, Message: execErr.Error()}
	default:
		return StatusFailed, &RunError{Code: ErrorPython, Message: execErr.Error()}
	}
}

func appendCapped(b *strings.Builder, chunk []byte, limit int) bool {
	room := limit - b.Len()
	if room <= 0 {
		return len(chunk) > 0
	}
	if len(chunk) <= room {
		b.Write(chunk)
		return false
	}
	cut := room
	for cut > 0 && cut < len(chunk) && chunk[cut]&0xC0 == 0x80 {
		cut--
	}
	b.Write(chunk[:cut])
	return true
}

type session struct {
	key     string
	tenant  uuid.UUID
	caps    pykernel.CapabilitySet
	workdir string
}

func (s session) Key() string                          { return s.key }
func (s session) TenantID() uuid.UUID                  { return s.tenant }
func (s session) Capabilities() pykernel.CapabilitySet { return s.caps }
func (s session) Mode() pykernel.Mode                  { return pykernel.ModeApply }
func (s session) Workdir() string                      { return s.workdir }
