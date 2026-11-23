package engine

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/iota-uz/iota-sdk/pkg/automation/catalog"
	"github.com/iota-uz/iota-sdk/pkg/automation/connectors"
	"github.com/iota-uz/iota-sdk/pkg/automation/expr"
	"github.com/iota-uz/iota-sdk/pkg/automation/graph"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/events"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

const opInbound = serrors.Op("automation.engine.inbound")

// Inbound publishes connector-originated events into the outbox of #1320 in tx: the engine's consumer
// starts triggers and wakes waits on them as on any native event.
type Inbound interface {
	Publish(ctx context.Context, tx pgx.Tx, ev InboundEvent) (published bool, err error) // false: duplicate DedupKey
	// Correlation finds the tenant of an open Await by token (platform credentials); false → closed or unknown.
	Correlation(ctx context.Context, token string) (tenantID uuid.UUID, ok bool, err error)
}

// InboundEvent is one event a connector received.
type InboundEvent struct {
	TenantID     uuid.UUID       // for a platform credential: resolved by the connector (correlation, own rules)
	ConnectionID string          // UUID or "platform:<kind>" (catalog.ConnectionInfo.ID of #1330)
	Event        string          // key of graph.EventSpec from the connector's TriggerSource.Events() (#1330)
	DedupKey     string          // provider id: update_id, Message-ID, wamid…; unique per (connection, event)
	Correlation  string          // optional: token of an open Await
	Subject      SubjectRef      // usually {Entity: "conversation", Key: "<connection_id>:<chat>"}
	Payload      json.RawMessage // validated against the event payload schema; ≤ Limits.PayloadBytes
	OccurredAt   time.Time
	// BySelf marks a message the connector sent itself (its own bot, its own mailbox): event.by_self.
	// Triggers do not start on it unless include_self.
	BySelf bool
}

// Errors of Inbound.Publish.
var (
	ErrInboundInvalid = errors.New("automation: inbound event is invalid")
)

// Inbound is the engine's Inbound.
func (r *Runtime) Inbound() Inbound { return &inbound{r: r, specs: map[string]graph.ParamsSpec{}} }

type inbound struct {
	r     *Runtime
	mu    sync.Mutex
	specs map[string]graph.ParamsSpec // event key → its payload validator
}

func (in *inbound) connector(ev InboundEvent) string {
	if kind, ok := strings.CutPrefix(ev.ConnectionID, catalog.PlatformPrefix); ok {
		return kind
	}
	if k, _, ok := strings.Cut(ev.Event, "."); ok {
		return k
	}
	return "unknown"
}

// validator is the payload schema of an event, compiled once.
func (in *inbound) validator(spec graph.EventSpec) (graph.ParamsSpec, error) {
	in.mu.Lock()
	defer in.mu.Unlock()
	if p, ok := in.specs[spec.Key]; ok {
		return p, nil
	}
	p, err := graph.SchemaParams(spec.Payload.JSONSchema())
	if err != nil {
		return nil, err
	}
	in.specs[spec.Key] = p
	return p, nil
}

func (in *inbound) Publish(ctx context.Context, tx pgx.Tx, ev InboundEvent) (bool, error) {
	published, err := in.publish(ctx, tx, ev)
	switch {
	case err != nil:
		in.r.metrics.inboundEvent(ctx, in.connector(ev), "rejected")
	case published:
		in.r.metrics.inboundEvent(ctx, in.connector(ev), "published")
	default:
		in.r.metrics.inboundEvent(ctx, in.connector(ev), "duplicate")
	}
	return published, err
}

func (in *inbound) publish(ctx context.Context, tx pgx.Tx, ev InboundEvent) (bool, error) {
	switch {
	case ev.TenantID == uuid.Nil:
		return false, fmt.Errorf("%w: no tenant", ErrInboundInvalid)
	case ev.ConnectionID == "" || ev.Event == "" || ev.DedupKey == "":
		return false, fmt.Errorf("%w: connection, event and dedup key are required", ErrInboundInvalid)
	case len(ev.Payload) > in.r.cfg.Limits.PayloadBytes:
		return false, fmt.Errorf("%w: the payload has %d bytes, at most %d", ErrInboundInvalid, len(ev.Payload), in.r.cfg.Limits.PayloadBytes)
	}
	view, err := in.r.cfg.Catalog.ForTenant(ctx, ev.TenantID)
	if err != nil {
		return false, serrors.Wrap(opInbound, err)
	}
	spec, ok := view.Catalog.Events[ev.Event]
	if !ok || len(view.EventRequires[ev.Event]) == 0 {
		return false, fmt.Errorf("%w: %s is not an inbound event of the tenant's catalog", ErrInboundInvalid, ev.Event)
	}
	payload := map[string]any{}
	if len(ev.Payload) > 0 {
		if err := expr.UnmarshalJSON(ev.Payload, &payload); err != nil || payload == nil {
			return false, fmt.Errorf("%w: the payload is not a JSON object", ErrInboundInvalid)
		}
	}
	delete(payload, catalog.BySelfField) // the engine's own field: set from BySelf
	if p, err := in.validator(spec); err == nil {
		if _, issues := p.Parse(payload); len(issues) > 0 {
			return false, fmt.Errorf("%w: payload of %s: %s", ErrInboundInvalid, ev.Event, issues[0].Message)
		}
	}
	subject := normSubject(ev.Subject)
	if spec.Subject != graph.SubjectNone && subject.Entity != spec.Subject {
		return false, fmt.Errorf("%w: %s is about a %s, not a %s", ErrInboundInvalid, ev.Event, spec.Subject, subject.Entity)
	}
	tag, err := tx.Exec(ctx, `
		INSERT INTO automation.inbound_dedup (tenant_id, connection_id, event, dedup_key) VALUES ($1, $2, $3, $4)
		ON CONFLICT DO NOTHING`, ev.TenantID, ev.ConnectionID, ev.Event, ev.DedupKey)
	if err != nil {
		return false, serrors.FromDB(opInbound, err)
	}
	if tag.RowsAffected() == 0 {
		return false, nil
	}
	if ev.Correlation != "" {
		if err := in.takeCorrelation(ctx, tx, ev.TenantID, ev.Correlation); err != nil {
			return false, err
		}
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return false, err
	}
	cause, err := json.Marshal(cause{Inbound: &inboundCause{ConnectionID: ev.ConnectionID, Correlation: ev.Correlation, BySelf: ev.BySelf}})
	if err != nil {
		return false, err
	}
	var entity, key any
	if subject.Entity != graph.SubjectNone {
		entity, key = subject.Entity, subject.Key
	}
	occurred := ev.OccurredAt
	if occurred.IsZero() {
		occurred = time.Now()
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO extruntime.outbox_events (id, tenant_id, event, subject_entity, subject_key, payload, occurred_at, cause)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		uuid.New(), ev.TenantID, ev.Event, entity, key, body, occurred, cause); err != nil {
		return false, serrors.FromDB(opInbound, err)
	}
	if _, err := tx.Exec(ctx, `SELECT pg_notify($1, '')`, events.NotifyChannel); err != nil {
		return false, serrors.FromDB(opInbound, err)
	}
	return true, nil
}

// takeCorrelation checks that the token names an open wait of the tenant; a single answer closes it, so
// a second answer is ErrCorrelationClosed.
func (in *inbound) takeCorrelation(ctx context.Context, tx pgx.Tx, tenant uuid.UUID, token string) error {
	if !validToken(token) {
		return ErrCorrelationClosed
	}
	var single bool
	err := tx.QueryRow(ctx, `SELECT single FROM automation.correlations WHERE token = $1 AND tenant_id = $2 AND expires_at > now() FOR UPDATE`,
		token, tenant).Scan(&single)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrCorrelationClosed
	}
	if err != nil {
		return serrors.FromDB(opInbound, err)
	}
	if single {
		if _, err := tx.Exec(ctx, `DELETE FROM automation.correlations WHERE token = $1`, token); err != nil {
			return serrors.FromDB(opInbound, err)
		}
	}
	return nil
}

func (in *inbound) Correlation(ctx context.Context, token string) (uuid.UUID, bool, error) {
	if !validToken(token) {
		return uuid.Nil, false, nil
	}
	var tenant uuid.UUID
	err := in.r.cfg.DB.QueryRow(ctx, `SELECT tenant_id FROM automation.correlations WHERE token = $1 AND expires_at > now()`, token).Scan(&tenant)
	if errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, false, nil
	}
	if err != nil {
		return uuid.Nil, false, serrors.FromDB(opInbound, err)
	}
	return tenant, true, nil
}

// WebhookReceiver is an optional interface of connectors.Connector.
type WebhookReceiver interface {
	// Verify checks the request authenticity (HMAC, secret header) with the connection credentials.
	Verify(r *http.Request, body []byte, c connectors.Connection, creds connectors.Credentials) error
	// Handle parses the body into events; it may answer the provider synchronously (challenge, ack).
	Handle(ctx context.Context, w http.ResponseWriter, body []byte, c connectors.Connection, creds connectors.Credentials) ([]InboundEvent, error)
}

// ClosedAnswerer is an optional interface of a WebhookReceiver: the answer to the provider for an event
// whose correlation closed meanwhile (a second press of a button: "already answered").
type ClosedAnswerer interface {
	AnswerClosed(ctx context.Context, ev InboundEvent, c connectors.Connection, creds connectors.Credentials)
}

// Poller is an optional interface of connectors.Connector for providers without push.
type Poller interface {
	Interval(c connectors.Connection) time.Duration // ≥ 30s
	Poll(ctx context.Context, c connectors.Connection, creds connectors.Credentials, cursor json.RawMessage) (events []InboundEvent, next json.RawMessage, err error)
}

// InboundConfig wires InboundHandler.
type InboundConfig struct {
	Connections connectors.ConnectionStore
	Registry    catalog.Registry
	Inbound     Inbound
	MaxBody     int64 // default 1 MiB
	// DB resolves the tenant of a connection and runs the publishing transaction.
	DB *pgxpool.Pool
	// Connectors finds the connector of a connection's kind.
	Connectors connectors.Directory
	// Platform holds the platform credentials of /automations/hooks/platform/{kind}.
	Platform connectors.PlatformCredentials
	// TenantOf, when set, is the tenant of the request's host: a connection of another tenant is 404.
	TenantOf func(r *http.Request) (uuid.UUID, bool)
}

// InboundHandler serves POST/GET /automations/hooks/{connection_id} and /automations/hooks/platform/{kind};
// mounted by modules/automations. For platform routes Verify/Handle get the platform Connection and
// credentials. An unknown connection, a connection of another tenant and a failed Verify are 404 without
// a body; a body over MaxBody is 413; the provider gets 200 after the outbox commit, 5xx on a failure
// of the database so it repeats.
func InboundHandler(cfg InboundConfig) http.Handler {
	if cfg.MaxBody <= 0 {
		cfg.MaxBody = 1 << 20
	}
	return &inboundHandler{cfg: cfg}
}

type inboundHandler struct{ cfg InboundConfig }

func notFound(w http.ResponseWriter) { w.WriteHeader(http.StatusNotFound) }

// hookTarget reads the connection or the platform kind from the path: …/hooks/{id} or
// …/hooks/platform/{kind}.
func hookTarget(path string) (id string, platform string, ok bool) {
	i := strings.LastIndex(path, "/hooks/")
	if i < 0 {
		return "", "", false
	}
	rest := strings.Trim(path[i+len("/hooks/"):], "/")
	if kind, found := strings.CutPrefix(rest, "platform/"); found {
		return "", kind, kind != "" && !strings.Contains(kind, "/")
	}
	return rest, "", rest != "" && !strings.Contains(rest, "/")
}

func (h *inboundHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	id, kind, ok := hookTarget(r.URL.Path)
	if !ok || h.cfg.Connectors == nil {
		notFound(w)
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, h.cfg.MaxBody+1))
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	if int64(len(body)) > h.cfg.MaxBody {
		w.WriteHeader(http.StatusRequestEntityTooLarge)
		return
	}
	ctx := r.Context()
	var (
		conn     connectors.Connection
		creds    connectors.Credentials
		connID   string
		tenantOf uuid.UUID
	)
	if kind != "" {
		if h.cfg.Platform == nil {
			notFound(w)
			return
		}
		pc, _, has := h.cfg.Platform.Platform(catalog.ConnectorKind(kind))
		if !has {
			notFound(w)
			return
		}
		conn = connectors.Connection{Kind: catalog.ConnectorKind(kind), Name: "platform", Status: connectors.StatusConnected}
		creds, connID = pc, catalog.PlatformConnectionID(catalog.ConnectorKind(kind))
	} else {
		cid, err := uuid.Parse(id)
		if err != nil || h.cfg.DB == nil || h.cfg.Connections == nil {
			notFound(w)
			return
		}
		err = h.cfg.DB.QueryRow(ctx, `SELECT tenant_id FROM automation.connections WHERE id = $1`, cid).Scan(&tenantOf)
		if errors.Is(err, pgx.ErrNoRows) {
			notFound(w)
			return
		}
		if err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		if h.cfg.TenantOf != nil {
			if host, ok := h.cfg.TenantOf(r); ok && host != tenantOf {
				notFound(w)
				return
			}
		}
		conn, creds, err = h.cfg.Connections.Get(ctx, tenantOf, cid)
		if errors.Is(err, connectors.ErrNotFound) || (err == nil && conn.Status != connectors.StatusConnected) {
			notFound(w)
			return
		}
		if err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		connID = cid.String()
	}
	cn, found := h.cfg.Connectors.Get(conn.Kind)
	recv, isReceiver := cn.(WebhookReceiver)
	if !found || !isReceiver {
		notFound(w)
		return
	}
	if err := recv.Verify(r, body, conn, creds); err != nil {
		notFound(w)
		return
	}
	buf := &bufferedResponse{header: http.Header{}}
	evs, err := recv.Handle(ctx, buf, body, conn, creds)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	closed, err := h.publish(ctx, evs, connID, tenantOf, kind != "")
	if err != nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}
	if ca, ok := recv.(ClosedAnswerer); ok {
		for _, ev := range closed {
			ca.AnswerClosed(ctx, ev, conn, creds)
		}
	}
	buf.flush(w)
}

// publish writes the events of one request in one transaction; an event of another tenant than the
// connection's is dropped. closed are the events whose correlation closed meanwhile.
func (h *inboundHandler) publish(ctx context.Context, evs []InboundEvent, connID string, tenant uuid.UUID, platform bool) ([]InboundEvent, error) {
	if len(evs) == 0 {
		return nil, nil
	}
	if h.cfg.DB == nil || h.cfg.Inbound == nil {
		return nil, errors.New("automation inbound: no database")
	}
	var closed []InboundEvent
	err := pgx.BeginFunc(ctx, h.cfg.DB, func(tx pgx.Tx) error {
		closed = closed[:0]
		for _, ev := range evs {
			ev.ConnectionID = connID
			switch {
			case platform && ev.TenantID == uuid.Nil:
				continue // the connector found no tenant (an unknown correlation)
			case platform:
				if src, ok := h.cfg.Connections.(connectors.CredentialSource); ok {
					if _, _, err := src.Open(ctx, ev.TenantID, connID); err != nil {
						continue // the platform credential is not the tenant's
					}
				}
			case ev.TenantID == uuid.Nil:
				ev.TenantID = tenant
			case ev.TenantID != tenant:
				continue
			}
			sp, err := tx.Begin(ctx)
			if err != nil {
				return err
			}
			_, err = h.cfg.Inbound.Publish(ctx, sp, ev)
			switch {
			case errors.Is(err, ErrCorrelationClosed):
				_ = sp.Rollback(ctx)
				closed = append(closed, ev)
				continue
			case errors.Is(err, ErrInboundInvalid):
				_ = sp.Rollback(ctx)
				continue
			case err != nil:
				_ = sp.Rollback(ctx)
				return err
			}
			if err := sp.Commit(ctx); err != nil {
				return err
			}
		}
		return nil
	})
	return closed, err
}

// bufferedResponse holds what a connector answers the provider until the events are committed.
type bufferedResponse struct {
	header http.Header
	status int
	body   bytes.Buffer
}

func (b *bufferedResponse) Header() http.Header         { return b.header }
func (b *bufferedResponse) Write(p []byte) (int, error) { return b.body.Write(p) }
func (b *bufferedResponse) WriteHeader(status int)      { b.status = status }

func (b *bufferedResponse) flush(w http.ResponseWriter) {
	for k, v := range b.header {
		w.Header()[k] = v
	}
	status := b.status
	if status == 0 {
		status = http.StatusOK
	}
	w.WriteHeader(status)
	_, _ = w.Write(b.body.Bytes())
}
