package engine_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/pkg/automation/catalog"
	"github.com/iota-uz/iota-sdk/pkg/automation/connectors"
	"github.com/iota-uz/iota-sdk/pkg/automation/engine"
	"github.com/iota-uz/iota-sdk/pkg/automation/expr"
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
	// A message of another key that carries the token does not take it.
	other := ae.hook(h, ae.hookPath(), acmeSig, acmeUpdate{ID: "o-1", Event: evMessage, Text: "hi", From: "ann", Correlation: token})
	assert.Equal(t, http.StatusOK, other.Code)
	assert.Zero(t, ae.acme.closed.Load())
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

// One poller at a time polls a connection: another one (of any process) skips it while the claim lives,
// and a poll that outlived its claim saves nothing over the newer owner's state.
func TestPollerClaimsTheConnection(t *testing.T) {
	ae := newAcmeEnv(t)
	p := ae.proc()
	ae.create(p, ae.human(), messageGraph(`{"id": "m", "event": "acme.message.v1"}`))
	ctx := context.Background()
	now := time.Now()
	clock := func(at *time.Time) func() time.Time { return func() time.Time { return *at } }
	a := p.rt.NewPoller(engine.PollerConfig{Connections: ae.store, Connectors: ae.dir, Clock: clock(&now)})
	bNow := now
	b := p.rt.NewPoller(engine.PollerConfig{Connections: ae.store, Connectors: ae.dir, Clock: clock(&bNow)})
	inner := make(chan time.Time, 2)
	ae.acme.onPoll = func() {
		// A polls: B, as due as A, skips the connection A holds.
		next, err := b.PollOnce(ctx)
		require.NoError(t, err)
		inner <- next
	}
	_, err := a.PollOnce(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(1), ae.acme.polls.Load(), "B did not poll while A held the connection")
	assert.True(t, (<-inner).After(now), "B looks again after A's claim")

	// A's next poll outlives its claim: B takes the connection over and A's late save is dropped.
	now = now.Add(engine.MinPollInterval)
	bNow = now.Add(engine.MinPollInterval + 10*time.Minute)
	ae.acme.onPoll = func() {
		_, err := b.PollOnce(ctx)
		require.NoError(t, err)
	}
	_, err = a.PollOnce(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(3), ae.acme.polls.Load(), "B polled after A's claim ran out")
	var raw []byte
	require.NoError(t, ae.db.QueryRow(ctx, `SELECT inbound FROM automation.connections WHERE tenant_id = $1 AND id = $2`, ae.tenant, ae.conn.ID).Scan(&raw))
	var st struct {
		Poll struct {
			NextAt time.Time `json:"next_at"`
			Owner  string    `json:"owner"`
		} `json:"poll"`
	}
	require.NoError(t, json.Unmarshal(raw, &st))
	assert.Empty(t, st.Poll.Owner, "B released its claim")
	assert.WithinDuration(t, bNow.Add(engine.MinPollInterval), st.Poll.NextAt, time.Second, "B's state stands, not A's")
}

// A poll that outlived its claim and then got 401 leaves the connection connected: the newer owner's
// poll succeeded with the credentials, and only an owner sets needs_reauth.
func TestStalePollDoesNotSetNeedsReauth(t *testing.T) {
	ae := newAcmeEnv(t)
	p := ae.proc()
	ae.create(p, ae.human(), messageGraph(`{"id": "m", "event": "acme.message.v1"}`))
	ctx := context.Background()
	now := time.Now()
	a := p.rt.NewPoller(engine.PollerConfig{Connections: ae.store, Connectors: ae.dir, Clock: func() time.Time { return now }})
	later := now.Add(engine.MinPollInterval + 10*time.Minute)
	b := p.rt.NewPoller(engine.PollerConfig{Connections: ae.store, Connectors: ae.dir, Clock: func() time.Time { return later }})
	ae.acme.onPoll = func() {
		// A's claim runs out: B takes the connection over and polls it; then A's provider refuses A.
		_, err := b.PollOnce(ctx)
		require.NoError(t, err)
		ae.acme.mu.Lock()
		ae.acme.pollErr = &catalog.CallError{Code: catalog.CallUnauthorized, Message: "stale token"}
		ae.acme.mu.Unlock()
	}
	_, err := a.PollOnce(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(2), ae.acme.polls.Load())
	c, _, err := ae.store.Get(ctx, ae.tenant, ae.conn.ID)
	require.NoError(t, err)
	assert.Equal(t, connectors.StatusConnected, c.Status, "the stale poll did not stop polling")
}

// A poll that lost its claim before its batch committed publishes nothing: the new owner's events stand,
// and an event only the stale poll returned starts no run.
func TestStalePollPublishesNothing(t *testing.T) {
	ae := newAcmeEnv(t)
	p := ae.proc()
	ae.create(p, ae.human(), messageGraph(`{"id": "m", "event": "acme.message.v1"}`))
	ctx := context.Background()
	conn := ae.conn.ID.String()
	ae.acme.pollQ = [][]engine.InboundEvent{{acmeUpdate{ID: "b-1", Event: evMessage, Text: "fresh", From: "ann"}.event(conn)}}
	now := time.Now()
	a := p.rt.NewPoller(engine.PollerConfig{Connections: ae.store, Connectors: ae.dir, Clock: func() time.Time { return now }})
	later := now.Add(engine.MinPollInterval + 10*time.Minute)
	b := p.rt.NewPoller(engine.PollerConfig{Connections: ae.store, Connectors: ae.dir, Clock: func() time.Time { return later }})
	ae.acme.onPoll = func() {
		// A's claim runs out: B takes the connection over and publishes its batch; A's provider then
		// answers A with an event B never saw.
		_, err := b.PollOnce(ctx)
		require.NoError(t, err)
		ae.acme.mu.Lock()
		ae.acme.pollQ = [][]engine.InboundEvent{{acmeUpdate{ID: "a-1", Event: evMessage, Text: "stale", From: "ann"}.event(conn)}}
		ae.acme.mu.Unlock()
	}
	_, err := a.PollOnce(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(2), ae.acme.polls.Load())
	assert.Equal(t, 1, ae.count(`SELECT count(*) FROM extruntime.outbox_events WHERE event = $1`, evMessage), "only B's event")
	p.until(func() bool { return len(noteTexts(t, ae.env)) == 1 }, "B's event ran")
	assert.Equal(t, []string{"fresh|fresh|1|false"}, noteTexts(t, ae.env))
}

// A poll whose credentials a reauthorization replaced while it ran, and that then got 401, leaves the
// connection connected: no other poller took it over, but the refused credentials are no longer its.
func TestPollAfterReauthorizationDoesNotSetNeedsReauth(t *testing.T) {
	ae := newAcmeEnv(t)
	p := ae.proc()
	ae.create(p, ae.human(), messageGraph(`{"id": "m", "event": "acme.message.v1"}`))
	ctx := context.Background()
	now := time.Now()
	poller := p.rt.NewPoller(engine.PollerConfig{Connections: ae.store, Connectors: ae.dir, Clock: func() time.Time { return now }})
	ae.acme.onPoll = func() {
		// The user reauthorizes the connection meanwhile; the provider then refuses the old request.
		c, _, err := ae.store.Get(ctx, ae.tenant, ae.conn.ID)
		require.NoError(t, err)
		_, err = ae.store.Save(ctx, c, connectors.Credentials{"token": acmeToken, "secret": acmeSig})
		require.NoError(t, err)
		ae.acme.mu.Lock()
		ae.acme.pollErr = &catalog.CallError{Code: catalog.CallUnauthorized, Message: "old token"}
		ae.acme.mu.Unlock()
	}
	_, err := poller.PollOnce(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(1), ae.acme.polls.Load(), "no other poll ran")
	c, _, err := ae.store.Get(ctx, ae.tenant, ae.conn.ID)
	require.NoError(t, err)
	assert.Equal(t, connectors.StatusConnected, c.Status, "the reauthorized connection keeps polling")
	var owner string
	require.NoError(t, ae.db.QueryRow(ctx, `SELECT coalesce(inbound->'poll'->>'owner', '') FROM automation.connections WHERE tenant_id = $1 AND id = $2`,
		ae.tenant, ae.conn.ID).Scan(&owner))
	assert.Empty(t, owner, "the claim is released")
}

// A poll whose claim waited for the connection's row lock past the interval is due an interval after
// it got the lock, not after it began: the next poll does not follow it at once.
func TestPollerIntervalCountsFromTheClaim(t *testing.T) {
	ae := newAcmeEnv(t)
	p := ae.proc()
	ae.create(p, ae.human(), messageGraph(`{"id": "m", "event": "acme.message.v1"}`))
	ctx := context.Background()
	var clock atomic.Int64
	start := time.Now()
	clock.Store(start.UnixNano())
	now := func() time.Time { return time.Unix(0, clock.Load()) }
	poller := p.rt.NewPoller(engine.PollerConfig{Connections: ae.store, Connectors: ae.dir, Clock: now})

	// Another transaction holds the connection's row while the poller begins.
	lock, err := ae.db.Begin(ctx)
	require.NoError(t, err)
	defer func() { _ = lock.Rollback(ctx) }()
	_, err = lock.Exec(ctx, `SELECT 1 FROM automation.connections WHERE tenant_id = $1 AND id = $2 FOR UPDATE`, ae.tenant, ae.conn.ID)
	require.NoError(t, err)
	pollCtx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()
	done := make(chan error, 1)
	go func() { _, err := poller.PollOnce(pollCtx); done <- err }()
	require.Eventually(t, func() bool {
		return ae.count(`SELECT count(*) FROM pg_stat_activity WHERE datname = current_database() AND wait_event_type = 'Lock'`) > 0
	}, 20*time.Second, 20*time.Millisecond, "the claim waits for the row lock")
	late := start.Add(3 * engine.MinPollInterval)
	clock.Store(late.UnixNano())
	require.NoError(t, lock.Commit(ctx))
	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(30 * time.Second):
		cancel()
		t.Fatal("the poll did not finish after the row lock was released")
	}
	require.Equal(t, int64(1), ae.acme.polls.Load())

	var nextAt time.Time
	require.NoError(t, ae.db.QueryRow(ctx, `SELECT (inbound->'poll'->>'next_at')::timestamptz FROM automation.connections WHERE tenant_id = $1 AND id = $2`,
		ae.tenant, ae.conn.ID).Scan(&nextAt))
	assert.WithinDuration(t, late.Add(engine.MinPollInterval), nextAt, time.Second, "due an interval after the claim")
	// Right after it, another poller (another process) finds the connection not due.
	other := p.rt.NewPoller(engine.PollerConfig{Connections: ae.store, Connectors: ae.dir, Clock: func() time.Time { return late.Add(time.Second) }})
	_, err = other.PollOnce(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(1), ae.acme.polls.Load(), "no second poll at once")
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
		{TenantID: ae.tenant, ConnectionID: "x", Event: evMessage, DedupKey: "5", Payload: json.RawMessage(`{"text": null, "from": "b"}`), Subject: graph.Subject{Entity: graph.SubjectConversation, Key: "k"}},
		{TenantID: ae.tenant, ConnectionID: "x", Event: evMessage, DedupKey: "6", Payload: json.RawMessage(`{"from": "b"}`), Subject: graph.Subject{Entity: graph.SubjectConversation, Key: "k"}},
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

// A sweep that read a window as due before an event extended it leaves the window open: the window is
// due (its first event is old), a second event before the old deadline extends it into the future, and
// the sweep of the key it read closes nothing.
func TestStaleSweepLeavesAnExtendedWindow(t *testing.T) {
	ae := newAcmeEnv(t)
	p := ae.proc()
	am := ae.create(p, ae.human(), messageGraph(`{"id": "m", "event": "acme.message.v1", "collect": {"key": "event.from", "window": "60s"}}`))
	ctx := context.Background()
	publishAt := func(id string, at time.Time) {
		ev := acmeUpdate{ID: id, Event: evMessage, Text: id, From: "ann"}.event(ae.conn.ID.String())
		ev.TenantID, ev.ConnectionID, ev.OccurredAt = ae.tenant, ae.conn.ID.String(), at
		require.NoError(t, pgx.BeginFunc(ctx, ae.db, func(tx pgx.Tx) error { _, err := p.rt.Inbound().Publish(ctx, tx, ev); return err }))
		_, err := p.disp.RunOnce(ctx)
		require.NoError(t, err)
	}
	now := time.Now()
	publishAt("s-1", now.Add(-90*time.Second)) // deadline now-30s: due
	var key string
	require.NoError(t, ae.db.QueryRow(ctx, `SELECT collect_key FROM automation.inbound_collect WHERE automation_id = $1 AND deadline <= now()`, am.ID).Scan(&key))
	publishAt("s-2", now.Add(-40*time.Second)) // before the old deadline: joins, deadline now+20s
	require.NoError(t, p.rt.SweepWindow(ctx, ae.tenant, am.ID, "m", key))
	var count int
	require.NoError(t, ae.db.QueryRow(ctx, `SELECT count FROM automation.inbound_collect WHERE automation_id = $1 AND collect_key = $2 AND deadline > now()`, am.ID, key).Scan(&count))
	assert.Equal(t, 2, count, "the same window stays, extended")
	assert.Zero(t, ae.count(`SELECT count(*) FROM automation.runs`), "no run started")
}

// The token takes only the answer the ask named before sending, before or after the wait opens:
// ordinary messages and unrelated wait-only events with the token pass (distinct ones too); a single
// answer is spent by its first delivery, also when the answer is an event that can start graphs.
func TestCorrelationTakesOnlyTheNamedAnswer(t *testing.T) {
	ae := newAcmeEnv(t)
	p := ae.proc()
	ae.create(p, ae.human(), askGraph)
	ae.publish(ae.tenant, orderCreated, "o-1", order{}, 0)
	p.until(func() bool { return ae.count(`SELECT count(*) FROM automation.runs`) == 1 }, "a run")
	ctx := context.Background()
	var runID uuid.UUID
	require.NoError(t, ae.db.QueryRow(ctx, `SELECT id FROM automation.runs`).Scan(&runID))
	issue := func(answer string) string {
		token := engine.NewCorrelationToken()
		_, err := ae.db.Exec(ctx, `INSERT INTO automation.correlations (token, tenant_id, run_id, wait_id, expires_at, event, single)
			VALUES ($1, $2, $3, 'w/answer', now() + interval '1 hour', $4, true)`, token, ae.tenant, runID, answer)
		require.NoError(t, err)
		return token
	}
	publish := func(token, id, event string) error {
		ev := acmeUpdate{ID: id, Event: event, Text: "t", Choice: "yes", From: "ann", Correlation: token}.event(ae.conn.ID.String())
		ev.TenantID, ev.ConnectionID = ae.tenant, ae.conn.ID.String()
		return pgx.BeginFunc(ctx, ae.db, func(tx pgx.Tx) error { _, err := p.rt.Inbound().Publish(ctx, tx, ev); return err })
	}

	// The answer is a wait-only event.
	token := issue(evAnswer)
	require.NoError(t, publish(token, "a-1", evMessage), "an ordinary message passes")
	require.NoError(t, publish(token, "a-2", evMessage), "and a second, distinct one")
	require.NoError(t, publish(token, "a-3", evReceipt), "an unrelated wait-only event passes")
	require.NoError(t, publish(token, "a-4", evReceipt), "and a second, distinct one")
	require.Equal(t, 1, ae.count(`SELECT count(*) FROM automation.correlations WHERE token = $1`, token), "none of them took the token")
	require.NoError(t, publish(token, "a-5", evAnswer), "the answer")
	require.ErrorIs(t, publish(token, "a-6", evAnswer), engine.ErrCorrelationClosed, "a second press")

	// The answer is an event that can also start graphs.
	token = issue(evMessage)
	require.NoError(t, publish(token, "b-1", evAnswer), "another event passes")
	require.NoError(t, publish(token, "b-2", evMessage), "the answer")
	require.ErrorIs(t, publish(token, "b-3", evMessage), engine.ErrCorrelationClosed, "a second, distinct answer is already answered")
}

// An ask that does not name its answer event is refused when the graph is saved.
func TestAskWithoutAnswerEventIsRefused(t *testing.T) {
	ae := newAcmeEnv(t)
	require.NoError(t, ae.reg.RegisterNode(bareAsk{acmeAsk{a: ae.acme}}))
	p := ae.proc()
	_, _, err := p.rt.Service().Create(context.Background(), ae.human(), engine.CreateInput{Graph: json.RawMessage(strings.ReplaceAll(askGraph, `"acme.ask"`, `"acme.ask_bare"`))})
	var ve *engine.ValidationError
	require.ErrorAs(t, err, &ve)
	assert.Equal(t, engine.CodeUnsupported, ve.Errors[0].Code)
}

// bareAsk is an Asker without AnswerEventer.
type bareAsk struct{ inner acmeAsk }

func (b bareAsk) Spec() graph.NodeSpec {
	s := b.inner.Spec()
	s.Type = "acme.ask_bare"
	return s
}
func (b bareAsk) Requires() []catalog.ConnectorKind { return nil }
func (b bareAsk) Idempotent() bool                  { return false }
func (b bareAsk) Execute(ctx context.Context, in catalog.CallInput) (catalog.CallResult, error) {
	return b.inner.Execute(ctx, in)
}
func (b bareAsk) Await(in catalog.CallInput, res catalog.CallResult) (engine.Await, error) {
	return b.inner.Await(in, res)
}
func (b bareAsk) Answer(payload json.RawMessage) (string, json.RawMessage, error) {
	return b.inner.Answer(payload)
}

// An ordinary message and two presses of the answer, all before the wait opens: the second press is
// already answered, one answer reaches the outbox, the message is published, and the run moves once.
func TestEarlyAnswersAfterAnUnrelatedEvent(t *testing.T) {
	ae := newAcmeEnv(t)
	p := ae.proc()
	h := ae.handler(p, nil)
	codes := []int{}
	ae.acme.onAsk = func(token string) {
		for _, u := range []acmeUpdate{
			{ID: "e-1", Event: evMessage, Text: "hi", From: "ann", Correlation: token},
			{ID: "e-2", Event: evAnswer, Choice: "yes", From: "ann", Correlation: token},
			{ID: "e-3", Event: evAnswer, Choice: "no", From: "ann", Correlation: token},
		} {
			codes = append(codes, ae.hook(h, ae.hookPath(), acmeSig, u).Code)
		}
	}
	ae.create(p, ae.human(), askGraph)
	ae.publish(ae.tenant, orderCreated, "o-1", order{}, 0)
	p.until(func() bool { return len(noteTexts(t, ae.env)) == 1 }, "the early answer moved the run")
	assert.Equal(t, []int{http.StatusOK, http.StatusOK, http.StatusOK}, codes)
	assert.Equal(t, int64(1), ae.acme.closed.Load(), "the second press is already answered")
	assert.Equal(t, 1, ae.count(`SELECT count(*) FROM extruntime.outbox_events WHERE event = $1`, evAnswer))
	assert.Equal(t, 1, ae.count(`SELECT count(*) FROM extruntime.outbox_events WHERE event = $1`, evMessage))
	assert.Equal(t, []string{"answer yes"}, noteTexts(t, ae.env))
	assert.Zero(t, ae.count(`SELECT count(*) FROM automation.correlations`), "the token is spent once the wait opens")
}

// The answer named when the token was issued holds across a replay: an ask that names another answer
// after it resumed (here after a 429) is not sent again, and the token keeps its answer.
func TestAnswerOfAResumedAskIsTheIssuedOne(t *testing.T) {
	ae := newAcmeEnv(t)
	p := ae.proc()
	ae.acme.onAsk = func(string) { ae.acme.answerKey.Store(evReceipt) } // the selection changes after the first send
	ae.create(p, ae.human(), strings.Replace(askGraph, `"params": {"text": "approve?"}`, `"params": {"text": "approve?", "mode": "429_once"}`, 1))
	ae.publish(ae.tenant, orderCreated, "o-1", order{}, 0)
	p.until(func() bool { return len(noteTexts(t, ae.env)) == 1 }, "the run ends")
	assert.Equal(t, []string{"bad"}, noteTexts(t, ae.env), "the ask fails on the changed answer")
	calls := ae.acme.callsOf("acme.ask")
	require.Len(t, calls, 1, "it is not sent again")
	assert.Zero(t, ae.count(`SELECT count(*) FROM automation.waits WHERE correlation IS NOT NULL`), "no wait for the other answer")
	var issued string
	require.NoError(t, ae.db.QueryRow(context.Background(), `SELECT result->>'event' FROM extruntime.workflow_occurrences WHERE name = 'correlation'`).Scan(&issued))
	assert.Equal(t, evAnswer, issued, "the receipt keeps the issued answer")
}

// An older event that arrives late joins its window without shortening it.
func TestLateEventDoesNotShortenTheWindow(t *testing.T) {
	ae := newAcmeEnv(t)
	p := ae.proc()
	am := ae.create(p, ae.human(), messageGraph(`{"id": "m", "event": "acme.message.v1", "collect": {"key": "event.from", "window": "60s"}}`))
	ctx := context.Background()
	base := time.Now().Truncate(time.Second)
	for i, off := range []int{0, 40, 10} {
		ev := acmeUpdate{ID: fmt.Sprintf("l-%d", i), Event: evMessage, Text: "t", From: "ann"}.event(ae.conn.ID.String())
		ev.TenantID, ev.ConnectionID, ev.OccurredAt = ae.tenant, ae.conn.ID.String(), base.Add(time.Duration(off)*time.Second)
		require.NoError(t, pgx.BeginFunc(ctx, ae.db, func(tx pgx.Tx) error { _, err := p.rt.Inbound().Publish(ctx, tx, ev); return err }))
		_, err := p.disp.RunOnce(ctx)
		require.NoError(t, err)
	}
	var deadline time.Time
	require.NoError(t, ae.db.QueryRow(ctx, `SELECT deadline FROM automation.inbound_collect WHERE automation_id = $1`, am.ID).Scan(&deadline))
	assert.WithinDuration(t, base.Add(100*time.Second), deadline, time.Second, "the deadline of t=40 stays")
}

// A wait filter that reads run.id is decided with the run's own id at the router.
func TestWaitFilterReadsTheRunAtTheRouter(t *testing.T) {
	ae := newAcmeEnv(t)
	p := ae.proc()
	ae.create(p, ae.human(), graphJSON("Run filter", `{"id": "created", "event": "acme.order.created.v1"}`,
		`{"id": "pay", "type": "wait", "params": {"event": "acme.order.paid.v1", "filter": "run.id != \"\"", "timeout": "1h"}},
		 {"id": "paid", "type": "acme.note", "params": {"text": "paid"}}, {"id": "fin", "type": "end", "params": {}}`,
		`{"from": {"node": "created", "outcome": "fired"}, "to": "pay"}, {"from": {"node": "pay", "outcome": "matched"}, "to": "paid"},
		 {"from": {"node": "pay", "outcome": "timeout"}, "to": "fin"}, {"from": {"node": "paid", "outcome": "done"}, "to": "fin"}, {"from": {"node": "paid", "outcome": "failed"}, "to": "fin"}`))
	ae.publish(ae.tenant, orderCreated, "o-1", order{}, 0)
	p.until(func() bool {
		return ae.count(`SELECT count(*) FROM automation.waits WHERE status = 'open' AND kind = 'event'`) == 1
	}, "the wait opens")
	ae.publish(ae.tenant, orderPaid, "o-1", order{Amount: 1}, 0)
	p.until(func() bool { return len(noteTexts(t, ae.env)) == 1 }, "the payment wakes the wait")
}

// A source may declare by_self only as the engine's optional bool.
func TestBySelfDeclarationIsTheEngines(t *testing.T) {
	reg := catalog.NewRegistry(nil)
	err := reg.RegisterTriggers(bySelfSource{field: expr.StringField})
	require.Error(t, err)
	require.NoError(t, catalog.NewRegistry(nil).RegisterTriggers(bySelfSource{field: expr.OptionalOf(expr.BoolField)}))
}

type bySelfSource struct{ field expr.Field }

func (s bySelfSource) Requires() []catalog.ConnectorKind { return nil }
func (s bySelfSource) Events() []graph.EventSpec {
	return []graph.EventSpec{{Key: "x.msg.v1", Title: "M", Subject: graph.SubjectNone, Payload: expr.Fields{{Name: "by_self", Field: s.field}}}}
}
