package engine_test

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/pkg/automation/catalog"
	"github.com/iota-uz/iota-sdk/pkg/automation/connectors"
	"github.com/iota-uz/iota-sdk/pkg/automation/engine"
	"github.com/iota-uz/iota-sdk/pkg/automation/graph"
)

// messageGraph notes every inbound message; trigger is the trigger's JSON.
func messageGraph(trigger string) string {
	return graphJSON("Inbound", trigger,
		`{"id": "note", "type": "acme.note", "params": {"text": "{{ event.text }}|{{ events[0].text }}|{{ string(size(events)) }}|{{ string(event.by_self) }}"}},
		 {"id": "fin", "type": "end", "params": {}}`,
		`{"from": {"node": "m", "outcome": "fired"}, "to": "note"},
		 {"from": {"node": "note", "outcome": "done"}, "to": "fin"}, {"from": {"node": "note", "outcome": "failed"}, "to": "fin"}`)
}

func noteTexts(t *testing.T, e *env) []string {
	t.Helper()
	rows, err := e.db.Query(context.Background(), `SELECT text FROM test_effects ORDER BY text`)
	require.NoError(t, err)
	out, err := pgx.CollectRows(rows, pgx.RowTo[string])
	require.NoError(t, err)
	return out
}

// A webhook with a wrong signature and one for the connection of another tenant look the same: 404 and
// no body. A body over the limit is 413; an unknown connection 404.
func TestWebhookRefusalsAreIndistinguishable(t *testing.T) {
	ae := newAcmeEnv(t)
	p := ae.proc()
	other := ae.newTenant()
	h := ae.handler(p, func(r *http.Request) (uuid.UUID, bool) {
		if r.Header.Get("X-Tenant") == "other" {
			return other, true
		}
		return ae.tenant, true
	})
	bad := ae.hook(h, ae.hookPath(), "wrong", acmeUpdate{ID: "1", Text: "x"})
	req := httptest.NewRequest(http.MethodPost, ae.hookPath(), strings.NewReader(`{"id":"2","text":"x"}`))
	req.Header.Set("X-Sig", acmeSig)
	req.Header.Set("X-Tenant", "other")
	foreign := httptest.NewRecorder()
	h.ServeHTTP(foreign, req)
	unknown := ae.hook(h, "/automations/hooks/"+uuid.NewString(), acmeSig, acmeUpdate{ID: "3"})
	for _, w := range []*httptest.ResponseRecorder{bad, foreign, unknown} {
		assert.Equal(t, http.StatusNotFound, w.Code)
		assert.Empty(t, w.Body.String())
		assert.Equal(t, bad.Header(), w.Header())
	}
	big := httptest.NewRequest(http.MethodPost, ae.hookPath(), strings.NewReader(strings.Repeat("x", 2<<20)))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, big)
	assert.Equal(t, http.StatusRequestEntityTooLarge, w.Code)
	assert.Zero(t, ae.count(`SELECT count(*) FROM automation.inbound_dedup`))
}

// A provider that repeats an update starts no second run; the provider gets 200 both times.
func TestRepeatedInboundMessageStartsOneRun(t *testing.T) {
	ae := newAcmeEnv(t)
	p := ae.proc()
	ae.create(p, ae.human(), messageGraph(`{"id": "m", "event": "acme.message.v1"}`))
	h := ae.handler(p, nil)
	first := ae.hook(h, ae.hookPath(), acmeSig, acmeUpdate{ID: "u-1", Text: "hello", From: "ann"})
	second := ae.hook(h, ae.hookPath(), acmeSig, acmeUpdate{ID: "u-1", Text: "hello", From: "ann"})
	assert.Equal(t, http.StatusOK, first.Code)
	assert.Equal(t, `{"ok":true}`, first.Body.String(), "the connector's own answer after the commit")
	assert.Equal(t, http.StatusOK, second.Code)
	p.until(func() bool { return len(noteTexts(t, ae.env)) == 1 }, "the message ran the graph")
	for i := 0; i < 5; i++ {
		p.pass()
	}
	assert.Equal(t, []string{"hello|hello|1|false"}, noteTexts(t, ae.env))
	assert.Equal(t, 1, ae.count(`SELECT count(*) FROM extruntime.outbox_events WHERE event = $1`, evMessage))
	assert.Equal(t, 1, ae.count(`SELECT count(*) FROM automation.runs`))
}

// A message the graph's own connector sent (by_self) does not start it; include_self lets it.
func TestOwnMessagesDoNotStartTheGraph(t *testing.T) {
	ae := newAcmeEnv(t)
	p := ae.proc()
	ae.create(p, ae.human(), messageGraph(`{"id": "m", "event": "acme.message.v1"}`))
	h := ae.handler(p, nil)
	ae.hook(h, ae.hookPath(), acmeSig, acmeUpdate{ID: "self-1", Text: "echo", From: "bot", Self: true})
	ae.hook(h, ae.hookPath(), acmeSig, acmeUpdate{ID: "u-1", Text: "hi", From: "ann"})
	p.until(func() bool { return len(noteTexts(t, ae.env)) == 1 }, "the person's message ran")
	for i := 0; i < 3; i++ {
		p.pass()
	}
	assert.Equal(t, []string{"hi|hi|1|false"}, noteTexts(t, ae.env))

	ae.create(p, ae.human(), strings.Replace(messageGraph(`{"id": "m", "event": "acme.message.v1", "include_self": true}`), `"Inbound"`, `"Echo"`, 1))
	ae.hook(h, ae.hookPath(), acmeSig, acmeUpdate{ID: "self-2", Text: "echo", From: "bot", Self: true})
	p.until(func() bool { return len(noteTexts(t, ae.env)) == 2 }, "include_self starts on its own message")
	assert.Equal(t, []string{"echo|echo|1|true", "hi|hi|1|false"}, noteTexts(t, ae.env))
}

// A record a node of the graph created does not start the same graph again.
func TestOwnEventsDoNotStartTheGraphAgain(t *testing.T) {
	ae := newAcmeEnv(t)
	p := ae.proc()
	ae.create(p, ae.human(), graphJSON("Emitter", `{"id": "created", "event": "acme.order.created.v1"}`,
		`{"id": "emit", "type": "acme.emit", "params": {}}, {"id": "fin", "type": "end", "params": {}}`,
		`{"from": {"node": "created", "outcome": "fired"}, "to": "emit"}, {"from": {"node": "emit", "outcome": "done"}, "to": "fin"}`))
	ae.publish(ae.tenant, orderCreated, "o-1", order{}, 0)
	p.until(func() bool {
		return ae.count(`SELECT count(*) FROM extruntime.outbox_events WHERE event = $1`, evCreated) == 2
	}, "the run published its own order event")
	p.until(func() bool {
		return ae.count(`SELECT count(*) FROM automation.runs r JOIN extruntime.workflow_instances i ON i.id = r.instance_id WHERE i.status = 'completed'`) == 1
	}, "done")
	for i := 0; i < 5; i++ {
		p.pass()
	}
	assert.Equal(t, 1, ae.count(`SELECT count(*) FROM automation.runs`), "the graph's own event starts nothing")
	var cause []byte
	require.NoError(t, ae.db.QueryRow(context.Background(), `SELECT cause FROM extruntime.outbox_events WHERE event = $1 AND cause IS NOT NULL`, evCreated).Scan(&cause))
	assert.Contains(t, string(cause), "automation_id", "the event carries its Origin")
}

// Four messages of one sender within 30s and a 60s window start one run with the four events.
func TestCollectMergesMessagesIntoOneRun(t *testing.T) {
	ae := newAcmeEnv(t)
	p := ae.proc()
	ae.create(p, ae.human(), messageGraph(`{"id": "m", "event": "acme.message.v1", "collect": {"key": "event.from", "window": "60s"}}`))
	in := p.rt.Inbound()
	base := time.Now().Add(-100 * time.Second) // the window closed already: the next pass starts the run
	ctx := context.Background()
	for i, text := range []string{"m1", "m2", "m3", "m4"} {
		u := acmeUpdate{ID: "c-" + text, Event: evMessage, Text: text, From: "ann"}
		ev := u.event(ae.conn.ID.String())
		ev.TenantID, ev.ConnectionID, ev.OccurredAt = ae.tenant, ae.conn.ID.String(), base.Add(time.Duration(i)*10*time.Second)
		require.NoError(t, pgx.BeginFunc(ctx, ae.db, func(tx pgx.Tx) error {
			ok, err := in.Publish(ctx, tx, ev)
			require.True(t, ok)
			return err
		}))
	}
	// Another sender has a window of its own, still open.
	other := acmeUpdate{ID: "c-bob", Event: evMessage, Text: "b1", From: "bob"}.event(ae.conn.ID.String())
	other.TenantID, other.ConnectionID = ae.tenant, ae.conn.ID.String()
	require.NoError(t, pgx.BeginFunc(ctx, ae.db, func(tx pgx.Tx) error { _, err := in.Publish(ctx, tx, other); return err }))

	p.until(func() bool { return len(noteTexts(t, ae.env)) == 1 }, "the window closed into one run")
	assert.Equal(t, []string{"m4|m1|4|false"}, noteTexts(t, ae.env), "event is the last, events all four")
	assert.Equal(t, 1, ae.count(`SELECT count(*) FROM automation.runs`))
	assert.Equal(t, 1, ae.count(`SELECT count(*) FROM automation.inbound_collect`), "bob's window stays open")
	var evs []json.RawMessage
	var raw []byte
	require.NoError(t, ae.db.QueryRow(ctx, `SELECT events FROM automation.runs`).Scan(&raw))
	require.NoError(t, json.Unmarshal(raw, &evs))
	assert.Len(t, evs, 4)
}

// collect is only for inbound triggers.
func TestCollectNeedsAnInboundTrigger(t *testing.T) {
	ae := newAcmeEnv(t)
	p := ae.proc()
	_, _, err := p.rt.Service().Create(context.Background(), ae.human(), engine.CreateInput{Graph: json.RawMessage(
		strings.Replace(sendGraph("acme.send", "ok", "", true), `"event": "acme.order.created.v1"`, `"event": "acme.order.created.v1", "collect": {"key": "event.note", "window": "1m"}`, 1))})
	var ve *engine.ValidationError
	require.ErrorAs(t, err, &ve)
	assert.Equal(t, "triggers[0].collect", ve.Errors[0].Path)
}

// askGraph asks and notes the answer.
const askGraph = `{"name": "Ask", "triggers": [{"id": "created", "event": "acme.order.created.v1"}],
 "nodes": [
  {"id": "ask", "type": "acme.ask", "params": {"text": "approve?"}},
  {"id": "yes", "type": "acme.note", "params": {"text": "answer {{ nodes.ask.output.choice }}"}},
  {"id": "no", "type": "acme.note", "params": {"text": "no"}},
  {"id": "late", "type": "acme.note", "params": {"text": "late"}},
  {"id": "bad", "type": "acme.note", "params": {"text": "bad"}},
  {"id": "fin", "type": "end", "params": {}}
 ],
 "edges": [
  {"from": {"node": "created", "outcome": "fired"}, "to": "ask"},
  {"from": {"node": "ask", "outcome": "yes"}, "to": "yes"},
  {"from": {"node": "ask", "outcome": "no"}, "to": "no"},
  {"from": {"node": "ask", "outcome": "timeout"}, "to": "late"},
  {"from": {"node": "ask", "outcome": "failed"}, "to": "bad"},
  {"from": {"node": "yes", "outcome": "done"}, "to": "fin"}, {"from": {"node": "yes", "outcome": "failed"}, "to": "fin"},
  {"from": {"node": "no", "outcome": "done"}, "to": "fin"}, {"from": {"node": "no", "outcome": "failed"}, "to": "fin"},
  {"from": {"node": "late", "outcome": "done"}, "to": "fin"}, {"from": {"node": "late", "outcome": "failed"}, "to": "fin"},
  {"from": {"node": "bad", "outcome": "done"}, "to": "fin"}, {"from": {"node": "bad", "outcome": "failed"}, "to": "fin"}
 ]}`

// Two presses of the ask's button give one transition; the second is answered "already answered".
func TestTwoAnswersOfAnAskGiveOneTransition(t *testing.T) {
	ae := newAcmeEnv(t)
	p := ae.proc()
	ae.create(p, ae.human(), askGraph)
	ae.publish(ae.tenant, orderCreated, "o-1", order{}, 0)
	p.until(func() bool {
		return ae.count(`SELECT count(*) FROM automation.waits WHERE status = 'open' AND correlation IS NOT NULL`) == 1
	}, "the answer wait opens")
	calls := ae.acme.callsOf("acme.ask")
	require.Len(t, calls, 1)
	token := calls[0].Token
	require.Len(t, token, engine.CorrelationTokenLen)
	tenant, ok, err := p.rt.Inbound().Correlation(context.Background(), token)
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, ae.tenant, tenant)

	h := ae.handler(p, nil)
	first := ae.hook(h, ae.hookPath(), acmeSig, acmeUpdate{ID: "a-1", Event: evAnswer, Choice: "yes", Correlation: token, From: "ann"})
	second := ae.hook(h, ae.hookPath(), acmeSig, acmeUpdate{ID: "a-2", Event: evAnswer, Choice: "no", Correlation: token, From: "ann"})
	assert.Equal(t, http.StatusOK, first.Code)
	assert.Equal(t, http.StatusOK, second.Code)
	assert.Equal(t, int64(1), ae.acme.closed.Load(), "the second press is answered as already answered")
	p.until(func() bool { return len(noteTexts(t, ae.env)) == 1 }, "the answer moved the run")
	run := runOf(t, ae.env, p)
	p.until(func() bool { return ae.status(p, run.RunID) == engine.RunDone }, "done")
	assert.Equal(t, []string{"answer yes"}, noteTexts(t, ae.env))
	_, ok, err = p.rt.Inbound().Correlation(context.Background(), token)
	require.NoError(t, err)
	assert.False(t, ok, "the token closed with the wait")
	// A late answer is refused as closed.
	err = pgx.BeginFunc(context.Background(), ae.db, func(tx pgx.Tx) error {
		ev := acmeUpdate{ID: "a-3", Event: evAnswer, Choice: "no", Correlation: token, From: "ann"}.event(ae.conn.ID.String())
		ev.TenantID, ev.ConnectionID = ae.tenant, ae.conn.ID.String()
		_, err := p.rt.Inbound().Publish(context.Background(), tx, ev)
		return err
	})
	require.ErrorIs(t, err, engine.ErrCorrelationClosed)
}

// An answer that arrives before the parent opened its wait is found by the wait's catch-up.
func TestAnswerBeforeTheWaitOpensIsCaught(t *testing.T) {
	ae := newAcmeEnv(t)
	p := ae.proc()
	ae.create(p, ae.human(), askGraph)
	ae.publish(ae.tenant, orderCreated, "o-1", order{}, 0)
	p.until(func() bool { return len(ae.acme.callsOf("acme.ask")) == 1 }, "the question is sent")
	token := ae.acme.callsOf("acme.ask")[0].Token
	h := ae.handler(p, nil)
	ae.hook(h, ae.hookPath(), acmeSig, acmeUpdate{ID: "a-1", Event: evAnswer, Choice: "yes", Correlation: token, From: "ann"})
	p.until(func() bool { return len(noteTexts(t, ae.env)) == 1 }, "the early answer moved the run")
	assert.Equal(t, []string{"answer yes"}, noteTexts(t, ae.env))
}

// The poller polls the connection an enabled graph uses, moves its cursor after the events are
// committed and stops on an authentication failure.
func TestPollerPublishesAndStopsOnReauth(t *testing.T) {
	ae := newAcmeEnv(t)
	p := ae.proc()
	ae.create(p, ae.human(), messageGraph(`{"id": "m", "event": "acme.message.v1"}`))
	conn := ae.conn.ID.String()
	ae.acme.pollQ = [][]engine.InboundEvent{
		{acmeUpdate{ID: "p-1", Event: evMessage, Text: "polled", From: "ann"}.event(conn)},
		{acmeUpdate{ID: "p-1", Event: evMessage, Text: "polled", From: "ann"}.event(conn)}, // the provider repeats it
	}
	now := time.Now()
	poller := p.rt.NewPoller(engine.PollerConfig{Connections: ae.store, Connectors: ae.dir, Clock: func() time.Time { return now }})
	ctx := context.Background()
	next, err := poller.PollOnce(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(1), ae.acme.polls.Load())
	assert.WithinDuration(t, now.Add(engine.MinPollInterval), next, time.Second)
	_, err = poller.PollOnce(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(1), ae.acme.polls.Load(), "not due yet")
	now = now.Add(engine.MinPollInterval)
	_, err = poller.PollOnce(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(2), ae.acme.polls.Load())
	p.until(func() bool { return len(noteTexts(t, ae.env)) == 1 }, "the polled message ran the graph")
	assert.Equal(t, 1, ae.count(`SELECT count(*) FROM extruntime.outbox_events WHERE event = $1`, evMessage), "the repeat is dropped")

	ae.acme.pollErr = &catalog.CallError{Code: catalog.CallUnauthorized, Message: "token revoked"}
	now = now.Add(engine.MinPollInterval)
	_, err = poller.PollOnce(ctx)
	require.NoError(t, err)
	c, _, err := ae.store.Get(ctx, ae.tenant, ae.conn.ID)
	require.NoError(t, err)
	assert.Equal(t, connectors.StatusNeedsReauth, c.Status)
}

// An idle engine and poller run no pass between their planned ones.
func TestIdleWorkersDoNotSpin(t *testing.T) {
	ae := newAcmeEnv(t)
	p := ae.proc(func(c *engine.Config) { c.MaintenanceInterval = time.Second })
	poller := p.rt.NewPoller(engine.PollerConfig{Connections: ae.store, Connectors: ae.dir, Rescan: time.Second})
	ctx, cancel := context.WithTimeout(context.Background(), 3500*time.Millisecond)
	defer cancel()
	done := make(chan struct{}, 2)
	go func() { p.rt.Serve(ctx); done <- struct{}{} }()
	go func() { poller.Serve(ctx); done <- struct{}{} }()
	<-done
	<-done
	assert.LessOrEqual(t, p.rt.Passes(), int64(4), "a pass at start and one per second")
	assert.LessOrEqual(t, poller.Iterations(), 4)
	assert.GreaterOrEqual(t, p.rt.Passes(), int64(3))
}

func TestInboundRejectsWhatItCannotPublish(t *testing.T) {
	ae := newAcmeEnv(t)
	p := ae.proc()
	in := p.rt.Inbound()
	ctx := context.Background()
	cases := []engine.InboundEvent{
		{TenantID: ae.tenant, ConnectionID: "x", Event: "acme.unknown.v1", DedupKey: "1", Subject: graph.Subject{Entity: graph.SubjectConversation, Key: "k"}},
		{TenantID: ae.tenant, ConnectionID: "x", Event: evMessage, DedupKey: "2", Payload: json.RawMessage(`{"text": 1}`), Subject: graph.Subject{Entity: graph.SubjectConversation, Key: "k"}},
		{TenantID: ae.tenant, ConnectionID: "x", Event: evMessage, DedupKey: "3", Payload: json.RawMessage(`{"text": "a", "from": "b"}`), Subject: graph.Subject{Entity: "acme.order", Key: "k"}},
		{TenantID: ae.tenant, ConnectionID: "x", Event: evMessage, Payload: json.RawMessage(`{}`)},
	}
	for i, ev := range cases {
		err := pgx.BeginFunc(ctx, ae.db, func(tx pgx.Tx) error { _, err := in.Publish(ctx, tx, ev); return err })
		assert.ErrorIs(t, err, engine.ErrInboundInvalid, "case %d", i)
	}
}

// A window whose deadline passed before the next event (its flush was late) is closed into its own
// run; the later event opens a new window.
func TestCollectClosesAnExpiredWindowBeforeALaterEvent(t *testing.T) {
	ae := newAcmeEnv(t)
	p := ae.proc()
	ae.create(p, ae.human(), messageGraph(`{"id": "m", "event": "acme.message.v1", "collect": {"key": "event.from", "window": "60s"}}`))
	in := p.rt.Inbound()
	ctx := context.Background()
	base := time.Now().Add(-10 * time.Minute)
	for i, text := range []string{"m1", "m2"} {
		ev := acmeUpdate{ID: "x-" + text, Event: evMessage, Text: text, From: "ann"}.event(ae.conn.ID.String())
		ev.TenantID, ev.ConnectionID, ev.OccurredAt = ae.tenant, ae.conn.ID.String(), base.Add(time.Duration(i)*5*time.Minute)
		require.NoError(t, pgx.BeginFunc(ctx, ae.db, func(tx pgx.Tx) error { _, err := in.Publish(ctx, tx, ev); return err }))
	}
	// Both events are delivered before any flush runs.
	_, err := p.disp.RunOnce(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, ae.count(`SELECT count(*) FROM automation.runs`), "m1's window closed when m2 came")
	assert.Equal(t, 1, ae.count(`SELECT count(*) FROM automation.inbound_collect WHERE jsonb_array_length(events) = 1`))
	p.until(func() bool { return len(noteTexts(t, ae.env)) == 2 }, "both windows ran")
	assert.Equal(t, []string{"m1|m1|1|false", "m2|m2|1|false"}, noteTexts(t, ae.env))
}

// failingOpen is a CredentialSource whose platform lookup fails.
type failingOpen struct {
	connectors.ConnectionStore
	err error
}

func (f failingOpen) Open(context.Context, uuid.UUID, string) (catalog.ConnectionInfo, connectors.Credentials, error) {
	return catalog.ConnectionInfo{}, nil, f.err
}

// platformAcme is a platform credential of acme.
type platformAcme struct{}

func (platformAcme) Platform(catalog.ConnectorKind) (connectors.Credentials, connectors.Identity, bool) {
	return connectors.Credentials{"secret": acmeSig}, connectors.Identity{}, true
}

// A platform webhook whose tenant lookup fails gets 5xx (the provider repeats it); a tenant without the
// platform credential is skipped with 200.
func TestPlatformWebhookFailsOnAStorageError(t *testing.T) {
	ae := newAcmeEnv(t)
	p := ae.proc()
	acme := &tenantAcme{acme: ae.acme, tenant: ae.tenant}
	dir := connectors.NewDirectory(nil)
	require.NoError(t, dir.Register(acme))
	serve := func(openErr error) int {
		h := engine.InboundHandler(engine.InboundConfig{Connections: failingOpen{ConnectionStore: ae.store, err: openErr}, Registry: ae.reg,
			Inbound: p.rt.Inbound(), DB: ae.db, Connectors: dir, Platform: platformAcme{}})
		return ae.hook(h, "/automations/hooks/platform/acme", acmeSig, acmeUpdate{ID: uuid.NewString(), Text: "x", From: "ann"}).Code
	}
	assert.Equal(t, http.StatusServiceUnavailable, serve(errors.New("connection refused")))
	assert.Equal(t, http.StatusOK, serve(connectors.ErrNotFound))
	assert.Zero(t, ae.count(`SELECT count(*) FROM extruntime.outbox_events WHERE event = $1`, evMessage))
}

// tenantAcme is acme whose platform updates name the tenant.
type tenantAcme struct {
	*acme
	tenant uuid.UUID
}

func (a *tenantAcme) Handle(ctx context.Context, w http.ResponseWriter, body []byte, c connectors.Connection, creds connectors.Credentials) ([]engine.InboundEvent, error) {
	evs, err := a.acme.Handle(ctx, w, body, c, creds)
	for i := range evs {
		evs[i].TenantID = a.tenant
	}
	return evs, err
}

// A failed poll is logged with the credentials masked.
func TestPollerMasksTheProviderError(t *testing.T) {
	ae := newAcmeEnv(t)
	var buf syncBuffer
	p := ae.proc(func(c *engine.Config) { c.Logger = slog.New(slog.NewTextHandler(&buf, nil)) })
	ae.create(p, ae.human(), messageGraph(`{"id": "m", "event": "acme.message.v1"}`))
	ae.acme.pollErr = errors.New("GET https://api.acme.test/updates?token=" + acmeToken + ": timeout")
	poller := p.rt.NewPoller(engine.PollerConfig{Connections: ae.store, Connectors: ae.dir})
	_, err := poller.PollOnce(context.Background())
	require.NoError(t, err)
	out := buf.String()
	assert.Contains(t, out, "token=***")
	assert.NotContains(t, out, acmeToken)
}

type syncBuffer struct {
	mu sync.Mutex
	b  strings.Builder
}

func (s *syncBuffer) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.Write(p)
}

func (s *syncBuffer) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.String()
}
