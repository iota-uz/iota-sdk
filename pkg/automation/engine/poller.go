package engine

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/iota-uz/iota-sdk/pkg/automation/catalog"
	"github.com/iota-uz/iota-sdk/pkg/automation/code"
	"github.com/iota-uz/iota-sdk/pkg/automation/connectors"
	"github.com/iota-uz/iota-sdk/pkg/automation/graph"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

const opPoll = serrors.Op("automation.engine.poll")

// MinPollInterval is the shortest interval a Poller is polled at.
const MinPollInterval = 30 * time.Second

// PollerConfig wires the poller of inbound connectors without push.
type PollerConfig struct {
	Connections connectors.ConnectionStore
	Connectors  connectors.Directory
	// Rescan is how often the set of polled connections is read again (a graph enabled, a connection
	// added); default 1m. Between rescans the poller sleeps until the next connection is due.
	Rescan time.Duration
	// Clock is the poller's time (tests); default time.Now.
	Clock func() time.Time
}

// pollState is the poller's part of a connection's inbound state.
type pollState struct {
	Cursor json.RawMessage `json:"cursor,omitempty"`
	NextAt *time.Time      `json:"next_at,omitempty"`
	// Owner holds the connection's poll from its claim to its save, until LeaseUntil: one poller (of
	// any process) reads, polls, commits and saves a connection at a time, so a cursor never moves back.
	Owner      string     `json:"owner,omitempty"`
	LeaseUntil *time.Time `json:"lease_until,omitempty"`
}

// inboundState is the inbound state of a connection: the poller's part beside what a connector keeps.
type inboundState map[string]json.RawMessage

const pollKey = "poll"

// polled is a connection the poller polls.
type polled struct {
	tenant uuid.UUID
	id     uuid.UUID
	kind   catalog.ConnectorKind
}

// InboundPoller polls the connected connections that an enabled graph's inbound trigger uses, each at its
// connector's interval. Its cursor moves only after the events it returned are committed; a duplicate
// after a crash between the two is dropped by inbound_dedup.
type InboundPoller struct {
	r   *Runtime
	cfg PollerConfig

	mu      sync.Mutex
	due     map[uuid.UUID]time.Time // connection → when it is due next (in memory: the state keeps it too)
	scanned time.Time
	targets []polled
	// Iterations counts the passes of Serve that did work or were planned (tests).
	iterations int
}

// NewPoller builds the poller of the engine's inbound connectors.
func (r *Runtime) NewPoller(cfg PollerConfig) *InboundPoller {
	if cfg.Rescan <= 0 {
		cfg.Rescan = time.Minute
	}
	if cfg.Clock == nil {
		cfg.Clock = time.Now
	}
	return &InboundPoller{r: r, cfg: cfg, due: map[uuid.UUID]time.Time{}}
}

// Serve polls until ctx ends. It sleeps until the next connection is due or the next rescan, whichever
// comes first: with nothing to poll it wakes only for its rescans.
func (p *InboundPoller) Serve(ctx context.Context) {
	for {
		next, err := p.PollOnce(ctx)
		if err != nil && ctx.Err() == nil {
			p.r.cfg.Logger.Warn("automation poller: pass failed", "error", err)
		}
		d := time.Until(next)
		if d < 0 {
			d = 0
		}
		t := time.NewTimer(d)
		select {
		case <-ctx.Done():
			t.Stop()
			return
		case <-t.C:
		}
	}
}

// Iterations is how many passes Serve ran.
func (p *InboundPoller) Iterations() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.iterations
}

// PollOnce polls the connections that are due and returns when the next one is (or the next rescan).
func (p *InboundPoller) PollOnce(ctx context.Context) (time.Time, error) {
	now := p.cfg.Clock()
	p.mu.Lock()
	p.iterations++
	rescan := p.scanned.IsZero() || !now.Before(p.scanned.Add(p.cfg.Rescan))
	p.mu.Unlock()
	if rescan {
		targets, failed, err := p.scan(ctx)
		if err != nil {
			return now.Add(p.cfg.Rescan), err
		}
		p.mu.Lock()
		// A tenant whose catalog could not be read keeps its last targets: its failure stops nobody's
		// polling, and the next rescan tries it again.
		for _, t := range p.targets {
			if failed[t.tenant] {
				targets = append(targets, t)
			}
		}
		p.targets, p.scanned = targets, now
		keep := map[uuid.UUID]bool{}
		for _, t := range targets {
			keep[t.id] = true
		}
		for id := range p.due {
			if !keep[id] {
				delete(p.due, id)
			}
		}
		p.mu.Unlock()
	}
	p.mu.Lock()
	targets := append([]polled(nil), p.targets...)
	p.mu.Unlock()
	var errs []error
	for _, t := range targets {
		p.mu.Lock()
		at, known := p.due[t.id]
		p.mu.Unlock()
		if known && now.Before(at) {
			continue
		}
		next, err := p.pollOne(ctx, t, now)
		if err != nil {
			errs = append(errs, err)
		}
		p.mu.Lock()
		// A rescan that ran meanwhile may have dropped the target: its due time is not kept.
		if slices.ContainsFunc(p.targets, func(x polled) bool { return x.id == t.id }) {
			p.due[t.id] = next
		}
		p.mu.Unlock()
	}
	p.mu.Lock()
	next := p.scanned.Add(p.cfg.Rescan)
	for _, at := range p.due {
		if at.Before(next) {
			next = at
		}
	}
	p.mu.Unlock()
	return next, errors.Join(errs...)
}

// scan lists the connected connections of a Poller connector that an inbound trigger of an enabled
// version uses (its connection param, or the one connection of its kinds).
func (p *InboundPoller) scan(ctx context.Context) ([]polled, map[uuid.UUID]bool, error) {
	rows, err := p.r.cfg.DB.Query(ctx, `
		SELECT t.tenant_id, t.trigger_id, v.graph
		FROM automation.triggers t
		JOIN automation.automations a ON a.tenant_id = t.tenant_id AND a.id = t.automation_id AND a.enabled_version = t.version AND a.deleted_at IS NULL
		JOIN automation.versions v ON v.tenant_id = t.tenant_id AND v.automation_id = t.automation_id AND v.version = t.version
		WHERE t.kind = 'inbound'`)
	if err != nil {
		return nil, nil, serrors.FromDB(opPoll, err)
	}
	type use struct {
		tenant  uuid.UUID
		trigger string
		raw     []byte
	}
	var uses []use
	for rows.Next() {
		var u use
		if err := rows.Scan(&u.tenant, &u.trigger, &u.raw); err != nil {
			rows.Close()
			return nil, nil, serrors.FromDB(opPoll, err)
		}
		uses = append(uses, u)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, nil, serrors.FromDB(opPoll, err)
	}
	views := map[uuid.UUID]catalog.View{}
	failed := map[uuid.UUID]bool{}
	seen := map[uuid.UUID]bool{}
	var out []polled
	for _, u := range uses {
		g, errs := graph.Decode(u.raw)
		if len(errs) > 0 {
			continue
		}
		view, ok := views[u.tenant]
		if !ok {
			if failed[u.tenant] {
				continue
			}
			if view, err = p.r.cfg.Catalog.ForTenant(ctx, u.tenant); err != nil {
				failed[u.tenant] = true
				p.r.cfg.Logger.Warn("automation poller: the catalog of a tenant could not be read", "tenant_id", u.tenant.String(), "error", err)
				continue
			}
			views[u.tenant] = view
		}
		for _, t := range g.Triggers {
			if t.ID != u.trigger {
				continue
			}
			ref, kind, err := catalog.ResolveConnection(view.Connections, view.EventRequires[t.Event], t.Params[catalog.ConnectionParam])
			if err != nil {
				continue
			}
			id, err := uuid.Parse(ref.ID) // a platform credential is not polled per tenant
			if err != nil || seen[id] {
				continue
			}
			cn, ok := p.cfg.Connectors.Get(kind)
			if _, isPoller := cn.(Poller); !ok || !isPoller {
				continue
			}
			seen[id] = true
			out = append(out, polled{tenant: u.tenant, id: id, kind: kind})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].id.String() < out[j].id.String() })
	return out, failed, nil
}

// pollOne polls one connection when its state says it is due; it returns when it is due next.
func (p *InboundPoller) pollOne(ctx context.Context, t polled, now time.Time) (time.Time, error) {
	cn, _ := p.cfg.Connectors.Get(t.kind)
	poller := cn.(Poller)
	c, creds, err := p.cfg.Connections.Get(ctx, t.tenant, t.id)
	if errors.Is(err, connectors.ErrNotFound) {
		return now.Add(p.cfg.Rescan), nil
	}
	if err != nil {
		return now.Add(MinPollInterval), serrors.Wrap(opPoll, err)
	}
	interval := poller.Interval(c)
	if interval < MinPollInterval {
		interval = MinPollInterval
	}
	if c.Status != connectors.StatusConnected {
		return now.Add(p.cfg.Rescan), nil // re-authorization comes first; a rescan sees it
	}
	state, owner, wait, err := p.claim(ctx, t, interval)
	if err != nil {
		return p.cfg.Clock().Add(interval), err
	}
	if owner == "" {
		return wait, nil // another process (or before a restart) polled it, or polls it now
	}
	// The next poll is due an interval after this one starts (after the claim, however long its lock
	// waited), whenever the pass began.
	now = p.cfg.Clock()
	// release ends the claim keeping the cursor (a poll that moved nothing).
	release := func() error {
		return p.saveState(ctx, t, owner, pollState{Cursor: state.Cursor, NextAt: ptrTime(now.Add(interval))})
	}
	evs, next, err := poller.Poll(ctx, c, creds, state.Cursor)
	if err != nil {
		p.r.metrics.pollError(ctx, string(t.kind))
		if errors.Is(err, connectors.ErrNeedsReauth) || isUnauthorized(err) {
			// Polling stops until the connection is authorized again, only while this poll still owns the
			// connection and its credentials are still the connection's (version, checked and set under the
			// row lock): a poll that outlived its claim, or whose credentials a reauthorization replaced
			// meanwhile, does not stop polling with credentials that work.
			set, serr := p.cfg.Connections.SetStatusIf(ctx, t.tenant, t.id, connectors.StatusNeedsReauth, "the provider refused the credentials",
				func(raw json.RawMessage, version int) bool { return pollOwner(raw) == owner && version == c.Version })
			if serr != nil {
				return now.Add(interval), errors.Join(serrors.Wrap(opPoll, serr), release())
			}
			if !set {
				// The claim was lost (the newer owner decides) or the credentials changed: the next poll
				// uses the new ones.
				return now.Add(interval), release()
			}
			return now.Add(p.cfg.Rescan), release()
		}
		// A network failure keeps the cursor: the next poll asks again from it. The provider's error may
		// echo the credentials the connector used: it is logged masked.
		p.r.cfg.Logger.Warn("automation poller: poll failed", "connection_id", t.id.String(), "kind", string(t.kind),
			"error", code.NewMasker(map[string]string(creds)).Mask(err.Error()))
		return now.Add(interval), release()
	}
	// The batch and the cursor commit together, under the connection's row lock: only while this poll
	// still owns the claim and its credentials (the version it read, still connected) are the
	// connection's. Events and a cursor of credentials a reauthorization replaced meanwhile are dropped.
	err = p.commitPoll(ctx, t, owner, c.Version, evs, pollState{Cursor: next, NextAt: ptrTime(now.Add(interval))})
	switch {
	case errors.Is(err, errClaimLost):
		return now.Add(interval), nil // the batch is discarded; the new owner's cursor stands
	case errors.Is(err, errCredentialsChanged):
		// The next poll asks again from the same cursor with the connection's credentials now. A token
		// refresh during the poll changes the version too: its batch comes again then, nothing is lost.
		return now.Add(interval), release()
	case err != nil:
		return now.Add(interval), errors.Join(serrors.Wrap(opPoll, err), release()) // the cursor stays: the events come again
	}
	return now.Add(interval), nil
}

// commitPoll publishes a poll's events and saves its state in one transaction under the connection's
// row lock, fenced by the claim's owner and the credentials version the poll read.
func (p *InboundPoller) commitPoll(ctx context.Context, t polled, owner string, version int, evs []InboundEvent, st pollState) error {
	in := p.r.Inbound()
	return publishInTx(ctx, p.r.cfg.DB, func(ctx context.Context, tx pgx.Tx) error {
		var (
			raw    []byte
			ver    int
			status string
		)
		if err := tx.QueryRow(ctx, `SELECT inbound, version, status FROM automation.connections WHERE tenant_id = $1 AND id = $2 FOR UPDATE`,
			t.tenant, t.id).Scan(&raw, &ver, &status); err != nil {
			return err
		}
		if pollOwner(raw) != owner {
			return errClaimLost
		}
		if ver != version || status != string(connectors.StatusConnected) {
			return errCredentialsChanged
		}
		for _, ev := range evs {
			ev.TenantID, ev.ConnectionID = t.tenant, t.id.String()
			sp, err := tx.Begin(ctx)
			if err != nil {
				return err
			}
			if _, err := in.Publish(ctx, sp, ev); err != nil {
				_ = sp.Rollback(ctx)
				if errors.Is(err, ErrInboundInvalid) || errors.Is(err, ErrCorrelationClosed) {
					continue
				}
				return err
			}
			if err := sp.Commit(ctx); err != nil {
				return err
			}
		}
		// The cursor moves with the events it read, and the claim ends.
		all := inboundState{}
		if len(raw) > 0 {
			_ = json.Unmarshal(raw, &all)
			if all == nil {
				all = inboundState{}
			}
		}
		b, err := json.Marshal(st)
		if err != nil {
			return err
		}
		all[pollKey] = b
		next, err := json.Marshal(all)
		if err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `UPDATE automation.connections SET inbound = $3, updated_at = now() WHERE tenant_id = $1 AND id = $2`, t.tenant, t.id, next)
		return err
	})
}

func isUnauthorized(err error) bool {
	var ce *catalog.CallError
	return errors.As(err, &ce) && ce.Code == catalog.CallUnauthorized
}

func ptrTime(t time.Time) *time.Time { return &t }

// errClaimLost: the poll's claim of the connection passed to another poller before its batch committed.
var errClaimLost = errors.New("automation poller: the claim of the connection was lost")

// errCredentialsChanged: the connection's credentials (or status) changed while the poll ran.
var errCredentialsChanged = errors.New("automation poller: the connection's credentials changed during the poll")

// pollOwner is the owner of the poll claim an inbound state holds.
func pollOwner(raw json.RawMessage) string {
	var all inboundState
	if len(raw) == 0 || json.Unmarshal(raw, &all) != nil {
		return ""
	}
	var st pollState
	if b, ok := all[pollKey]; ok {
		_ = json.Unmarshal(b, &st)
	}
	return st.Owner
}

// pollLease is how long a claim holds a connection beyond its interval: a poll (and its commit) that
// outlives it loses the connection to the next claim, and its save is dropped.
const pollLease = 5 * time.Minute

// claim takes the connection's poll under the row lock when it is due and no live claim holds it: the
// state it returns is the one to poll from, with the claim's owner token. When the connection is not the
// caller's to poll, owner is empty and wait is when to look again. The claim also sets the next poll an
// interval away, so the polls of a connection are at least an interval apart whichever process runs them.
// Its time is read under the lock: a claim that waited for the lock is due by when it got it.
func (p *InboundPoller) claim(ctx context.Context, t polled, interval time.Duration) (pollState, string, time.Time, error) {
	var st pollState
	var owner string
	var wait time.Time
	err := p.cfg.Connections.UpdateInbound(ctx, t.tenant, t.id, func(raw json.RawMessage) (json.RawMessage, error) {
		all := inboundState{}
		if len(raw) > 0 {
			_ = json.Unmarshal(raw, &all)
			if all == nil {
				all = inboundState{}
			}
		}
		now := p.cfg.Clock()
		st = pollState{}
		if b, ok := all[pollKey]; ok {
			_ = json.Unmarshal(b, &st)
		}
		if st.Owner != "" && st.LeaseUntil != nil && now.Before(*st.LeaseUntil) {
			wait = *st.LeaseUntil
			if st.NextAt != nil && st.NextAt.After(wait) {
				wait = *st.NextAt
			}
			return raw, nil
		}
		if st.NextAt != nil && now.Before(*st.NextAt) {
			wait = *st.NextAt
			return raw, nil
		}
		owner = uuid.NewString()
		claimed := st
		claimed.Owner, claimed.LeaseUntil, claimed.NextAt = owner, ptrTime(now.Add(interval+pollLease)), ptrTime(now.Add(interval))
		b, err := json.Marshal(claimed)
		if err != nil {
			return nil, err
		}
		all[pollKey] = b
		return json.Marshal(all)
	})
	if err != nil {
		return pollState{}, "", time.Time{}, serrors.Wrap(opPoll, err)
	}
	return st, owner, wait, nil
}

// saveState ends the owner's claim with the state it polled to. A claim lost to another (its lease ran
// out) saves nothing: the newer owner's cursor stands.
func (p *InboundPoller) saveState(ctx context.Context, t polled, owner string, st pollState) error {
	err := p.cfg.Connections.UpdateInbound(ctx, t.tenant, t.id, func(raw json.RawMessage) (json.RawMessage, error) {
		all := inboundState{}
		if len(raw) > 0 {
			_ = json.Unmarshal(raw, &all)
			if all == nil {
				all = inboundState{}
			}
		}
		var cur pollState
		if b, ok := all[pollKey]; ok {
			_ = json.Unmarshal(b, &cur)
		}
		if cur.Owner != owner {
			return raw, nil
		}
		b, err := json.Marshal(st)
		if err != nil {
			return nil, err
		}
		all[pollKey] = b
		return json.Marshal(all)
	})
	return serrors.Wrap(opPoll, err)
}
