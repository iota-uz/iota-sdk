package events

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/iota-uz/iota-sdk/pkg/extruntime/host"
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

// Subscription is one active subscription of an activated package version.
type Subscription struct {
	ID       string
	Package  string
	Version  string
	Export   string
	Event    string
	Artifact pool.Artifact
	Secrets  []string
	// AfterSeq is the activation watermark (see Watermark): events with a
	// lower or equal sequence number are never delivered to this
	// subscription. A handler update does not replay the journal.
	AfterSeq int64
}

// SubscriptionSource lists the active subscriptions; the management slice
// implements it over activations.
type SubscriptionSource interface {
	ActiveSubscriptions(ctx context.Context) ([]Subscription, error)
}

// Runner executes an export; *host.Host implements it.
type Runner interface {
	Run(ctx context.Context, req host.RunRequest) (host.Result, error)
}

// Config configures the dispatcher. Zero values select the defaults.
type Config struct {
	DB            *pgxpool.Pool
	Runner        Runner
	Subscriptions SubscriptionSource
	Subjects      host.SubjectLoader

	// PollInterval is the fallback poll period when no NOTIFY arrives. Default 2s.
	PollInterval time.Duration
	// BatchSize bounds events fetched per subscription and pass. Default 50.
	BatchSize int
	// MaxAttempts before a poisoned event is dead-lettered. Default 5.
	MaxAttempts int
	// BackoffBase doubles per attempt up to BackoffMax. Defaults 1s and 5m.
	BackoffBase, BackoffMax time.Duration
	// BlockedRecheck is how often a blocked delivery (no owner, subject
	// refused) is re-evaluated. Default 30s.
	BlockedRecheck time.Duration
	// Retention is how long delivered events stay in the outbox. Default 7d.
	Retention     time.Duration
	PruneInterval time.Duration // default 1h
	Logger        *slog.Logger
}

func (c *Config) defaults() error {
	if c.DB == nil || c.Runner == nil || c.Subscriptions == nil || c.Subjects == nil {
		return serrors.New(serrors.Invalid, "extruntime events: DB, Runner, Subscriptions and Subjects are required")
	}
	if c.PollInterval <= 0 {
		c.PollInterval = 2 * time.Second
	}
	if c.BatchSize <= 0 {
		c.BatchSize = 50
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
		c.Retention = 7 * 24 * time.Hour
	}
	if c.PruneInterval <= 0 {
		c.PruneInterval = time.Hour
	}
	if c.Logger == nil {
		c.Logger = slog.Default()
	}
	return nil
}

// Dispatcher delivers outbox events to active subscriptions after commit.
//
// Only rows whose inserting transaction is older than every running
// transaction are read (xmin gating), so a transaction that took a lower seq
// but commits later is never skipped; the activation watermark only excludes
// events older than the activation. Each delivery is its own transaction with
// the subscription receipt, so a duplicate delivery is a no-op, and a poisoned
// event is retried with backoff and dead-lettered without blocking later ones.
type Dispatcher struct {
	cfg Config

	mu      sync.Mutex
	cancel  context.CancelFunc
	done    chan struct{}
	lastPrn time.Time
}

func NewDispatcher(cfg Config) (*Dispatcher, error) {
	if err := cfg.defaults(); err != nil {
		return nil, err
	}
	return &Dispatcher{cfg: cfg}, nil
}

type outboxRow struct {
	id        string
	seq       int64
	tenant    uuid.UUID
	event     string
	subEntity *string
	subKey    *string
	payload   json.RawMessage
	initiator *int64
	occurred  time.Time
	status    string
	attempts  int
}

const fetchSQL = `
SELECT e.id::text, e.seq, e.tenant_id, e.event, e.subject_entity, e.subject_key, e.payload, e.initiator_user_id, e.occurred_at,
       COALESCE(r.status, ''), COALESCE(r.attempts, 0)
FROM extruntime.outbox_events e
LEFT JOIN extruntime.subscription_receipts r ON r.subscription = $1 AND r.event_id = e.id
WHERE e.event = $2
  AND e.seq > $3
  AND e.xmin::text::xid8 < pg_snapshot_xmin(pg_current_snapshot())
  AND (r.event_id IS NULL OR (r.status IN ('pending', 'failed', 'blocked') AND r.next_at <= now()))
ORDER BY e.seq
LIMIT $4`

// RunOnce delivers everything currently deliverable to every active
// subscription and returns the number of successful deliveries. Subscriptions
// are independent: they run concurrently and one failing never holds back
// another.
func (d *Dispatcher) RunOnce(ctx context.Context) (int, error) {
	subs, err := d.cfg.Subscriptions.ActiveSubscriptions(ctx)
	if err != nil {
		return 0, serrors.Wrap(opRunOnce, err)
	}
	var (
		wg    sync.WaitGroup
		mu    sync.Mutex
		total int
		first error
	)
	for _, sub := range subs {
		wg.Add(1)
		go func() {
			defer wg.Done()
			n, err := d.runSubscription(ctx, sub)
			mu.Lock()
			total += n
			if err != nil && first == nil {
				first = err
			}
			mu.Unlock()
		}()
	}
	wg.Wait()
	if first != nil {
		return total, serrors.Wrap(opRunOnce, first)
	}
	return total, nil
}

func (d *Dispatcher) runSubscription(ctx context.Context, sub Subscription) (int, error) {
	delivered := 0
	for {
		if ctx.Err() != nil {
			return delivered, ctx.Err()
		}
		rows, err := d.fetch(ctx, sub)
		if err != nil {
			return delivered, err
		}
		if len(rows) == 0 {
			return delivered, nil
		}
		progressed := 0
		for _, row := range rows {
			ok, err := d.deliver(ctx, sub, row)
			if err != nil {
				return delivered, err
			}
			if ok {
				delivered++
			}
			progressed++
		}
		if progressed < d.cfg.BatchSize {
			return delivered, nil
		}
	}
}

func (d *Dispatcher) fetch(ctx context.Context, sub Subscription) ([]outboxRow, error) {
	rows, err := d.cfg.DB.Query(ctx, fetchSQL, sub.ID, sub.Event, sub.AfterSeq, d.cfg.BatchSize)
	if err != nil {
		return nil, serrors.FromDB(opRunOnce, err)
	}
	defer rows.Close()
	var out []outboxRow
	for rows.Next() {
		var r outboxRow
		if err := rows.Scan(&r.id, &r.seq, &r.tenant, &r.event, &r.subEntity, &r.subKey, &r.payload, &r.initiator, &r.occurred, &r.status, &r.attempts); err != nil {
			return nil, serrors.FromDB(opRunOnce, err)
		}
		out = append(out, r)
	}
	return out, serrors.FromDB(opRunOnce, rows.Err())
}

// deliver runs the subscription export for one event. It returns true when
// the handler ran and its transaction (with the receipt) committed.
func (d *Dispatcher) deliver(ctx context.Context, sub Subscription, ev outboxRow) (bool, error) {
	subject, blocked, err := d.subjectFor(ctx, sub, ev)
	if err != nil {
		return false, serrors.Wrap(opDeliver, err)
	}
	if blocked != "" {
		return false, d.recordBlocked(ctx, sub, ev, blocked)
	}

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
		RETURNING attempts`, sub.ID, ev.id, ev.tenant).Scan(&attempts)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil // already delivered or dead: duplicate delivery is a no-op
	}
	if err != nil {
		return false, serrors.FromDB(opDeliver, err)
	}

	input, err := d.eventInput(ev)
	if err != nil {
		return false, serrors.Wrap(opDeliver, err)
	}
	res, runErr := d.cfg.Runner.Run(ctx, host.RunRequest{
		Export: host.Export{
			Kind:     pool.KindSubscription,
			Resource: sub.ID,
			Binding: host.Binding{
				Contract: sub.ID, Package: sub.Package, Version: sub.Version, Export: sub.Export,
				Artifact: sub.Artifact, Secrets: sub.Secrets,
			},
		},
		Input:        input,
		Subject:      subject,
		Trigger:      host.Trigger{Kind: host.TriggerSubscription, Ref: sub.ID},
		Tx:           tx,
		Producer:     spi.Producer{EventID: ev.id},
		StartKeyBase: sub.ID + ":" + ev.id,
	})
	if runErr == nil {
		if res.InvocationID != "" {
			if _, err := tx.Exec(ctx, `UPDATE extruntime.subscription_receipts SET invocation_id = $3::uuid WHERE subscription = $1 AND event_id = $2::uuid`,
				sub.ID, ev.id, res.InvocationID); err != nil {
				return false, serrors.FromDB(opDeliver, err)
			}
		}
		if err := tx.Commit(ctx); err != nil {
			return false, serrors.FromDB(opDeliver, err)
		}
		return true, nil
	}
	if ctx.Err() != nil {
		return false, ctx.Err()
	}
	_ = tx.Rollback(context.WithoutCancel(ctx))
	return false, d.recordFailure(ctx, sub, ev, attempts, res.InvocationID, runErr)
}

// subjectFor resolves who the handler acts as: the event initiator with
// current rights, or the explicit owner of the subscription in the tenant.
// A non-empty blocked string is the diagnostic of a refused delivery.
func (d *Dispatcher) subjectFor(ctx context.Context, sub Subscription, ev outboxRow) (spi.Subject, string, error) {
	userID, kind := int64(0), spi.SubjectInitiator
	if ev.initiator != nil {
		userID = *ev.initiator
	} else {
		kind = spi.SubjectOwner
		err := d.cfg.DB.QueryRow(ctx, `SELECT user_id FROM extruntime.trigger_owners WHERE trigger_id = $1 AND tenant_id = $2`, sub.ID, ev.tenant).Scan(&userID)
		if errors.Is(err, pgx.ErrNoRows) {
			return spi.Subject{}, fmt.Sprintf("event %s has no initiator and subscription %s has no owner-user in tenant %s", ev.id, sub.ID, ev.tenant), nil
		}
		if err != nil {
			return spi.Subject{}, "", serrors.FromDB(opDeliver, err)
		}
	}
	subject, err := d.cfg.Subjects.LoadSubject(ctx, ev.tenant, userID, kind)
	if errors.Is(err, host.ErrSubjectBlocked) {
		return spi.Subject{}, err.Error(), nil
	}
	if err != nil {
		return spi.Subject{}, "", err
	}
	subject.TenantID, subject.UserID, subject.Kind = ev.tenant, userID, kind
	return subject, "", nil
}

func (d *Dispatcher) eventInput(ev outboxRow) (json.RawMessage, error) {
	var subject *spi.ObjectRef
	if ev.subEntity != nil && ev.subKey != nil {
		subject = &spi.ObjectRef{Entity: *ev.subEntity, Key: *ev.subKey}
	}
	var initiator *string
	if ev.initiator != nil {
		s := strconv.FormatInt(*ev.initiator, 10)
		initiator = &s
	}
	payload := ev.payload
	if len(payload) == 0 {
		payload = json.RawMessage(`null`)
	}
	return json.Marshal(struct {
		ID         string          `json:"id"`
		Event      string          `json:"event"`
		OccurredAt string          `json:"occurredAt"`
		Subject    *spi.ObjectRef  `json:"subject"`
		Payload    json.RawMessage `json:"payload"`
		Initiator  *string         `json:"initiator"`
	}{ev.id, ev.event, ev.occurred.UTC().Format(time.RFC3339Nano), subject, payload, initiator})
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
func (d *Dispatcher) recordFailure(ctx context.Context, sub Subscription, ev outboxRow, attempts int, invocationID string, runErr error) error {
	status := StatusFailed
	if host.IsInfrastructure(runErr) {
		attempts = max(attempts-1, 0)
	} else if attempts >= d.cfg.MaxAttempts {
		status = StatusDead
	}
	var inv any
	if invocationID != "" {
		inv = invocationID
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
		sub.ID, ev.id, ev.tenant, status, attempts, d.backoff(max(attempts, 1)).String(), msg, inv)
	if err != nil {
		return serrors.FromDB(opDeliver, err)
	}
	d.cfg.Logger.Warn("extruntime: subscription delivery failed", "subscription", sub.ID, "event", ev.id, "attempt", attempts, "status", status, "error", msg)
	return nil
}

func (d *Dispatcher) recordBlocked(ctx context.Context, sub Subscription, ev outboxRow, diagnostic string) error {
	_, err := d.cfg.DB.Exec(ctx, `
		INSERT INTO extruntime.subscription_receipts AS r (subscription, event_id, tenant_id, status, attempts, next_at, last_error)
		VALUES ($1, $2::uuid, $3, 'blocked', 0, now() + $4::interval, $5)
		ON CONFLICT (subscription, event_id) DO UPDATE
		SET status = 'blocked', next_at = EXCLUDED.next_at, last_error = EXCLUDED.last_error, updated_at = now()
		WHERE r.status IN ('pending', 'failed', 'blocked')`,
		sub.ID, ev.id, ev.tenant, d.cfg.BlockedRecheck.String(), diagnostic)
	return serrors.FromDB(opDeliver, err)
}

// Prune deletes delivered events older than the retention that have no
// delivery still pending.
func (d *Dispatcher) Prune(ctx context.Context) (int64, error) {
	tag, err := d.cfg.DB.Exec(ctx, `
		DELETE FROM extruntime.outbox_events e
		WHERE e.occurred_at < now() - $1::interval
		  AND e.xmin::text::xid8 < pg_snapshot_xmin(pg_current_snapshot())
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
			// Idle: probe the connection so a dead one is noticed.
			if perr := conn.Conn().Ping(ctx); perr != nil {
				return perr
			}
		default:
			return err
		}
	}
}
