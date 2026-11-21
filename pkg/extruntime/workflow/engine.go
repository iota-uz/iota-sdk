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
//
// A workflow contract may instead have a Go implementation (Config.Natives,
// docs/extension-runtime.md §9.3). Its runner does not replay the journal: it
// keeps the state of its own machine in its own tables, written inside fenced
// NativeRun.Transaction calls together with the receipt, and reads receipts
// lazily (NativeRun.Lookup). Native instances share leases, fencing, timers,
// signals, human tasks, terminate and resolve/retry with package workflows,
// and run without the Node pool.
package workflow

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/iota-uz/iota-sdk/pkg/extruntime/host"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/manifest"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/pool"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/spi"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

const opAttach = serrors.Op("extruntime.workflow.AttachPackages")

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
// (or, for a pinned instance, a specific) package version, or the Go
// implementation registered in Config.Natives (Native set; Package, Version,
// Export and Artifact are empty, and the instance passes no PackageGate,
// ResolveArtifact or artifact hash check).
type Implementation struct {
	Package  string
	Version  string
	Export   string
	Artifact pool.Artifact
	Secrets  []string
	Native   bool
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

// CommandInvoker dispatches the commands NativeRun.Call calls; *host.Host
// implements it, with or without a Node pool.
type CommandInvoker interface {
	Invoke(ctx context.Context, req host.Request) (host.Result, error)
}

// Config wires the engine. Zero values select the defaults.
//
// Catalog, Bindings, Artifacts and Impl are needed only to execute package
// (TS) workflows; they are set together, here or later with AttachPackages.
// Without them the engine runs its Natives alone, and a package instance a run
// reaches is blocked (unavailable). Catalog alone (without the others) is
// allowed: NativeRun.Call reads contract shapes from it.
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
	// Commands dispatches NativeRun.Call (*host.Host). Nil: a Call blocks the
	// instance (unavailable). AttachPackages may set it later.
	Commands CommandInvoker
	// Natives maps workflow contract IDs to their Go implementations. A
	// native contract runs in its own slot pool (NativeContract.Concurrency)
	// and needs neither the Node pool nor the package catalog.
	Natives map[string]NativeContract
	// NativeSource, when set, supplies more native contracts the first time
	// the engine needs them (Ready, Serve, a start, a run): for a composition
	// root that collects the registrations after the engine is constructed.
	// An ID in both Natives and the source is an error.
	NativeSource func() (map[string]NativeContract, error)

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
	if c.DB == nil || c.Subjects == nil {
		return serrors.New(serrors.Invalid, "extruntime workflow: DB and Subjects are required")
	}
	if set := countSet(c.Bindings != nil, c.Artifacts != nil, c.Impl != nil); set != 0 && (set != 3 || c.Catalog == nil) {
		return serrors.New(serrors.Invalid, "extruntime workflow: Catalog, Bindings, Artifacts and Impl are required together to execute package workflows")
	}
	natives, err := normalizeNatives(nil, c.Natives)
	if err != nil {
		return err
	}
	c.Natives = natives
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

// normalizeNatives adds more to into (a new map when into is nil), applying
// the defaults; a duplicate ID or a contract without a runner is an error.
func normalizeNatives(into, more map[string]NativeContract) (map[string]NativeContract, error) {
	if into == nil {
		into = make(map[string]NativeContract, len(more))
	}
	for id, nc := range more {
		if id == "" || nc.Runner == nil {
			return nil, serrors.New(serrors.Invalid, fmt.Sprintf("extruntime workflow: native workflow %q needs an ID and a Runner", id))
		}
		if _, dup := into[id]; dup {
			return nil, serrors.New(serrors.Invalid, fmt.Sprintf("extruntime workflow: native workflow %s is registered twice", id))
		}
		if nc.Concurrency <= 0 {
			nc.Concurrency = 4
		}
		if nc.RunTimeout <= 0 {
			nc.RunTimeout = 5 * time.Minute
		}
		into[id] = nc
	}
	return into, nil
}

func countSet(flags ...bool) int {
	n := 0
	for _, f := range flags {
		if f {
			n++
		}
	}
	return n
}

// Engine is the durable workflow engine. It implements host.WorkflowHandler
// (the wf.* host calls), spi.Starter (start intents) and the management API.
//
// One engine serves a process: claim takes any ready instance, so a second
// engine over the same table would take instances the first one owns. The
// Go-only core builds it; the runtime attaches the package executor to it
// (AttachPackages) instead of building another.
type Engine struct {
	cfg Config

	mu     sync.Mutex
	runner Runner
	pkg    packageDeps
	runs   map[string]*runState // run token -> state
	cancel context.CancelFunc
	done   chan struct{}
	// pkgConcurrency is the number of package workers Serve starts:
	// Config.Concurrency, or Packages.Concurrency once attached.
	pkgConcurrency int

	// natives are Config.Natives with the NativeSource merged in once it
	// loaded (natLoaded); a failed load is retried by the next use.
	natMu     sync.Mutex
	natLoaded bool
	natives   map[string]NativeContract
}

// packageDeps are what package (TS) workflows need.
type packageDeps struct {
	catalog   spi.Catalog
	bindings  host.Bindings
	artifacts host.ArtifactResolver
	impl      Implementations
	commands  CommandInvoker
}

func (p packageDeps) complete() bool {
	return p.catalog != nil && p.bindings != nil && p.artifacts != nil && p.impl != nil
}

// Packages attaches the package executor to an engine built without it.
type Packages struct {
	Catalog   spi.Catalog
	Bindings  host.Bindings
	Artifacts host.ArtifactResolver
	Impl      Implementations
	// Runner executes workflow exports (*host.Host).
	Runner Runner
	// Commands replaces Config.Commands (the runtime's host serves both
	// native and package commands).
	Commands CommandInvoker
	// Concurrency, when positive, replaces Config.Concurrency: the number of
	// package workflow runs the process executes at once.
	Concurrency int
}

// AttachPackages connects the package executor to the engine: from now on it
// executes package workflows too. Call it before Serve (Serve sizes the
// package workers then). A native contract whose ID the catalog declares as a
// package workflow is a composition error.
func (e *Engine) AttachPackages(ctx context.Context, p Packages) error {
	if p.Catalog == nil || p.Bindings == nil || p.Artifacts == nil || p.Impl == nil || p.Runner == nil {
		return serrors.New(serrors.Invalid, "extruntime workflow: AttachPackages needs Catalog, Bindings, Artifacts, Impl and Runner")
	}
	// The registration lock is held until the executor is published, so a
	// concurrent first load of NativeSource either is checked here or sees
	// the executor and checks itself (allNatives).
	e.natMu.Lock()
	defer e.natMu.Unlock()
	natives := e.cfg.Natives
	if e.natLoaded {
		natives = e.natives
	}
	if err := checkPackageConflicts(ctx, p.Catalog, natives); err != nil {
		return err
	}
	e.mu.Lock()
	e.runner = p.Runner
	e.pkg = packageDeps{catalog: p.Catalog, bindings: p.Bindings, artifacts: p.Artifacts, impl: p.Impl, commands: e.pkg.commands}
	if p.Commands != nil {
		e.pkg.commands = p.Commands
	}
	if p.Concurrency > 0 {
		e.pkgConcurrency = p.Concurrency
	}
	e.mu.Unlock()
	return nil
}

// PackageConcurrency is the number of package workflow runs Serve executes
// at once.
func (e *Engine) PackageConcurrency() int {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.pkgConcurrency
}

// SetCommands sets the host NativeRun.Call dispatches commands through.
func (e *Engine) SetCommands(c CommandInvoker) {
	e.mu.Lock()
	e.pkg.commands = c
	e.mu.Unlock()
}

func (e *Engine) deps() packageDeps {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.pkg
}

// catalog is the contract catalog, nil without one.
func (e *Engine) catalog() spi.Catalog { return e.deps().catalog }

func boolPtr(b bool) *bool { return &b }

var (
	_ host.WorkflowHandler = (*Engine)(nil)
	_ spi.Starter          = (*Engine)(nil)
	_ host.Signaler        = (*Engine)(nil)
)

func New(cfg Config) (*Engine, error) {
	if err := cfg.defaults(); err != nil {
		return nil, err
	}
	return &Engine{
		cfg: cfg, runner: cfg.Runner, runs: map[string]*runState{},
		pkg:     packageDeps{catalog: cfg.Catalog, bindings: cfg.Bindings, artifacts: cfg.Artifacts, impl: cfg.Impl, commands: cfg.Commands},
		natives: cfg.Natives, natLoaded: cfg.NativeSource == nil, pkgConcurrency: cfg.Concurrency,
	}, nil
}

// checkPackageConflicts refuses a native contract whose ID an active package
// declares as its own workflow.
func checkPackageConflicts(ctx context.Context, cat spi.Catalog, natives map[string]NativeContract) error {
	for id := range natives {
		res, ok, err := cat.Resolve(ctx, id)
		if err != nil {
			return serrors.Wrap(opAttach, err)
		}
		if ok && !res.Native && res.Resource.Kind == manifest.ResWorkflow {
			return serrors.New(serrors.Invalid, fmt.Sprintf("extruntime workflow: %s has a Go implementation and is also declared as a package workflow", id))
		}
	}
	return nil
}

// Ready loads the NativeSource (once) and checks the native contracts
// against the attached package catalog. Composition roots call it at start,
// so a bad registration fails the start instead of the first run.
func (e *Engine) Ready(ctx context.Context) error {
	_, err := e.allNatives()
	return err
}

// allNatives returns every native contract, loading the source on first use.
// A load is published only after it was validated, including against the
// package catalog when the package executor is attached (AttachPackages
// checks a load that happened before it): no start or run ever selects a
// registration that failed. A failed load is retried by the next use.
func (e *Engine) allNatives() (map[string]NativeContract, error) {
	e.natMu.Lock()
	defer e.natMu.Unlock()
	if e.natLoaded {
		return e.natives, nil
	}
	more, err := e.cfg.NativeSource()
	if err != nil {
		return nil, serrors.Wrap(opAttach, err)
	}
	merged, err := normalizeNatives(maps.Clone(e.cfg.Natives), more)
	if err != nil {
		return nil, err
	}
	if d := e.deps(); d.complete() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := checkPackageConflicts(ctx, d.catalog, merged); err != nil {
			return nil, err
		}
	}
	e.natives, e.natLoaded = merged, true
	return merged, nil
}

// lookupNative returns the Go implementation registered for a workflow
// contract. err is set when the registrations could not be loaded: then the
// caller cannot tell a native contract from a package one and must not fall
// back to either.
func (e *Engine) lookupNative(workflowID string) (NativeContract, bool, error) {
	natives, err := e.allNatives()
	if err != nil {
		return NativeContract{}, false, err
	}
	nc, ok := natives[workflowID]
	return nc, ok, nil
}

// Native returns the Go implementation registered for a workflow contract;
// false also when the registrations cannot be loaded (logged).
func (e *Engine) Native(workflowID string) (NativeContract, bool) {
	nc, ok, err := e.lookupNative(workflowID)
	if err != nil {
		e.cfg.Logger.Error("extruntime: native workflows are not available", "error", err)
	}
	return nc, ok
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

type blockedError struct {
	reason, detail string
}

func (b *blockedError) Error() string { return b.reason + ": " + b.detail }

func marshal(v any) json.RawMessage {
	b, err := json.Marshal(v)
	if err != nil {
		panic(fmt.Sprintf("extruntime workflow: marshal %T: %v", v, err))
	}
	return b
}
