package workflow

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/iota-uz/iota-sdk/pkg/extruntime/host"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/listenconn"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/manifest"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/pool"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/spi"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

// RunOnce claims and runs ready instances one after another until none is
// ready, and returns how many runs it executed. It is the deterministic entry
// point used by tests and by single-shot schedulers.
func (e *Engine) RunOnce(ctx context.Context) (int, error) {
	n := 0
	for {
		inst, claimed, err := e.claim(ctx, claimFilter{})
		if err != nil {
			return n, err
		}
		if !claimed {
			return n, nil
		}
		e.runClaimed(ctx, inst)
		n++
		if ctx.Err() != nil {
			return n, ctx.Err()
		}
	}
}

// wakeHub fans the scheduler's wake-ups out to every worker.
type wakeHub struct {
	mu   sync.Mutex
	subs []chan struct{}
}

func (h *wakeHub) subscribe() chan struct{} {
	c := make(chan struct{}, 1)
	h.mu.Lock()
	h.subs = append(h.subs, c)
	h.mu.Unlock()
	return c
}

func (h *wakeHub) poke() {
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, s := range h.subs {
		select {
		case s <- struct{}{}:
		default:
		}
	}
}

// Serve runs the scheduler in the background, woken by NOTIFY (start, signal,
// decision) and by a poll that catches timers and expired leases. Each native
// contract has its own pool of NativeContract.Concurrency workers, so native
// runs never take the slots of package workflows; package workflows get
// Config.Concurrency workers (or Packages.Concurrency). Attach the package
// executor before Serve: without it the package workers block every ready
// package instance as unavailable.
func (e *Engine) Serve(ctx context.Context) {
	e.mu.Lock()
	if e.cancel != nil {
		e.mu.Unlock()
		return
	}
	ctx, e.cancel = context.WithCancel(ctx)
	e.done = make(chan struct{})
	done, hub, wg, pkgSlots := e.done, &wakeHub{}, &sync.WaitGroup{}, e.pkgConcurrency
	e.mu.Unlock()

	// Package workers run with or without the package executor: without it
	// they block the ready package instances as unavailable (run), so an
	// instance never waits silently in a process that cannot execute it.
	// Workers are tracked by this call's hub and wait group, never by those
	// of a later Serve.
	startWorkers(ctx, hub, wg, pkgSlots, func(ctx context.Context, sig <-chan struct{}) {
		e.worker(ctx, claimFilter{native: boolPtr(false)}, sig)
	})
	startNatives := func() bool {
		natives, err := e.allNatives()
		if err != nil {
			e.cfg.Logger.Error("extruntime: native workflows are not served yet; their registrations are loaded again on the next poll", "error", err)
			return false
		}
		for id, nc := range natives {
			filter := claimFilter{native: boolPtr(true), workflow: id}
			startWorkers(ctx, hub, wg, nc.Concurrency, func(ctx context.Context, sig <-chan struct{}) { e.worker(ctx, filter, sig) })
		}
		return true
	}
	nativesUp := startNatives()
	wake := listen(ctx, e.cfg.DB, NotifyChannel, e.cfg.PollInterval, e.cfg.Logger)
	go func() {
		defer close(done)
		for {
			select {
			case <-ctx.Done():
				wg.Wait()
				return
			case <-wake:
				if !nativesUp {
					// A failed load of the registrations is retried here; the
					// pools start once it succeeds.
					nativesUp = startNatives()
				}
				hub.poke()
			}
		}
	}()
}

// startWorkers starts n workers on hub and wg.
func startWorkers(ctx context.Context, hub *wakeHub, wg *sync.WaitGroup, n int, work func(ctx context.Context, wake <-chan struct{})) {
	for range n {
		sig := hub.subscribe()
		wg.Add(1)
		go func() {
			defer wg.Done()
			work(ctx, sig)
		}()
	}
}

func (e *Engine) worker(ctx context.Context, filter claimFilter, wake <-chan struct{}) {
	for ctx.Err() == nil {
		inst, claimed, err := e.claim(ctx, filter)
		if err != nil {
			if ctx.Err() == nil {
				e.cfg.Logger.Error("extruntime: workflow claim failed", "error", err)
			}
		} else if claimed {
			e.runClaimed(ctx, inst)
			continue
		}
		select {
		case <-ctx.Done():
			return
		case <-wake:
		}
	}
}

// Shutdown stops Serve and waits for the in-flight runs to be abandoned; their
// leases expire and the instances replay on the next claim.
func (e *Engine) Shutdown() {
	e.mu.Lock()
	cancel, done := e.cancel, e.done
	e.cancel, e.done = nil, nil
	e.mu.Unlock()
	if cancel == nil {
		return
	}
	cancel()
	<-done
}

func (e *Engine) register(rs *runState) {
	e.mu.Lock()
	e.runs[rs.token] = rs
	e.mu.Unlock()
}

func (e *Engine) unregister(rs *runState) {
	e.mu.Lock()
	delete(e.runs, rs.token)
	e.mu.Unlock()
}

func (e *Engine) runByToken(token string) *runState {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.runs[token]
}

// cancelLocal aborts the in-flight run of the instance in this process.
func (e *Engine) cancelLocal(instance string) {
	for _, rs := range e.localRuns(instance) {
		rs.markStale(errTerminated)
		rs.cancel()
	}
}

// staleLocal lets the in-flight run of the instance finish its current step
// and rejects its further host calls.
func (e *Engine) staleLocal(instance string) {
	for _, rs := range e.localRuns(instance) {
		rs.markStale(errTerminated)
	}
}

func (e *Engine) localRuns(instance string) []*runState {
	e.mu.Lock()
	defer e.mu.Unlock()
	var out []*runState
	for _, rs := range e.runs {
		if rs.inst.ID.String() == instance {
			out = append(out, rs)
		}
	}
	return out
}

// heartbeat renews the lease while the run is active. A renewal matches only
// a lease that is still valid on the database clock, so an expired lease is
// never resurrected. No matched row means the lease expired or was taken
// over (the run is aborted and records nothing more) or the instance was
// terminated: then its in-flight step may finish, and every further host call
// and write is fenced.
// renew extends a live lease. The lease is checked after the row lock is
// granted (lockLease), so a lease that expired while the renewal waited for
// the lock stays expired.
func (e *Engine) renew(ctx context.Context, rs *runState) (leaseState, error) {
	var lease leaseState
	err := e.inTx(ctx, func(tx pgx.Tx) error {
		var err error
		if lease, err = lockLease(ctx, tx, rs); err != nil || !lease.live {
			return err
		}
		_, err = tx.Exec(ctx, `UPDATE extruntime.workflow_instances SET lease_until = clock_timestamp() + $3::interval WHERE id = $1 AND tenant_id = $2`,
			rs.inst.ID, rs.inst.TenantID, pgInterval(e.cfg.LeaseTTL))
		return serrors.FromDB(opFence, err)
	})
	return lease, err
}

func (e *Engine) heartbeat(ctx context.Context, rs *runState) func() {
	hb, cancel := context.WithCancel(ctx)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		t := time.NewTicker(e.cfg.Heartbeat)
		defer t.Stop()
		for {
			select {
			case <-hb.Done():
				return
			case <-t.C:
				lease, err := e.renew(hb, rs)
				if err == nil && !lease.live {
					if lease.status == StatusTerminated {
						rs.markStale(errTerminated)
						return
					}
					rs.markStale(errLeaseLost)
					rs.cancel()
					return
				}
			}
		}
	}()
	return func() { cancel(); wg.Wait() }
}

// runClaimed replays one claimed instance up to its next suspension and
// records how the run ended. All writes are fenced by the claim's epoch.
func (e *Engine) runClaimed(ctx context.Context, inst *Instance) {
	timeout := e.cfg.RunTimeout
	var (
		native    *NativeContract
		nativeErr error
	)
	if inst.Native {
		// Package claims never wait on the native registrations. A native
		// run resolves its contract once: the timeout and the dispatch below
		// use the same lookup.
		nc, ok, err := e.lookupNative(inst.Workflow)
		switch {
		case err != nil:
			nativeErr = err
		case ok:
			native, timeout = &nc, nc.RunTimeout
		}
	}
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	rs := &runState{
		inst: *inst, epoch: inst.LeaseEpoch, cancel: cancel,
		token:   fmt.Sprintf("%s:%d", inst.ID, inst.LeaseEpoch),
		journal: map[occKey]Occurrence{},
		lazy:    inst.Native,
		native:  native, nativeErr: nativeErr,
	}
	e.register(rs)
	defer e.unregister(rs)
	stopHB := e.heartbeat(runCtx, rs)
	defer stopHB()

	run := e.run
	if inst.Native {
		run = e.runNative
	}
	if err := run(runCtx, ctx, rs); err != nil && ctx.Err() == nil {
		e.cfg.Logger.Error("extruntime: workflow run bookkeeping failed", "instance", inst.ID, "error", err)
	}
}

func (e *Engine) run(runCtx, parent context.Context, rs *runState) error {
	inst := rs.inst
	deps := e.deps()
	if !deps.complete() {
		return e.finishBlocked(parent, rs, BlockUnavailable, fmt.Sprintf("workflow %s is a package workflow and this process has no package runtime (EXTRUNTIME_ENABLED is off); resume the instance where the runtime runs", inst.Workflow))
	}
	if gate, ok := deps.impl.(PackageGate); ok {
		active, gerr := gate.PackageActive(runCtx, inst.Package)
		switch {
		case gerr != nil:
			return e.finishTransient(parent, rs, gerr)
		case !active:
			return e.finishBlocked(parent, rs, BlockPackageDeactivated, fmt.Sprintf("package %s has no active version; resume the instance after it is activated again", inst.Package))
		}
	}
	subject, err := e.cfg.Subjects.LoadSubject(runCtx, inst.TenantID, inst.SubjectUserID, inst.SubjectKind)
	switch {
	case errors.Is(err, host.ErrSubjectBlocked):
		return e.finishBlocked(parent, rs, BlockSubject, err.Error())
	case err != nil:
		return e.finishTransient(parent, rs, err)
	}
	subject.TenantID, subject.UserID, subject.Kind = inst.TenantID, inst.SubjectUserID, inst.SubjectKind
	rs.subject = subject

	art, secrets, err := deps.artifacts.ResolveArtifact(runCtx, inst.Package, inst.Version)
	if err != nil {
		if parent.Err() != nil {
			// The engine is stopping: released for the next process, no failure.
			return e.abandon(parent, rs)
		}
		return e.finishBlocked(parent, rs, BlockArtifact, fmt.Sprintf("pinned artifact %s@%s is unavailable: %v", inst.Package, inst.Version, err))
	}
	if inst.ArtifactHash != "" && art.Hash != inst.ArtifactHash {
		return e.finishBlocked(parent, rs, BlockArtifact, fmt.Sprintf("pinned artifact %s@%s changed: instance was started on %s, found %s", inst.Package, inst.Version, inst.ArtifactHash, art.Hash))
	}
	journal, err := e.loadJournal(runCtx, inst)
	if err != nil {
		return e.finishTransient(parent, rs, err)
	}
	rs.journal = journal

	runner, err := e.getRunner()
	if err != nil {
		return e.finishTransient(parent, rs, err)
	}
	res, runErr := runner.Run(runCtx, host.RunRequest{
		Export: host.Export{
			Kind:     pool.KindWorkflow,
			Resource: inst.Workflow,
			Binding: host.Binding{
				Contract: inst.Workflow, Package: inst.Package, Version: inst.Version, Export: inst.Export,
				Artifact: art, Secrets: secrets,
			},
		},
		Input:      inst.Input,
		Subject:    subject,
		Trigger:    host.Trigger{Kind: host.TriggerWorkflow, Ref: inst.ID.String()},
		Producer:   spi.Producer{InstanceID: inst.ID.String()},
		Occurrence: rs.journalPayload(),
		Token:      rs.token,
	})

	if code, abandoned := rs.staleCode(); abandoned {
		e.cfg.Logger.Info("extruntime: workflow run abandoned", "instance", inst.ID, "reason", code)
		return nil
	}
	if parent.Err() != nil {
		// The engine is stopping: released for the next process, no failure.
		return e.abandon(parent, rs)
	}
	if b := rs.blockedInfo(); b != nil {
		return e.finishBlocked(parent, rs, b.reason, b.detail)
	}
	if ab := rs.aborted(); ab != nil {
		// A host call hit the infrastructure: whatever the workflow made of it,
		// the run is replayed after a backoff, never failed or completed.
		return e.finishTransient(parent, rs, ab)
	}
	switch {
	case runErr == nil:
		if err := e.checkOutput(runCtx, inst, res.Output); err != nil {
			return e.finishFailed(parent, rs, err)
		}
		return e.finishCompleted(parent, rs, res.Output)
	case pool.IsSuspend(runErr):
		return e.finishSuspended(parent, rs)
	case parent.Err() != nil:
		return e.finishTransient(parent, rs, parent.Err())
	case host.IsInfrastructure(runErr):
		return e.finishTransient(parent, rs, runErr)
	}
	var we *pool.WireError
	if errors.As(runErr, &we) {
		switch {
		case errors.Is(we, pool.ErrArtifactMissing), errors.Is(we, pool.ErrExportMissing):
			return e.finishBlocked(parent, rs, BlockArtifact, we.Error())
		case we.Kind == pool.ErrKindDenied:
			return e.finishBlocked(parent, rs, BlockDenied, we.Error())
		case we.Code == codeLeaseLost || we.Code == codeTerminated:
			return nil
		case we.Code == pool.CodeCanceled:
			return e.finishTransient(parent, rs, runErr)
		}
		return e.finishFailed(parent, rs, we)
	}
	// Run timeout and anything else untyped: nothing about the business logic
	// is known, so the instance is replayed later.
	return e.finishTransient(parent, rs, runErr)
}

// checkOutput validates the workflow result against its declared output.
func (e *Engine) checkOutput(ctx context.Context, inst Instance, out []byte) *pool.WireError {
	declared := e.declaredOutput(ctx, inst.Workflow)
	if declared == nil {
		return nil
	}
	if verr := manifest.ValidateValue(*declared, out); verr != nil {
		return &pool.WireError{Kind: pool.ErrKindInternal, Code: "runtime.invalidOutput", Message: verr.Error()}
	}
	return nil
}

// declaredOutput is the output type a workflow declares, nil when it declares
// none or the catalog cannot say: an unknown contract is not a reason to fail
// a run that completed.
func (e *Engine) declaredOutput(ctx context.Context, workflow string) *manifest.Type {
	cat := e.catalog()
	if cat == nil {
		return nil
	}
	res, ok, err := cat.Resolve(ctx, workflow)
	if err != nil || !ok {
		return nil
	}
	return res.Resource.Output
}

// listen returns a channel that receives on every NOTIFY of channel and at
// least every poll interval, reconnecting on connection loss.
func listen(ctx context.Context, db *pgxpool.Pool, channel string, poll time.Duration, log interface {
	Warn(msg string, args ...any)
}) <-chan struct{} {
	out := make(chan struct{}, 1)
	poke := func() {
		select {
		case out <- struct{}{}:
		default:
		}
	}
	go func() {
		for ctx.Err() == nil {
			if err := listenOnce(ctx, db, channel, poll, poke); err != nil && ctx.Err() == nil {
				log.Warn("extruntime: LISTEN connection lost; polling until it is restored", "channel", channel, "error", err)
				select {
				case <-ctx.Done():
				case <-time.After(poll):
				}
			}
		}
	}()
	go func() {
		t := time.NewTicker(poll)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				poke()
			}
		}
	}()
	return out
}

// listenOnce holds LISTEN on a dedicated connection opened like the pool's own
// (listenconn: its settings and connect hooks), so the listener never takes a pool slot for the engine's lifetime.
func listenOnce(ctx context.Context, db *pgxpool.Pool, channel string, poll time.Duration, poke func()) error {
	conn, err := listenconn.Connect(ctx, db)
	if err != nil {
		return err
	}
	defer func() { _ = conn.Close(context.WithoutCancel(ctx)) }()
	if _, err := conn.Exec(ctx, "LISTEN "+pgx.Identifier{channel}.Sanitize()); err != nil {
		return err
	}
	poke()
	for {
		wctx, cancel := context.WithTimeout(ctx, 10*poll)
		_, err := conn.WaitForNotification(wctx)
		cancel()
		switch {
		case err == nil:
			poke()
		case ctx.Err() != nil:
			return nil
		case errors.Is(err, context.DeadlineExceeded):
			if perr := conn.Ping(ctx); perr != nil {
				return perr
			}
		default:
			return err
		}
	}
}
