package backtest

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
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
		RETURNING tenant_id, id, lease, dataset_id, source, created_by, created_origin`, s.leaseInterval()).
		Scan(&j.tenant, &j.id, &j.lease, &j.dataset, &src, &j.user, &j.origin)
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
	batch  []exampleRow
	counts importCounts
	full   bool
	beat   time.Time
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
		im.counts.unreadable++
		return nil
	}
	var payload map[string]any
	if len(ev.Payload) > 0 {
		if err := expr.UnmarshalJSON(ev.Payload, &payload); err != nil {
			im.counts.unreadable++
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
		im.counts.unreadable++
		return nil //nolint:nilerr // an event that does not encode or is too large is counted, the import goes on
	}
	at := ev.OccurredAt
	if at.IsZero() {
		at = im.s.cfg.Now()
	}
	row := exampleRow{id: uuid.New(), source: im.source, ref: ev.Key, title: ev.Title, kind: ev.Kind, event: body, subject: ev.Subject, at: at}
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
	if len(im.batch) == 0 {
		return nil
	}
	err := pgx.BeginFunc(im.ctx, im.s.cfg.DB, func(tx pgx.Tx) error {
		c, err := insertExamples(im.ctx, tx, im.job.tenant, im.job.dataset, im.job.user, im.batch)
		if err != nil {
			return err
		}
		im.counts.imported += c.added
		im.counts.duplicates += c.duplicates
		im.counts.overflow += c.overflow
		if c.overflow > 0 {
			im.full = true
		}
		return nil
	})
	im.batch = im.batch[:0]
	if err != nil {
		return err
	}
	if time.Since(im.beat) > im.s.heartbeatEvery() {
		im.beat = time.Now()
		tag, err := im.s.cfg.DB.Exec(im.ctx, `UPDATE automation.dataset_imports SET heartbeat_at = now(), imported = $3, duplicates = $4, overflow = $5, unreadable = $6
			WHERE tenant_id = $1 AND id = $2 AND lease = $7`, im.job.tenant, im.job.id, im.counts.imported, im.counts.duplicates, im.counts.overflow, im.counts.unreadable, im.job.lease)
		if err != nil {
			return dbErr(err)
		}
		if tag.RowsAffected() == 0 {
			return errCancelled // another worker took the import after this one's lease expired
		}
	}
	return nil
}

func (s *Svc) runImport(ctx context.Context, j importJob) {
	im := &importer{s: s, ctx: ctx, job: j, beat: time.Now()}
	err := s.importInto(ctx, im)
	if err == nil {
		err = im.flush()
	}
	if ctx.Err() != nil || errors.Is(err, errCancelled) {
		// The process stops, or another worker holds the import now: it imports again (duplicates are skipped).
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
		return hi.ImportHistory(ctx, ConnectionRef{Connection: conn, Credentials: creds, As: a.Subject}, src.Window, src.Filter, im.add)
	case ImportModule:
		im.source = SourceHistory
		hs, _ := s.source(src.Source)
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
			SELECT seq, id, event, COALESCE(subject_entity, ''), COALESCE(subject_key, ''), payload, occurred_at
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
			err := r.Scan(&x.seq, &x.id, &x.ev.Kind, &x.ev.Subject.Entity, &x.ev.Subject.Key, &x.payload, &x.ev.OccurredAt)
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

// pending are the examples of the backtest's dataset (as it was when the backtest was queued) it has no
// result for.
func (s *Svc) pending(ctx context.Context, j backtestJob) ([]Example, error) {
	rows, err := s.cfg.DB.Query(ctx, `SELECT `+exampleColumns+` FROM automation.dataset_examples e
		WHERE e.tenant_id = $1 AND e.dataset_id = $2 AND e.created_at <= $3
			AND NOT EXISTS (SELECT 1 FROM automation.backtest_results r WHERE r.tenant_id = e.tenant_id AND r.backtest_id = $4 AND r.example_id = e.id)
		ORDER BY e.occurred_at, e.id`, j.tenant, j.dataset, j.createdAt, j.id)
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
	var total int
	if err := s.cfg.DB.QueryRow(ctx, `SELECT count(*) FROM automation.dataset_examples WHERE tenant_id = $1 AND dataset_id = $2 AND created_at <= $3`,
		j.tenant, j.dataset, j.createdAt).Scan(&total); err != nil {
		return dbErr(err)
	}
	if _, err := s.cfg.DB.Exec(ctx, `UPDATE automation.backtests SET total = $3 WHERE tenant_id = $1 AND id = $2`, j.tenant, j.id, total); err != nil {
		return dbErr(err)
	}
	// realCalls: an agent's cached backtest reads the cache only; a person's calls a decision the cache
	// lacks (and records it). No other execute node is called: effects and calls are captured.
	realCalls := engine.RealCallsDecisions
	if j.decisions == DecisionsCached && a.Origin != engine.OriginHuman {
		realCalls = engine.RealCallsNone
	}
	runCtx, cancel := context.WithCancelCause(ctx)
	defer cancel(nil)
	var live atomic.Int64
	work := make(chan Example)
	var wg sync.WaitGroup
	var failMu sync.Mutex
	var fail error
	for i := 0; i < s.cfg.Parallel; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for ex := range work {
				n, err := s.runExample(runCtx, a, j, v.Graph, nodes, realCalls, ex)
				if err != nil {
					failMu.Lock()
					if fail == nil {
						fail = err
					}
					failMu.Unlock()
					cancel(err)
					continue
				}
				live.Add(int64(n))
			}
		}()
	}
	beat := time.NewTicker(s.heartbeatEvery())
	defer beat.Stop()
	var reported int64
	heartbeat := func() error {
		n := live.Load()
		tag, err := s.cfg.DB.Exec(context.WithoutCancel(ctx), `UPDATE automation.backtests SET heartbeat_at = now(), live_calls = live_calls + $3
			WHERE tenant_id = $1 AND id = $2 AND status = 'running' AND lease = $4`, j.tenant, j.id, n-reported, j.lease)
		if err != nil {
			return dbErr(err)
		}
		reported = n
		if tag.RowsAffected() == 0 {
			return errCancelled
		}
		return nil
	}
feed:
	for _, ex := range todo {
		for {
			select {
			case work <- ex:
				continue feed
			case <-beat.C:
				if err := heartbeat(); err != nil {
					cancel(err)
					break feed
				}
			case <-runCtx.Done():
				break feed
			}
		}
	}
	close(work)
	wg.Wait()
	hbErr := heartbeat()
	if errors.Is(context.Cause(runCtx), errCancelled) || errors.Is(hbErr, errCancelled) {
		return errCancelled
	}
	if fail != nil {
		return fail
	}
	if c := context.Cause(runCtx); c != nil && !errors.Is(c, context.Canceled) {
		return c
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	return hbErr
}

// runExample dry-runs the graph over one example and stores the result; it returns the calls made to
// providers. An error of the example (an event the graph refuses) is its result; an error of the
// backtest (rights lost, the version gone) is returned.
func (s *Svc) runExample(ctx context.Context, a Actor, j backtestJob, g json.RawMessage, nodes map[string]bool, realCalls string, ex Example) (int, error) {
	if j.decisions == DecisionsLive {
		// A live decision is paid: the example runs only while this worker still holds the backtest (a
		// worker whose lease another one took stops before it calls a provider again).
		var holds bool
		if err := s.cfg.DB.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM automation.backtests WHERE tenant_id = $1 AND id = $2 AND status = 'running' AND lease = $3)`,
			j.tenant, j.id, j.lease).Scan(&holds); err != nil {
			return 0, dbErr(err)
		}
		if !holds {
			return 0, errCancelled
		}
	}
	vars := json.RawMessage(`{}`)
	if len(ex.Snapshot) > 0 {
		vars = ex.Snapshot
	}
	subject := ex.Subject
	res, err := s.cfg.Authoring.DryRun(ctx, a, engine.DryRunInput{
		// The automation's memory is read as it is now (a backtest never writes it).
		Graph: g, AutomationID: j.automation, Subject: subject, Events: []json.RawMessage{ex.Event}, Decisions: j.decisions,
		RealCalls: realCalls, Vars: vars, Now: ex.OccurredAt,
	})
	var an analysis
	switch {
	case err == nil:
		an = analyze(res.Trace, nodes)
	case errors.Is(err, engine.ErrInvalidInput), errors.Is(err, engine.ErrLimitExceeded):
		an = analysis{end: EndError, errText: publicError(err), outcomes: map[string]string{}}
	default:
		return 0, err
	}
	enc := func(v any, empty string) []byte {
		b, err := json.Marshal(v)
		if err != nil || string(b) == "null" {
			return []byte(empty)
		}
		return b
	}
	_, err = s.cfg.DB.Exec(context.WithoutCancel(ctx), `
		INSERT INTO automation.backtest_results (tenant_id, backtest_id, example_id, end_kind, outcomes, decisions, effects, trace, error, live_calls)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10) ON CONFLICT DO NOTHING`,
		j.tenant, j.id, ex.ID, an.end, enc(an.outcomes, "{}"), enc(an.decisions, "[]"), enc(an.effects, "[]"), enc(traceRecords(an.trace), "[]"), an.errText, res.LiveCalls)
	if err != nil {
		return 0, dbErr(err)
	}
	return res.LiveCalls, nil
}
