package engine

import (
	"context"
	"encoding/json"
	"errors"
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
		targets, err := p.scan(ctx)
		if err != nil {
			return now.Add(p.cfg.Rescan), err
		}
		p.mu.Lock()
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
		p.due[t.id] = next
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
func (p *InboundPoller) scan(ctx context.Context) ([]polled, error) {
	rows, err := p.r.cfg.DB.Query(ctx, `
		SELECT t.tenant_id, t.trigger_id, v.graph
		FROM automation.triggers t
		JOIN automation.automations a ON a.tenant_id = t.tenant_id AND a.id = t.automation_id AND a.enabled_version = t.version AND a.deleted_at IS NULL
		JOIN automation.versions v ON v.tenant_id = t.tenant_id AND v.automation_id = t.automation_id AND v.version = t.version
		WHERE t.kind = 'inbound'`)
	if err != nil {
		return nil, serrors.FromDB(opPoll, err)
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
			return nil, serrors.FromDB(opPoll, err)
		}
		uses = append(uses, u)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, serrors.FromDB(opPoll, err)
	}
	views := map[uuid.UUID]catalog.View{}
	seen := map[uuid.UUID]bool{}
	var out []polled
	for _, u := range uses {
		g, errs := graph.Decode(u.raw)
		if len(errs) > 0 {
			continue
		}
		view, ok := views[u.tenant]
		if !ok {
			if view, err = p.r.cfg.Catalog.ForTenant(ctx, u.tenant); err != nil {
				return nil, serrors.Wrap(opPoll, err)
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
	return out, nil
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
	state, err := p.state(ctx, t)
	if err != nil {
		return now.Add(interval), err
	}
	if state.NextAt != nil && now.Before(*state.NextAt) {
		return *state.NextAt, nil // another process (or before a restart) polled it
	}
	evs, next, err := poller.Poll(ctx, c, creds, state.Cursor)
	if err != nil {
		p.r.metrics.pollError(ctx, string(t.kind))
		if errors.Is(err, connectors.ErrNeedsReauth) || isUnauthorized(err) {
			// Polling stops until the connection is authorized again.
			if serr := p.cfg.Connections.SetStatus(ctx, t.tenant, t.id, connectors.StatusNeedsReauth, "the provider refused the credentials"); serr != nil {
				return now.Add(interval), serrors.Wrap(opPoll, serr)
			}
			return now.Add(p.cfg.Rescan), nil
		}
		// A network failure keeps the cursor: the next poll asks again from it. The provider's error may
		// echo the credentials the connector used: it is logged masked.
		p.r.cfg.Logger.Warn("automation poller: poll failed", "connection_id", t.id.String(), "kind", string(t.kind),
			"error", code.NewMasker(map[string]string(creds)).Mask(err.Error()))
		return now.Add(interval), p.saveState(ctx, t, pollState{Cursor: state.Cursor, NextAt: ptrTime(now.Add(interval))})
	}
	if len(evs) > 0 {
		in := p.r.Inbound()
		err := pgx.BeginFunc(ctx, p.r.cfg.DB, func(tx pgx.Tx) error {
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
			return nil
		})
		if err != nil {
			return now.Add(interval), serrors.Wrap(opPoll, err) // the cursor stays: the events come again
		}
	}
	// The cursor moves after the events are committed.
	return now.Add(interval), p.saveState(ctx, t, pollState{Cursor: next, NextAt: ptrTime(now.Add(interval))})
}

func isUnauthorized(err error) bool {
	var ce *catalog.CallError
	return errors.As(err, &ce) && ce.Code == catalog.CallUnauthorized
}

func ptrTime(t time.Time) *time.Time { return &t }

func (p *InboundPoller) state(ctx context.Context, t polled) (pollState, error) {
	var st pollState
	err := p.cfg.Connections.UpdateInbound(ctx, t.tenant, t.id, func(raw json.RawMessage) (json.RawMessage, error) {
		var all inboundState
		if len(raw) > 0 {
			_ = json.Unmarshal(raw, &all)
		}
		if b, ok := all[pollKey]; ok {
			_ = json.Unmarshal(b, &st)
		}
		return raw, nil
	})
	return st, serrors.Wrap(opPoll, err)
}

func (p *InboundPoller) saveState(ctx context.Context, t polled, st pollState) error {
	err := p.cfg.Connections.UpdateInbound(ctx, t.tenant, t.id, func(raw json.RawMessage) (json.RawMessage, error) {
		all := inboundState{}
		if len(raw) > 0 {
			_ = json.Unmarshal(raw, &all)
			if all == nil {
				all = inboundState{}
			}
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
