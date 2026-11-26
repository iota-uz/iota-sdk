package backtest

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/iota-uz/iota-sdk/pkg/automation/catalog"
	"github.com/iota-uz/iota-sdk/pkg/automation/engine"
	"github.com/iota-uz/iota-sdk/pkg/automation/expr"
	"github.com/iota-uz/iota-sdk/pkg/automation/graph"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/spi"
)

// Serve runs queued imports and backtests until ctx is done. Several processes may serve: a job is
// claimed with a lease its worker renews, and a job whose worker died is taken again after LeaseTTL
// (a backtest resumes after the examples it has results for).
func (s *Svc) Serve(ctx context.Context) {
	t := time.NewTicker(s.cfg.PollInterval)
	defer t.Stop()
	for {
		for ctx.Err() == nil {
			worked, err := s.RunOnce(ctx)
			if err != nil && ctx.Err() == nil {
				s.cfg.Logger.Warn("automation backtest: a job failed to run", "error", err)
			}
			if !worked || err != nil {
				break
			}
		}
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}

// RunOnce runs one queued job (an import first, then a backtest); worked is false when none was due.
func (s *Svc) RunOnce(ctx context.Context) (bool, error) {
	if job, ok, err := s.claimImport(ctx); err != nil || ok {
		if ok {
			s.runImport(ctx, job)
		}
		return ok, err
	}
	job, ok, err := s.claimBacktest(ctx)
	if err != nil || !ok {
		return false, err
	}
	s.runBacktest(ctx, job)
	return true, nil
}

func (s *Svc) leaseInterval() string { return fmt.Sprintf("%f seconds", s.cfg.LeaseTTL.Seconds()) }

// heartbeatEvery is how often a job renews its lease.
func (s *Svc) heartbeatEvery() time.Duration { return max(s.cfg.LeaseTTL/4, 50*time.Millisecond) }

// actorOf is the person who queued a job, with the rights they hold now.
func (s *Svc) actorOf(ctx context.Context, tenant uuid.UUID, user int64, origin string) (Actor, error) {
	sub, err := s.cfg.Users.LoadSubject(ctx, tenant, user, spi.SubjectInitiator)
	if err != nil {
		return Actor{}, fmt.Errorf("%w: the person who queued it is not available", engine.ErrForbidden)
	}
	sub.TenantID, sub.UserID = tenant, user
	a := Actor{TenantID: tenant, UserID: user, Origin: engine.Origin(origin), Subject: sub}
	return a, s.authorize(a)
}

// publicError is what a failed job tells its author: the engine's message without its package prefix.
func publicError(err error) string {
	var ve *engine.ValidationError
	switch {
	case errors.As(err, &ve):
		return "the version does not validate against the current catalog"
	case errors.Is(err, engine.ErrForbidden), errors.Is(err, engine.ErrNotFound), errors.Is(err, engine.ErrInvalidInput),
		errors.Is(err, engine.ErrLimitExceeded), errors.Is(err, engine.ErrHumanRequired), errors.Is(err, ErrUnsupported):
		return strings.TrimPrefix(err.Error(), "automation: ")
	}
	return "the job could not be completed"
}

type importJob struct {
	tenant  uuid.UUID
	id      uuid.UUID
	lease   uuid.UUID
	dataset uuid.UUID
	source  ImportSource
	user    int64
	origin  string
	// counts are what the import committed before this claim (a worker that resumes it goes on from them).
	counts importCounts
}

func (s *Svc) claimImport(ctx context.Context) (importJob, bool, error) {
	var j importJob
	var src []byte
	err := s.cfg.DB.QueryRow(ctx, `
		UPDATE automation.dataset_imports SET status = 'running', started_at = COALESCE(started_at, now()), heartbeat_at = now(), lease = gen_random_uuid()
		WHERE (tenant_id, id) = (
			SELECT tenant_id, id FROM automation.dataset_imports
			WHERE status = 'queued' OR (status = 'running' AND heartbeat_at < now() - $1::interval)
			ORDER BY created_at LIMIT 1 FOR UPDATE SKIP LOCKED)
		RETURNING tenant_id, id, lease, dataset_id, source, created_by, created_origin, imported, duplicates, overflow, unreadable`, s.leaseInterval()).
		Scan(&j.tenant, &j.id, &j.lease, &j.dataset, &src, &j.user, &j.origin, &j.counts.imported, &j.counts.duplicates, &j.counts.overflow, &j.counts.unreadable)
	if errors.Is(err, pgx.ErrNoRows) {
		return j, false, nil
	}
	if err != nil {
		return j, false, dbErr(err)
	}
	if err := json.Unmarshal(src, &j.source); err != nil {
		s.finishImport(ctx, j, importCounts{}, errors.New("the source of the import is not readable"))
		return j, false, nil //nolint:nilerr // the job is failed and recorded; the worker goes on
	}
	return j, true, nil
}

type importCounts struct {
	imported, duplicates, overflow, unreadable int
}

func (s *Svc) finishImport(ctx context.Context, j importJob, c importCounts, err error) {
	status, msg := StatusDone, ""
	if err != nil {
		status, msg = StatusFailed, publicError(err)
	}
	if _, e := s.cfg.DB.Exec(context.WithoutCancel(ctx), `
		UPDATE automation.dataset_imports SET status = $3, imported = $4, duplicates = $5, overflow = $6, unreadable = $7, error = $8,
			finished_at = now(), heartbeat_at = now()
		WHERE tenant_id = $1 AND id = $2 AND lease = $9`, j.tenant, j.id, status, c.imported, c.duplicates, c.overflow, c.unreadable, msg, j.lease); e != nil {
		s.cfg.Logger.Warn("automation backtest: an import's end was not recorded", "error", e)
	}
}

// importer collects the events of an import into the dataset in batches.
type importer struct {
	s      *Svc
	ctx    context.Context
	job    importJob
	actor  Actor
	view   catalog.View
	source string
	// origin is the source within its kind: an event's key is unique within it.
	origin string
	batch  []exampleRow
	// counts are committed; unreadable counts what add left out since the last batch.
	counts     importCounts
	unreadable int
	// seen are the keys this pass emitted: a key again is a duplicate (one an earlier pass of the same
	// import added is skipped without counting: that pass counted it).
	seen       map[string]bool
	duplicates int
	full       bool
}

const importBatch = 100

// add projects an event for the actor and queues it; flush writes the queue.
func (im *importer) add(ev Event) error {
	if err := im.ctx.Err(); err != nil {
		return err
	}
	spec, ok := im.view.Catalog.Events[ev.Kind]
	if !ok || ev.Kind == "" {
		// The actor may not read the event (or the catalog does not know it).
		im.unreadable++
		return nil
	}
	var payload map[string]any
	if len(ev.Payload) > 0 {
		if err := expr.UnmarshalJSON(ev.Payload, &payload); err != nil {
			im.unreadable++
			return nil //nolint:nilerr // an unreadable event is counted, the import goes on
		}
	}
	if payload == nil {
		payload = map[string]any{}
	}
	payload = engine.KeepDeclared(payload, spec.Payload)
	payload["kind"] = ev.Kind
	if ev.BySelf {
		payload[catalog.BySelfField] = true
	}
	body, err := json.Marshal(payload)
	if err != nil || len(body) > MaxEventBytes {
		im.unreadable++
		return nil //nolint:nilerr // an event that does not encode or is too large is counted, the import goes on
	}
	at := ev.OccurredAt
	if at.IsZero() {
		at = im.s.cfg.Now()
	}
	if ev.Key != "" {
		if im.seen[ev.Key] {
			im.duplicates++
			return nil
		}
		im.seen[ev.Key] = true
	}
	row := exampleRow{id: uuid.New(), source: im.source, origin: im.origin, importID: im.job.id, ref: ev.Key, title: ev.Title, kind: ev.Kind, event: body, subject: ev.Subject, at: at}
	if !im.full {
		if len(ev.Snapshot) > 0 && string(ev.Snapshot) != "null" {
			row.snapshot = ev.Snapshot
		} else if row.snapshot, row.approximate, err = im.s.snapshot(im.ctx, im.actor.Subject, im.actor.TenantID, ev.Subject, at); err != nil {
			return err
		}
		if len(row.snapshot) > MaxEventBytes {
			row.snapshot, row.approximate = nil, true
		}
	}
	im.batch = append(im.batch, row)
	if len(im.batch) >= importBatch {
		return im.flush()
	}
	return nil
}

func (im *importer) flush() error {
	if len(im.batch) == 0 && im.unreadable == 0 && im.duplicates == 0 {
		return nil
	}
	// The batch and the import's counts commit together, and only while this worker holds the import: a
	// worker whose lease another one took leaves the dataset unchanged.
	var c insertCounts
	err := pgx.BeginFunc(im.ctx, im.s.cfg.DB, func(tx pgx.Tx) error {
		var holds bool
		if err := tx.QueryRow(im.ctx, `SELECT EXISTS (SELECT 1 FROM automation.dataset_imports
			WHERE tenant_id = $1 AND id = $2 AND lease = $3 AND status = 'running' FOR UPDATE)`, im.job.tenant, im.job.id, im.job.lease).Scan(&holds); err != nil {
			return dbErr(err)
		}
		if !holds {
			return errCancelled
		}
		var err error
		if len(im.batch) > 0 {
			if c, err = insertExamples(im.ctx, tx, im.job.tenant, im.job.dataset, im.job.user, im.batch); err != nil {
				return err
			}
		}
		_, err = tx.Exec(im.ctx, `UPDATE automation.dataset_imports SET heartbeat_at = now(),
			imported = imported + $4, duplicates = duplicates + $5, overflow = overflow + $6, unreadable = unreadable + $7
			WHERE tenant_id = $1 AND id = $2 AND lease = $3`, im.job.tenant, im.job.id, im.job.lease, c.added, c.duplicates+im.duplicates, c.overflow, im.unreadable)
		return dbErr(err)
	})
	if err != nil {
		return err
	}
	// Counted once the batch committed.
	im.counts.imported += c.added
	im.counts.duplicates += c.duplicates + im.duplicates
	im.counts.overflow += c.overflow
	im.counts.unreadable += im.unreadable
	im.unreadable, im.duplicates = 0, 0
	im.batch = im.batch[:0]
	if c.overflow > 0 {
		im.full = true
	}
	return nil
}

// renew keeps a job's lease while it works (an IMAP search, a sparse module source, a long example)
// and cancels the job with errCancelled when another worker took it.
func (s *Svc) renew(ctx context.Context, cancel context.CancelCauseFunc, table string, tenant, id, lease uuid.UUID) {
	t := time.NewTicker(s.heartbeatEvery())
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
		tag, err := s.cfg.DB.Exec(ctx, `UPDATE automation.`+table+` SET heartbeat_at = now()
			WHERE tenant_id = $1 AND id = $2 AND lease = $3 AND status = 'running'`, tenant, id, lease)
		if err != nil {
			if ctx.Err() == nil {
				s.cfg.Logger.Warn("automation backtest: a lease was not renewed", "error", err)
			}
			continue
		}
		if tag.RowsAffected() == 0 {
			cancel(errCancelled)
			return
		}
	}
}

func (s *Svc) runImport(ctx context.Context, j importJob) {
	runCtx, cancel := context.WithCancelCause(ctx)
	defer cancel(nil)
	go s.renew(runCtx, cancel, "dataset_imports", j.tenant, j.id, j.lease)
	im := &importer{s: s, ctx: runCtx, job: j, counts: j.counts, seen: map[string]bool{}}
	err := s.importInto(runCtx, im)
	if err == nil {
		err = im.flush()
	}
	if ctx.Err() != nil || errors.Is(err, errCancelled) || errors.Is(context.Cause(runCtx), errCancelled) {
		// The process stops, or another worker holds the import now: it goes on from the committed counts.
		return
	}
	s.finishImport(ctx, j, im.counts, err)
}

func (s *Svc) importInto(ctx context.Context, im *importer) error {
	j := im.job
	a, err := s.actorOf(ctx, j.tenant, j.user, j.origin)
	if err != nil {
		return err
	}
	im.actor = a
	var automationID uuid.UUID
	if err := s.cfg.DB.QueryRow(ctx, `SELECT automation_id FROM automation.datasets WHERE tenant_id = $1 AND id = $2`, j.tenant, j.dataset).Scan(&automationID); err != nil {
		return dbErr(err)
	}
	if _, err := s.automation(ctx, a, automationID); err != nil {
		return err
	}
	if im.view, err = s.cfg.Catalog.ForTenant(ctx, j.tenant, catalog.AsAuthor(a.Subject)); err != nil {
		return err
	}
	if err := s.checkSource(ctx, a, &j.source); err != nil {
		return err
	}
	src := j.source
	switch src.Kind {
	case ImportJournal:
		im.source = SourceJournal
		kinds := src.Events
		if len(kinds) == 0 {
			if kinds, err = s.triggerEvents(ctx, a, automationID); err != nil {
				return err
			}
		}
		return s.importJournal(ctx, im, kinds, src.Window)
	case ImportConnection:
		im.source = SourceHistory
		conn, hi, err := s.openConnection(ctx, a, src.ConnectionID)
		if err != nil {
			return err
		}
		_, creds, err := s.cfg.Connections.Get(ctx, a.TenantID, conn.ID)
		if err != nil {
			return err
		}
		im.origin = "connection:" + conn.ID.String()
		return hi.ImportHistory(ctx, ConnectionRef{Connection: conn, Credentials: creds, As: a.Subject}, src.Window, src.Filter, im.add)
	case ImportModule:
		im.source = SourceHistory
		hs, _ := s.source(src.Source)
		im.origin = "module:" + hs.Name
		return hs.Importer.ImportHistory(ctx, ConnectionRef{As: a.Subject}, src.Window, src.Filter, im.add)
	}
	return invalid("unknown import kind")
}

// triggerEvents are the events the triggers of the automation's latest version start on.
func (s *Svc) triggerEvents(ctx context.Context, a Actor, automationID uuid.UUID) ([]string, error) {
	v, err := s.versionOf(ctx, a, automationID, 0)
	if err != nil {
		return nil, err
	}
	var g struct {
		Triggers []graph.Trigger `json:"triggers"`
	}
	if err := json.Unmarshal(v.Graph, &g); err != nil {
		return nil, invalid("the graph is not readable")
	}
	seen := map[string]bool{}
	var out []string
	for _, t := range g.Triggers {
		if t.Event != "" && !seen[t.Event] {
			seen[t.Event] = true
			out = append(out, t.Event)
		}
	}
	return out, nil
}

// importJournal reads the outbox of the tenant over the window, in the order events were published.
func (s *Svc) importJournal(ctx context.Context, im *importer, kinds []string, w TimeWindow) error {
	if len(kinds) == 0 {
		return nil
	}
	var after int64
	for {
		rows, err := s.cfg.DB.Query(ctx, `
			SELECT seq, id, event, COALESCE(subject_entity, ''), COALESCE(subject_key, ''), payload, occurred_at,
				COALESCE((cause->'inbound'->>'by_self')::boolean, false)
			FROM extruntime.outbox_events
			WHERE tenant_id = $1 AND event = ANY($2) AND occurred_at >= $3 AND occurred_at < $4 AND seq > $5
			ORDER BY seq LIMIT 500`, im.job.tenant, kinds, w.From, w.To, after)
		if err != nil {
			return dbErr(err)
		}
		type outboxRow struct {
			seq     int64
			id      uuid.UUID
			ev      Event
			payload []byte
		}
		page, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (outboxRow, error) {
			var x outboxRow
			// by_self is the cause's, as the live dispatch reads it: a message the connector sent itself.
			err := r.Scan(&x.seq, &x.id, &x.ev.Kind, &x.ev.Subject.Entity, &x.ev.Subject.Key, &x.payload, &x.ev.OccurredAt, &x.ev.BySelf)
			return x, err
		})
		if err != nil {
			return dbErr(err)
		}
		for _, x := range page {
			x.ev.Key, x.ev.Payload = x.id.String(), x.payload
			if x.ev.Subject.Entity == "" {
				x.ev.Subject.Entity = graph.SubjectNone
			}
			if err := im.add(x.ev); err != nil {
				return err
			}
			after = x.seq
		}
		if len(page) < 500 {
			return nil
		}
	}
}

type backtestJob struct {
	tenant     uuid.UUID
	id         uuid.UUID
	lease      uuid.UUID
	automation uuid.UUID
	dataset    uuid.UUID
	version    int
	decisions  string
	user       int64
	origin     string
	createdAt  time.Time
}

func (s *Svc) claimBacktest(ctx context.Context) (backtestJob, bool, error) {
	var j backtestJob
	err := s.cfg.DB.QueryRow(ctx, `
		UPDATE automation.backtests SET status = 'running', started_at = COALESCE(started_at, now()), heartbeat_at = now(), lease = gen_random_uuid()
		WHERE (tenant_id, id) = (
			SELECT tenant_id, id FROM automation.backtests
			WHERE status = 'queued' OR (status = 'running' AND heartbeat_at < now() - $1::interval)
			ORDER BY created_at LIMIT 1 FOR UPDATE SKIP LOCKED)
		RETURNING tenant_id, id, lease, automation_id, dataset_id, version, decisions, created_by, created_origin, created_at`, s.leaseInterval()).
		Scan(&j.tenant, &j.id, &j.lease, &j.automation, &j.dataset, &j.version, &j.decisions, &j.user, &j.origin, &j.createdAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return j, false, nil
	}
	return j, err == nil, dbErr(err)
}

func (s *Svc) finishBacktest(ctx context.Context, j backtestJob, err error) {
	status, msg := StatusDone, ""
	if err != nil {
		status, msg = StatusFailed, publicError(err)
	}
	if _, e := s.cfg.DB.Exec(context.WithoutCancel(ctx), `
		UPDATE automation.backtests SET status = $3, error = $4, finished_at = now(), heartbeat_at = now()
		WHERE tenant_id = $1 AND id = $2 AND status = 'running' AND lease = $5`, j.tenant, j.id, status, msg, j.lease); e != nil {
		s.cfg.Logger.Warn("automation backtest: a backtest's end was not recorded", "error", e)
	}
}

// errCancelled stops a backtest whose row is no longer running (cancelled, or taken by another worker
// after its lease expired).
var errCancelled = errors.New("backtest: no longer running")

func (s *Svc) runBacktest(ctx context.Context, j backtestJob) {
	err := s.execute(ctx, j)
	if errors.Is(err, errCancelled) || ctx.Err() != nil {
		return
	}
	s.finishBacktest(ctx, j, err)
}

// pending are the examples the backtest was queued with (its example_ids) that it has no result for.
func (s *Svc) pending(ctx context.Context, j backtestJob) ([]Example, error) {
	rows, err := s.cfg.DB.Query(ctx, `SELECT `+exampleColumns+` FROM automation.dataset_examples e
		JOIN automation.backtests b ON b.tenant_id = e.tenant_id AND b.id = $2 AND e.id = ANY(b.example_ids)
		WHERE e.tenant_id = $1
			AND NOT EXISTS (SELECT 1 FROM automation.backtest_results r WHERE r.tenant_id = e.tenant_id AND r.backtest_id = $2 AND r.example_id = e.id)
		ORDER BY e.occurred_at, e.id`, j.tenant, j.id)
	if err != nil {
		return nil, dbErr(err)
	}
	out, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (Example, error) { return scanExample(r) })
	return out, dbErr(err)
}

func (s *Svc) execute(ctx context.Context, j backtestJob) error {
	a, err := s.actorOf(ctx, j.tenant, j.user, j.origin)
	if err != nil {
		return err
	}
	if j.decisions == DecisionsLive && a.Origin != engine.OriginHuman {
		return fmt.Errorf("%w: only a person runs a backtest with live decisions", engine.ErrHumanRequired)
	}
	v, err := s.cfg.Automations.GetVersion(ctx, a, j.automation, j.version)
	if err != nil {
		return err
	}
	// The graph is checked once against the actor's catalog: a version that no longer validates fails
	// the backtest rather than every example.
	if _, err := s.cfg.Authoring.Validate(ctx, a, v.Graph); err != nil {
		return err
	}
	nodes := s.decisionNodes(v.Graph)
	todo, err := s.pending(ctx, j)
	if err != nil {
		return err
	}
	// The total is the examples queued that still exist (one deleted with no result is gone).
	if _, err := s.cfg.DB.Exec(ctx, `UPDATE automation.backtests b SET total = (SELECT count(*) FROM automation.dataset_examples e
		WHERE e.tenant_id = b.tenant_id AND e.id = ANY(b.example_ids)) WHERE b.tenant_id = $1 AND b.id = $2 AND b.lease = $3`, j.tenant, j.id, j.lease); err != nil {
		return dbErr(err)
	}
	// realCalls: an agent's cached backtest reads the cache only; a person's calls a decision the cache
	// lacks (and records it). No other execute node is called: effects and calls are captured.
	realCalls := engine.RealCallsDecisions
	if j.decisions == DecisionsCached && a.Origin != engine.OriginHuman {
		realCalls = engine.RealCallsNone
	}
	// The calls of one example are bounded as Estimate counts them.
	maxCalls := 0
	if realCalls != engine.RealCallsNone && len(nodes) > 0 {
		maxCalls = callsPerExample(v.Graph, len(nodes))
	}
	runCtx, cancel := context.WithCancelCause(ctx)
	defer cancel(nil)
	// The lease is renewed until the last example in flight ends; a lost lease cancels the rest.
	renewed := make(chan struct{})
	go func() {
		defer close(renewed)
		s.renew(runCtx, cancel, "backtests", j.tenant, j.id, j.lease)
	}()
	// A live run pays every decision: one example at a time, so a worker whose lease another took has at
	// most one example in flight.
	parallel := s.cfg.Parallel
	if j.decisions == DecisionsLive {
		parallel = 1
	}
	work := make(chan Example)
	var wg sync.WaitGroup
	var failMu sync.Mutex
	var fail error
	for i := 0; i < parallel; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for ex := range work {
				if err := s.runExample(runCtx, a, j, v.Graph, nodes, realCalls, maxCalls, ex); err != nil {
					failMu.Lock()
					if fail == nil {
						fail = err
					}
					failMu.Unlock()
					cancel(err)
				}
			}
		}()
	}
feed:
	for _, ex := range todo {
		select {
		case work <- ex:
		case <-runCtx.Done():
			break feed
		}
	}
	close(work)
	wg.Wait()
	cancel(nil)
	<-renewed
	if errors.Is(context.Cause(runCtx), errCancelled) || errors.Is(fail, errCancelled) {
		return errCancelled
	}
	if fail != nil {
		return fail
	}
	return ctx.Err()
}

// runExample dry-runs the graph over one example and stores the result. An error of the example (an
// event the graph refuses, the calls past its bound) is its result; an error of the backtest (rights
// lost, the version gone, the lease lost) is returned.
func (s *Svc) runExample(ctx context.Context, a Actor, j backtestJob, g json.RawMessage, nodes map[string]bool, realCalls string, maxCalls int, ex Example) error {
	if realCalls != engine.RealCallsNone {
		// A decision the cache lacks is paid: the example runs only while this worker still holds the
		// backtest (one whose lease another worker took stops before it calls a provider again).
		if err := s.holds(ctx, s.cfg.DB, j, false); err != nil {
			return err
		}
	}
	vars := json.RawMessage(`{}`)
	if len(ex.Snapshot) > 0 {
		vars = ex.Snapshot
	}
	if selfOnly(g, ex.Event) {
		// As the live dispatch: a message the connector sent itself starts no trigger without include_self.
		return s.storeResult(ctx, j, ex, analysis{end: EndNotTriggered, outcomes: map[string]string{}, trace: []engine.TraceStep{
			{Kind: engine.TraceNotTriggered, Message: "the connector sent the message itself and no trigger includes its own messages"}}}, 0)
	}
	res, err := s.cfg.Authoring.DryRun(ctx, a, engine.DryRunInput{
		// The automation's memory is read as it is now (a backtest never writes it).
		Graph: g, AutomationID: j.automation, Subject: ex.Subject, Events: []json.RawMessage{ex.Event}, Decisions: j.decisions,
		RealCalls: realCalls, Vars: vars, Now: ex.OccurredAt, MaxCalls: maxCalls,
	})
	var an analysis
	switch {
	case err == nil:
		an = analyze(res.Trace, nodes)
	case errors.Is(err, engine.ErrInvalidInput), errors.Is(err, engine.ErrLimitExceeded):
		an = analysis{end: EndError, errText: publicError(err), outcomes: map[string]string{}}
	default:
		return err
	}
	return s.storeResult(ctx, j, ex, an, res.LiveCalls)
}

// storeResult writes an example's result, only while this worker holds the running backtest: a
// cancelled one or one another worker took keeps no result of this worker.
func (s *Svc) storeResult(ctx context.Context, j backtestJob, ex Example, an analysis, liveCalls int) error {
	enc := func(v any, empty string) []byte {
		b, err := json.Marshal(v)
		if err != nil || string(b) == "null" {
			return []byte(empty)
		}
		return b
	}
	return pgx.BeginFunc(context.WithoutCancel(ctx), s.cfg.DB, func(tx pgx.Tx) error {
		if err := s.holds(ctx, tx, j, true); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `
			INSERT INTO automation.backtest_results (tenant_id, backtest_id, example_id, end_kind, triggered, outcomes, decisions, effects, trace, error, live_calls)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11) ON CONFLICT DO NOTHING`,
			j.tenant, j.id, ex.ID, an.end, an.triggered, enc(an.outcomes, "{}"), enc(an.decisions, "[]"), enc(an.effects, "[]"), enc(traceRecords(an.trace), "[]"), an.errText, liveCalls)
		return dbErr(err)
	})
}

// holds is errCancelled unless the backtest runs under this worker's lease; lock keeps the row (a
// cancel or a new claim waits for the caller's transaction).
// selfOnly is whether an event the connector sent itself (by_self) meets only triggers that exclude
// such messages: the router starts none of them.
func selfOnly(g json.RawMessage, event json.RawMessage) bool {
	var ev struct {
		Kind   string `json:"kind"`
		BySelf bool   `json:"by_self"`
	}
	if json.Unmarshal(event, &ev) != nil || !ev.BySelf {
		return false
	}
	var gr struct {
		Triggers []graph.Trigger `json:"triggers"`
	}
	if json.Unmarshal(g, &gr) != nil {
		return false
	}
	for _, t := range gr.Triggers {
		if t.Event == ev.Kind && t.IncludeSelf {
			return false
		}
	}
	return true
}

func (s *Svc) holds(ctx context.Context, q querier, j backtestJob, lock bool) error {
	sql := `SELECT EXISTS (SELECT 1 FROM automation.backtests WHERE tenant_id = $1 AND id = $2 AND status = 'running' AND lease = $3)`
	if lock {
		sql = `SELECT EXISTS (SELECT 1 FROM automation.backtests WHERE tenant_id = $1 AND id = $2 AND status = 'running' AND lease = $3 FOR SHARE)`
	}
	var ok bool
	if err := q.QueryRow(context.WithoutCancel(ctx), sql, j.tenant, j.id, j.lease).Scan(&ok); err != nil {
		return dbErr(err)
	}
	if !ok {
		return errCancelled
	}
	return nil
}
