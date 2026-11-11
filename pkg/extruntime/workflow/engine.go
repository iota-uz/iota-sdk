// Package workflow is the durable workflow engine of the extension runtime: a
// Go-owned replay journal over PostgreSQL (docs/extension-runtime.md §8, §9).
//
// A workflow run is a deterministic re-execution of the instance's pinned TS
// function. Every durable primitive is an occurrence (instance, name, n);
// completed occurrences return their recorded result without executing, and
// the invoke carries them so the Node worker answers replays locally.
// sleep/waitFor/task on an unmet condition suspend the run: the worker and any
// transaction are released and the scheduler resumes the instance on wake_at,
// a signal or a task decision. Every state write is fenced by lease_epoch in
// the same transaction as the occurrence write, so a runner that lost its
// lease can never record.
package workflow

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/iota-uz/iota-sdk/pkg/extruntime/host"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/pool"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/spi"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

// NotifyChannel wakes runners after a start, signal or task decision.
const NotifyChannel = "extruntime_workflow"

// Instance states.
const (
	StatusRunning    = "running"
	StatusWaiting    = "waiting"
	StatusCompleted  = "completed"
	StatusFailed     = "failed"
	StatusTerminated = "terminated"
	StatusBlocked    = "blocked"
)

// Block reasons recorded in workflow_instances.blocked_reason.
const (
	BlockNondeterminism = "nondeterminism"
	BlockAmbiguous      = "ambiguous"
	BlockUnavailable    = "unavailable"
	BlockSubject        = "subject"
	BlockDenied         = "denied"
	BlockArtifact       = "artifact"
	BlockFailing        = "failing"
	// BlockPackageDeactivated: the package the instance is pinned to has no
	// active version, so the instance does not execute its code. Resume it after
	// the package is activated again.
	BlockPackageDeactivated = "packageDeactivated"
)

// Occurrence kinds.
const (
	KindTransaction = "transaction"
	KindStep        = "step"
	KindSleep       = "sleep"
	KindWaitFor     = "waitFor"
	KindTask        = "task"
	KindCall        = "call"
	KindStart       = "start"
	KindNow         = "now"
	KindUUID        = "uuid"
)

// Occurrence states.
const (
	OccStarted   = "started"
	OccCompleted = "completed"
	OccFailed    = "failed"
	OccWaiting   = "waiting"
	OccCancelled = "cancelled"
)

// Implementation is the code that runs a workflow: an export of an activated
// (or, for a pinned instance, a specific) package version.
type Implementation struct {
	Package  string
	Version  string
	Export   string
	Artifact pool.Artifact
	Secrets  []string
}

// Implementations resolves workflow implementations. The management slice
// implements it over activations and stored manifests.
type Implementations interface {
	// Active returns the implementation in the currently activated package
	// version that declares the workflow. New instances pin it.
	Active(ctx context.Context, workflowID string) (Implementation, error)
	// At returns the implementation inside a given package version, for
	// StartV2 on an older or newer version.
	At(ctx context.Context, workflowID, pkg, version string) (Implementation, error)
}

// PackageGate is optionally implemented by Implementations. A run of an
// instance whose package it reports inactive does not execute: the instance
// becomes blocked with BlockPackageDeactivated until Resume, so deactivating a
// package stops its live instances from running disabled code.
type PackageGate interface {
	// PackageActive reports whether the package has an active version. Any
	// version counts: an instance pinned to an older version of an active
	// package keeps running (old instances keep their pinned artifacts).
	PackageActive(ctx context.Context, pkg string) (bool, error)
}

// Runner executes a workflow export; *host.Host implements it.
type Runner interface {
	Run(ctx context.Context, req host.RunRequest) (host.Result, error)
}

// Config wires the engine. Zero values select the defaults.
type Config struct {
	DB        *pgxpool.Pool
	Catalog   spi.Catalog
	Bindings  host.Bindings
	Artifacts host.ArtifactResolver
	Impl      Implementations
	Subjects  host.SubjectLoader
	// Runner may be set later with Attach: the host needs the engine as its
	// WorkflowHandler, and the engine needs the host.
	Runner Runner

	// Owner identifies this process in leases. Default: random.
	Owner string
	// LeaseTTL bounds how long a crashed runner holds an instance. Default 30s.
	LeaseTTL time.Duration
	// Heartbeat renews the lease while a run is active. Default LeaseTTL/3.
	Heartbeat time.Duration
	// PollInterval is the scheduler's fallback poll period. Default 1s.
	PollInterval time.Duration
	// Concurrency is the number of runs this process executes at once. Default 4.
	Concurrency int
	// RunTimeout bounds a single run (one replay up to its next suspension).
	// Default 5m.
	RunTimeout time.Duration
	// MaxRunFailures blocks an instance whose runs keep failing for
	// infrastructure reasons (worker lost, pool closed). Default 5.
	MaxRunFailures int
	// RetryBackoff is the base delay before such a run is retried. Default 1s.
	RetryBackoff time.Duration
	// TaskAdminPermission lets holders decide tasks they are not assigned to.
	TaskAdminPermission string
	Logger              *slog.Logger
}

func (c *Config) defaults() error {
	if c.DB == nil || c.Catalog == nil || c.Bindings == nil || c.Artifacts == nil || c.Impl == nil || c.Subjects == nil {
		return serrors.New(serrors.Invalid, "extruntime workflow: DB, Catalog, Bindings, Artifacts, Impl and Subjects are required")
	}
	if c.Owner == "" {
		c.Owner = "engine-" + uuid.NewString()
	}
	if c.LeaseTTL <= 0 {
		c.LeaseTTL = 30 * time.Second
	}
	if c.Heartbeat <= 0 {
		c.Heartbeat = c.LeaseTTL / 3
	}
	if c.PollInterval <= 0 {
		c.PollInterval = time.Second
	}
	if c.Concurrency <= 0 {
		c.Concurrency = 4
	}
	if c.RunTimeout <= 0 {
		c.RunTimeout = 5 * time.Minute
	}
	if c.MaxRunFailures <= 0 {
		c.MaxRunFailures = 5
	}
	if c.RetryBackoff <= 0 {
		c.RetryBackoff = time.Second
	}
	if c.Logger == nil {
		c.Logger = slog.Default()
	}
	return nil
}

// Engine is the durable workflow engine. It implements host.WorkflowHandler
// (the wf.* host calls), spi.Starter (start intents) and the management API.
type Engine struct {
	cfg Config

	mu     sync.Mutex
	runner Runner
	runs   map[string]*runState // run token -> state
	cancel context.CancelFunc
	done   chan struct{}
}

var (
	_ host.WorkflowHandler = (*Engine)(nil)
	_ spi.Starter          = (*Engine)(nil)
	_ host.Signaler        = (*Engine)(nil)
)

func New(cfg Config) (*Engine, error) {
	if err := cfg.defaults(); err != nil {
		return nil, err
	}
	return &Engine{cfg: cfg, runner: cfg.Runner, runs: map[string]*runState{}}, nil
}

// Attach sets the runner once the host exists (the host is constructed with
// the engine as its WorkflowHandler).
func (e *Engine) Attach(r Runner) {
	e.mu.Lock()
	e.runner = r
	e.mu.Unlock()
}

func (e *Engine) getRunner() (Runner, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.runner == nil {
		return nil, serrors.New(serrors.Invalid, "extruntime workflow: no runner attached")
	}
	return e.runner, nil
}

// Errors of the management API.
var (
	ErrNotFound     = errors.New("extruntime workflow: not found")
	ErrNotAccepting = errors.New("extruntime workflow: instance does not accept this action in its current state")
	ErrTaskClosed   = errors.New("extruntime workflow: task is no longer open")
	ErrForbidden    = errors.New("extruntime workflow: actor may not perform this action")
	ErrInvalid      = errors.New("extruntime workflow: invalid request")
)

// Fencing errors surface to the worker as wire errors that end the run.
const (
	codeLeaseLost  = "workflow.leaseLost"
	codeTerminated = "workflow.terminated"
	codeBlocked    = "workflow.blocked"
	codeAborted    = "workflow.runAborted"
)

var (
	errLeaseLost  = &pool.WireError{Kind: pool.ErrKindInternal, Code: codeLeaseLost, Message: "the run lost its lease; another runner owns the instance"}
	errTerminated = &pool.WireError{Kind: pool.ErrKindInternal, Code: codeTerminated, Message: "the instance is no longer running; host calls are rejected"}
	// errRunAborted ends a run whose host call hit an infrastructure failure:
	// the workflow never sees the failure as its own, the run is replayed later.
	errRunAborted = &pool.WireError{Kind: pool.ErrKindInternal, Code: codeAborted, Message: "the run was aborted by an infrastructure failure and will be retried"}
)

type blockInfo struct {
	reason, detail string
}

func (b *blockInfo) Error() string { return b.reason + ": " + b.detail }

func marshal(v any) json.RawMessage {
	b, err := json.Marshal(v)
	if err != nil {
		panic(fmt.Sprintf("extruntime workflow: marshal %T: %v", v, err))
	}
	return b
}
