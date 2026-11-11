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
	"github.com/iota-uz/iota-sdk/pkg/extruntime/manifest"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/pool"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/spi"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

const opRun = serrors.Op("extruntime.workflow.run")

// RunOnce claims and runs ready instances one after another until none is
// ready, and returns how many runs it executed. It is the deterministic entry
// point used by tests and by single-shot schedulers.
func (e *Engine) RunOnce(ctx context.Context) (int, error) {
	n := 0
	for {
		inst, err := e.claim(ctx)
		if err != nil {
			return n, err
		}
		if inst == nil {
			return n, nil
		}
		e.runClaimed(ctx, inst)
		n++
		if ctx.Err() != nil {
			return n, ctx.Err()
		}
	}
}

// Serve runs the scheduler in the background: Concurrency workers claim ready
// instances, woken by NOTIFY (start, signal, decision) and by a poll that
// catches timers and expired leases.
func (e *Engine) Serve(ctx context.Context) {
	e.mu.Lock()
	if e.cancel != nil {
		e.mu.Unlock()
		return
	}
	ctx, e.cancel = context.WithCancel(ctx)
	e.done = make(chan struct{})
	done := e.done
	e.mu.Unlock()

	wake := listen(ctx, e.cfg.DB, NotifyChannel, e.cfg.PollInterval, e.cfg.Logger)
	var wg sync.WaitGroup
	signals := make([]chan struct{}, e.cfg.Concurrency)
	for i := range signals {
		signals[i] = make(chan struct{}, 1)
		wg.Add(1)
		go func(sig chan struct{}) {
			defer wg.Done()
			e.worker(ctx, sig)
		}(signals[i])
	}
	go func() {
		defer close(done)
		for {
			select {
			case <-ctx.Done():
				wg.Wait()
				return
			case <-wake:
				for _, s := range signals {
					select {
					case s <- struct{}{}:
					default:
					}
				}
			}
		}
	}()
}

func (e *Engine) worker(ctx context.Context, wake <-chan struct{}) {
	for ctx.Err() == nil {
		inst, err := e.claim(ctx)
		if err != nil {
			if ctx.Err() == nil {
				e.cfg.Logger.Error("extruntime: workflow claim failed", "error", err)
			}
		} else if inst != nil {
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
	e.mu.Lock()
	var victims []*runState
	for _, rs := range e.runs {
		if rs.inst.ID.String() == instance {
			victims = append(victims, rs)
		}
	}
	e.mu.Unlock()
	for _, rs := range victims {
		rs.markStale(errTerminated)
		rs.cancel()
	}
}

// heartbeat renews the lease while the run is active. A fenced renewal that
// matches no row means the lease was taken over or the instance terminated:
// the run is aborted and records nothing more.
func (e *Engine) heartbeat(ctx context.Context, rs *runState) (stop func()) {
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
				tag, err := e.cfg.DB.Exec(hb, `
					UPDATE extruntime.workflow_instances SET lease_until = now() + $4::interval
					WHERE id = $1 AND tenant_id = $2 AND lease_epoch = $3 AND status = 'running'`,
					rs.inst.ID, rs.inst.TenantID, rs.epoch, e.cfg.LeaseTTL.String())
				if err == nil && tag.RowsAffected() == 0 {
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
	runCtx, cancel := context.WithTimeout(ctx, e.cfg.RunTimeout)
	defer cancel()
	rs := &runState{
		inst: *inst, epoch: inst.LeaseEpoch, cancel: cancel,
		token:   fmt.Sprintf("%s:%d", inst.ID, inst.LeaseEpoch),
		journal: map[occKey]Occurrence{},
	}
	e.register(rs)
	defer e.unregister(rs)
	stopHB := e.heartbeat(runCtx, rs)
	defer stopHB()

	if err := e.run(runCtx, ctx, rs); err != nil && ctx.Err() == nil {
		e.cfg.Logger.Error("extruntime: workflow run bookkeeping failed", "instance", inst.ID, "error", err)
	}
}

func (e *Engine) run(runCtx, parent context.Context, rs *runState) error {
	inst := rs.inst
	if gate, ok := e.cfg.Impl.(PackageGate); ok {
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

	art, secrets, err := e.cfg.Artifacts.ResolveArtifact(runCtx, inst.Package, inst.Version)
	if err != nil {
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

	if we := rs.stale(); we != nil {
		e.cfg.Logger.Info("extruntime: workflow run abandoned", "instance", inst.ID, "reason", we.Code)
		return nil
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
	res, ok, err := e.cfg.Catalog.Resolve(ctx, inst.Workflow)
	if err != nil || !ok || res.Resource.Output == nil {
		return nil
	}
	if verr := manifest.ValidateValue(*res.Resource.Output, out); verr != nil {
		return &pool.WireError{Kind: pool.ErrKindInternal, Code: "runtime.invalidOutput", Message: verr.Error()}
	}
	return nil
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

func listenOnce(ctx context.Context, db *pgxpool.Pool, channel string, poll time.Duration, poke func()) error {
	conn, err := db.Acquire(ctx)
	if err != nil {
		return err
	}
	defer conn.Release()
	if _, err := conn.Exec(ctx, "LISTEN "+pgx.Identifier{channel}.Sanitize()); err != nil {
		return err
	}
	poke()
	for {
		wctx, cancel := context.WithTimeout(ctx, 10*poll)
		_, err := conn.Conn().WaitForNotification(wctx)
		cancel()
		switch {
		case err == nil:
			poke()
		case ctx.Err() != nil:
			return nil
		case errors.Is(err, context.DeadlineExceeded):
			if perr := conn.Conn().Ping(ctx); perr != nil {
				return perr
			}
		default:
			return err
		}
	}
}
