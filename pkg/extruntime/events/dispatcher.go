package events

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/host"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/listenconn"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/pool"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/spi"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

const (
	opRunOnce = serrors.Op("extruntime.events.Dispatcher.RunOnce")
	opDeliver = serrors.Op("extruntime.events.Dispatcher.deliver")
	opPrune   = serrors.Op("extruntime.events.Dispatcher.Prune")
)

// Receipt states of extruntime.subscription_receipts.
const (
	StatusDone    = "done"
	StatusFailed  = "failed"
	StatusDead    = "dead"
	StatusBlocked = "blocked"
)

// Config configures the dispatcher. Zero values select the defaults.
type Config struct {
	DB *pgxpool.Pool
	// Consumers lists the consumers of every pass: Go consumers of modules
	// and applications and, with the extension runtime, the package
	// subscriptions (PackageConsumers).
	Consumers ConsumerSource
	// Subjects loads current rights; consumers resolve the subject they act
	// as through it (ResolveSubject).
	Subjects host.SubjectLoader

	// PollInterval is the fallback poll period when no NOTIFY arrives. Default 2s.
	PollInterval time.Duration
	// BatchSize bounds events fetched per consumer and pass. Default 50.
	BatchSize int
	// Concurrency bounds the consumers delivering at once. Default 8. It is
	// capped by the pool (see deliverySlots): a delivery holds a connection
	// for its transaction and may take a second one (a package handler
	// records its invocation on the pool). LISTEN uses a dedicated
	// connection outside the pool.
	Concurrency int
	// MaxAttempts before a poisoned event is dead-lettered. Default 5.
	MaxAttempts int
	// BackoffBase doubles per attempt up to BackoffMax. Defaults 1s and 5m.
	BackoffBase, BackoffMax time.Duration
	// BlockedRecheck is how often a blocked delivery (no owner, subject
	// refused) is re-evaluated. Default 30s.
	BlockedRecheck time.Duration
	// Retention is how long events stay in the outbox once no delivery of
	// them is pending. Default 7d.
	Retention     time.Duration
	PruneInterval time.Duration // default 1h
	Logger        *slog.Logger
}

func (c *Config) defaults() error {
	if c.DB == nil || c.Consumers == nil || c.Subjects == nil {
		return serrors.New(serrors.Invalid, "extruntime events: DB, Consumers and Subjects are required")
	}
	if c.PollInterval <= 0 {
		c.PollInterval = 2 * time.Second
	}
	if c.BatchSize <= 0 {
		c.BatchSize = 50
	}
	if c.Concurrency <= 0 {
		c.Concurrency = 8
	}
	if c.MaxAttempts <= 0 {
		c.MaxAttempts = 5
	}
	if c.BackoffBase <= 0 {
		c.BackoffBase = time.Second
	}
	if c.BackoffMax < c.BackoffBase {
		c.BackoffMax = 5 * time.Minute
	}
	if c.BlockedRecheck <= 0 {
		c.BlockedRecheck = 30 * time.Second
	}
	if c.Retention <= 0 {
		c.Retention = DefaultRetention
	}
	if c.PruneInterval <= 0 {
		c.PruneInterval = time.Hour
	}
	if c.Logger == nil {
		c.Logger = slog.Default()
	}
	return nil
}

// DefaultRetention is how long events stay in the outbox by default.
const DefaultRetention = 7 * 24 * time.Hour

// running counts the dispatchers whose loop runs in this process.
var running atomic.Int64

// Running returns how many dispatchers serve in this process (Start without
// Stop). An application runs one.
func Running() int { return int(running.Load()) }

// Dispatcher delivers outbox events to consumers after commit.
//
// Every committed event without a done/dead receipt of the consumer is
// eligible; there is no read cursor, so a transaction that took a lower seq but
// commits later is never skipped and no running transaction anywhere in the
// cluster delays delivery. Order follows seq among visible rows (not a strict
// commit order). The consumer watermark only excludes older events. Each
// delivery is its own transaction with the consumer's receipt, so a duplicate
// delivery is a no-op, and a poisoned event is retried with backoff and
// dead-lettered without blocking later ones or other consumers.
type Dispatcher struct {
	cfg Config

	mu      sync.Mutex
	cancel  context.CancelFunc
	done    chan struct{}
	lastPrn time.Time
}

// deliverySlots is how many consumers deliver at once: Concurrency, capped so
// that two connections per delivery fit in the pool, with at least one
// delivery. The LISTEN connections (the dispatcher's and the catalog's) are
// dedicated connections outside the pool.
func deliverySlots(concurrency int, maxConns int32) int {
	limit := max(int(maxConns)/2, 1)
	return min(concurrency, limit)
}

func NewDispatcher(cfg Config) (*Dispatcher, error) {
	if err := cfg.defaults(); err != nil {
		return nil, err
	}
	cfg.Concurrency = deliverySlots(cfg.Concurrency, cfg.DB.Config().MaxConns)
	return &Dispatcher{cfg: cfg}, nil
}

type outboxRow struct {
	Envelope
	status   string
	attempts int
}

const fetchSQL = `
SELECT e.id::text, e.seq, e.tenant_id, e.event, e.subject_entity, e.subject_key, e.payload, e.initiator_user_id, e.occurred_at,
       COALESCE(r.status, ''), COALESCE(r.attempts, 0)
FROM extruntime.outbox_events e
LEFT JOIN extruntime.subscription_receipts r ON r.subscription = $1 AND r.event_id = e.id
WHERE e.event = ANY($2::text[])
  AND e.seq > $3
  AND (r.event_id IS NULL OR (r.status IN ('pending', 'failed', 'blocked') AND r.next_at <= now()))
ORDER BY e.seq
LIMIT $4`

// RunOnce delivers everything currently deliverable to every consumer and
// returns the number of successful deliveries. Consumers are independent:
// they run concurrently and one failing never holds back another. A consumer
// ID listed twice is delivered to once and reported as an error.
func (d *Dispatcher) RunOnce(ctx context.Context) (int, error) {
	consumers, srcErr := d.cfg.Consumers.Consumers(ctx)
	var errs []error
	if srcErr != nil {
		// The consumers that were read are still served: a failing source
		// (package subscriptions, a module's Go source) does not hold back
		// the others.
		errs = append(errs, srcErr)
	}
	// IDs are validated before any worker starts, so errs has one writer here.
	seen := make(map[string]struct{}, len(consumers))
	run := make([]Consumer, 0, len(consumers))
	for _, c := range consumers {
		if c == nil {
			continue
		}
		id := c.ID()
		if id == "" {
			errs = append(errs, errors.New("extruntime events: a consumer has an empty ID"))
			continue
		}
		if _, dup := seen[id]; dup {
			errs = append(errs, fmt.Errorf("extruntime events: consumer ID %q is registered twice", id))
			continue
		}
		seen[id] = struct{}{}
		run = append(run, c)
	}
	var (
		wg    sync.WaitGroup
		mu    sync.Mutex
		total int
		slots = make(chan struct{}, d.cfg.Concurrency)
	)
	for _, c := range run {
		wg.Add(1)
		go func() {
			defer wg.Done()
			slots <- struct{}{}
			defer func() { <-slots }()
			n, err := d.runConsumer(ctx, c)
			mu.Lock()
			defer mu.Unlock()
			total += n
			if err != nil {
				errs = append(errs, fmt.Errorf("consumer %s: %w", c.ID(), err))
			}
		}()
	}
	wg.Wait()
	if len(errs) > 0 {
		return total, serrors.Wrap(opRunOnce, errors.Join(errs...))
	}
	return total, nil
}

func (d *Dispatcher) runConsumer(ctx context.Context, c Consumer) (int, error) {
	keys, err := c.Events(ctx)
	if err != nil || len(keys) == 0 {
		return 0, err
	}
	after, err := c.AfterSeq(ctx)
	if err != nil {
		return 0, err
	}
	delivered := 0
	for {
		if ctx.Err() != nil {
			return delivered, ctx.Err()
		}
		rows, err := d.fetch(ctx, c.ID(), keys, after)
		if err != nil {
			return delivered, err
		}
		if len(rows) == 0 {
			return delivered, nil
		}
		for _, row := range rows {
			ok, err := d.deliver(ctx, c, row)
			if err != nil {
				return delivered, err
			}
			if ok {
				delivered++
			}
		}
		if len(rows) < d.cfg.BatchSize {
			return delivered, nil
		}
	}
}

func (d *Dispatcher) fetch(ctx context.Context, consumer string, keys []string, after int64) ([]outboxRow, error) {
	rows, err := d.cfg.DB.Query(ctx, fetchSQL, consumer, keys, after, d.cfg.BatchSize)
	if err != nil {
		return nil, serrors.FromDB(opRunOnce, err)
	}
	defer rows.Close()
	var out []outboxRow
	for rows.Next() {
		var (
			r                 outboxRow
			subEntity, subKey *string
			payload           []byte
		)
		if err := rows.Scan(&r.ID, &r.Seq, &r.TenantID, &r.Event, &subEntity, &subKey, &payload, &r.InitiatorUserID, &r.OccurredAt, &r.status, &r.attempts); err != nil {
			return nil, serrors.FromDB(opRunOnce, err)
		}
		if subEntity != nil && subKey != nil {
			r.Subject = &spi.ObjectRef{Entity: *subEntity, Key: *subKey}
		}
		r.Payload = json.RawMessage(payload)
		out = append(out, r)
	}
	return out, serrors.FromDB(opRunOnce, rows.Err())
}

// deliver hands one event to the consumer in a transaction with its receipt.
// It returns true when Deliver succeeded and the transaction committed.
func (d *Dispatcher) deliver(ctx context.Context, c Consumer, ev outboxRow) (bool, error) {
	tx, err := d.cfg.DB.Begin(ctx)
	if err != nil {
		return false, serrors.FromDB(opDeliver, err)
	}
	defer func() { _ = tx.Rollback(context.WithoutCancel(ctx)) }()

	var attempts int
	err = tx.QueryRow(ctx, `
		INSERT INTO extruntime.subscription_receipts AS r (subscription, event_id, tenant_id, status, attempts)
		VALUES ($1, $2::uuid, $3, 'done', 1)
		ON CONFLICT (subscription, event_id) DO UPDATE
		SET status = 'done', attempts = r.attempts + 1, last_error = '', updated_at = now()
		WHERE r.status IN ('pending', 'failed', 'blocked')
		RETURNING attempts`, c.ID(), ev.ID, ev.TenantID).Scan(&attempts)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil // already delivered or dead: duplicate delivery is a no-op
	}
	if err != nil {
		return false, serrors.FromDB(opDeliver, err)
	}
	// The consumer works in the event's tenant only.
	if _, err := tx.Exec(ctx, `SELECT set_config('app.tenant_id', $1, true)`, ev.TenantID.String()); err != nil {
		return false, serrors.FromDB(opDeliver, err)
	}

	commits := &afterCommits{}
	// The context carries the delivery transaction and the event's tenant, so
	// repositories that read them from the context (composables.UseTx,
	// UseTenantID) work inside the delivery and need no second connection.
	dctx := context.WithValue(composables.WithTenantID(composables.WithTx(ctx, tx), ev.TenantID), afterCommitKey{}, commits)
	runErr := c.Deliver(dctx, tx, ev.Envelope)
	if runErr == nil {
		runErr = tx.Commit(ctx)
		if runErr == nil {
			for _, fn := range commits.fns {
				fn(ctx)
			}
			return true, nil
		}
		// A failed commit (a deferred constraint, a transaction the consumer
		// left aborted) is a failed attempt like any other, so the event backs
		// off and dead-letters instead of blocking the later ones.
	}
	if ctx.Err() != nil {
		return false, ctx.Err()
	}
	_ = tx.Rollback(context.WithoutCancel(ctx))
	if errors.Is(runErr, ErrNoSubject) {
		return false, d.recordBlocked(ctx, c.ID(), ev, runErr.Error())
	}
	return false, d.recordFailure(ctx, c.ID(), ev, attempts, runErr)
}

func (d *Dispatcher) backoff(attempts int) time.Duration {
	delay := d.cfg.BackoffBase
	for i := 1; i < attempts && delay < d.cfg.BackoffMax; i++ {
		delay *= 2
	}
	return min(delay, d.cfg.BackoffMax)
}

// recordFailure keeps the failed attempt visible. It runs outside the rolled
// back delivery transaction and never overwrites a delivered receipt. A
// failure of the platform (database, worker, pool) is not the handler's: it is
// retried after the backoff without counting as an attempt, so an outage can
// never dead-letter events.
func (d *Dispatcher) recordFailure(ctx context.Context, consumer string, ev outboxRow, attempts int, runErr error) error {
	status := StatusFailed
	if host.IsInfrastructure(runErr) {
		attempts = max(attempts-1, 0)
	} else if attempts >= d.cfg.MaxAttempts {
		status = StatusDead
	}
	var inv any
	var ie *invocationError
	if errors.As(runErr, &ie) {
		inv = ie.invocationID
	}
	msg := runErr.Error()
	if we, ok := pool.AsWireError(runErr); ok {
		msg = we.Error()
	}
	_, err := d.cfg.DB.Exec(context.WithoutCancel(ctx), `
		INSERT INTO extruntime.subscription_receipts AS r (subscription, event_id, tenant_id, status, attempts, next_at, last_error, invocation_id)
		VALUES ($1, $2::uuid, $3, $4, $5, now() + $6::interval, $7, $8::uuid)
		ON CONFLICT (subscription, event_id) DO UPDATE
		SET status = EXCLUDED.status, attempts = EXCLUDED.attempts, next_at = EXCLUDED.next_at,
		    last_error = EXCLUDED.last_error, invocation_id = COALESCE(EXCLUDED.invocation_id, r.invocation_id), updated_at = now()
		WHERE r.status <> 'done'`,
		consumer, ev.ID, ev.TenantID, status, attempts, d.backoff(max(attempts, 1)).String(), msg, inv)
	if err != nil {
		return serrors.FromDB(opDeliver, err)
	}
	d.cfg.Logger.Warn("extruntime: event delivery failed", "consumer", consumer, "event", ev.ID, "attempt", attempts, "status", status, "error", msg)
	return nil
}

func (d *Dispatcher) recordBlocked(ctx context.Context, consumer string, ev outboxRow, diagnostic string) error {
	_, err := d.cfg.DB.Exec(ctx, `
		INSERT INTO extruntime.subscription_receipts AS r (subscription, event_id, tenant_id, status, attempts, next_at, last_error)
		VALUES ($1, $2::uuid, $3, 'blocked', 0, now() + $4::interval, $5)
		ON CONFLICT (subscription, event_id) DO UPDATE
		SET status = 'blocked', next_at = EXCLUDED.next_at, last_error = EXCLUDED.last_error, updated_at = now()
		WHERE r.status IN ('pending', 'failed', 'blocked')`,
		consumer, ev.ID, ev.TenantID, d.cfg.BlockedRecheck.String(), diagnostic)
	return serrors.FromDB(opDeliver, err)
}

// Prune deletes the events that occurred longer than the retention ago and
// have no pending, failed or blocked delivery of any consumer. An event no
// consumer has a receipt for is deleted too: a consumer that has not reached
// it within the retention (stopped background work, a key added later) never
// receives it.
func (d *Dispatcher) Prune(ctx context.Context) (int64, error) {
	tag, err := d.cfg.DB.Exec(ctx, `
		DELETE FROM extruntime.outbox_events e
		WHERE e.occurred_at < now() - $1::interval
		  AND NOT EXISTS (
			SELECT 1 FROM extruntime.subscription_receipts r
			WHERE r.event_id = e.id AND r.status IN ('pending', 'failed', 'blocked'))`, d.cfg.Retention.String())
	if err != nil {
		return 0, serrors.FromDB(opPrune, err)
	}
	return tag.RowsAffected(), nil
}

// Start runs the dispatcher until Stop or ctx ends: a LISTEN connection wakes
// it after a commit that published events, with a poll fallback.
func (d *Dispatcher) Start(ctx context.Context) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.cancel != nil {
		return
	}
	ctx, d.cancel = context.WithCancel(ctx)
	d.done = make(chan struct{})
	running.Add(1)
	go d.loop(ctx, d.done)
}

// Stop ends Start's loop and waits for the in-flight pass.
func (d *Dispatcher) Stop() {
	d.mu.Lock()
	cancel, done := d.cancel, d.done
	d.cancel, d.done = nil, nil
	d.mu.Unlock()
	if cancel == nil {
		return
	}
	cancel()
	<-done
}

func (d *Dispatcher) loop(ctx context.Context, done chan struct{}) {
	defer close(done)
	defer running.Add(-1)
	wake := listen(ctx, d.cfg.DB, NotifyChannel, d.cfg.PollInterval, d.cfg.Logger)
	for {
		if _, err := d.RunOnce(ctx); err != nil && ctx.Err() == nil {
			d.cfg.Logger.Error("extruntime: event dispatch pass failed", "error", err)
		}
		if time.Since(d.lastPrn) > d.cfg.PruneInterval {
			d.lastPrn = time.Now()
			if _, err := d.Prune(ctx); err != nil && ctx.Err() == nil {
				d.cfg.Logger.Error("extruntime: outbox prune failed", "error", err)
			}
		}
		select {
		case <-ctx.Done():
			return
		case <-wake:
		}
	}
}

// listen returns a channel that receives on every NOTIFY of channel and at
// least every poll interval. It reconnects on connection loss.
func listen(ctx context.Context, db *pgxpool.Pool, channel string, poll time.Duration, log *slog.Logger) <-chan struct{} {
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
// (listenconn: its settings and connect hooks), never on a pooled one: a long-lived LISTEN would take a pool slot
// for the dispatcher's lifetime, and a small pool (MaxConns=1) would have none
// left for fetching, delivering and pruning.
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
			// Idle: probe the connection so a dead one is noticed.
			if perr := conn.Ping(ctx); perr != nil {
				return perr
			}
		default:
			return err
		}
	}
}
