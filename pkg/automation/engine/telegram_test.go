package engine_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/pkg/automation/catalog"
	"github.com/iota-uz/iota-sdk/pkg/automation/connectors"
	"github.com/iota-uz/iota-sdk/pkg/automation/connectors/telegram"
	"github.com/iota-uz/iota-sdk/pkg/automation/connectors/telegram/telegramtest"
	"github.com/iota-uz/iota-sdk/pkg/automation/engine"
)

// The Telegram connector in the engine: a real database, the engine's webhook handler, and the fake Bot
// API that posts to it.

const (
	tgToken         = "111111:AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
	tgPlatformToken = "222222:BBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB"
)

// lazyDir resolves the directory once the environment has built it.
type lazyDir struct {
	d atomic.Pointer[connectors.Directory]
}

func (l *lazyDir) Register(c connectors.Connector) error { return (*l.d.Load()).Register(c) }
func (l *lazyDir) List() []connectors.Connector          { return (*l.d.Load()).List() }
func (l *lazyDir) Get(k catalog.ConnectorKind) (connectors.Connector, bool) {
	return (*l.d.Load()).Get(k)
}

// lateInbound is the runtime's Inbound of the current process.
type lateInbound struct{ in atomic.Value }

func (l *lateInbound) get() engine.Inbound { return l.in.Load().(engine.Inbound) }
func (l *lateInbound) Publish(ctx context.Context, tx pgx.Tx, ev engine.InboundEvent) (bool, error) {
	return l.get().Publish(ctx, tx, ev)
}
func (l *lateInbound) Correlation(ctx context.Context, token string) (uuid.UUID, bool, error) {
	return l.get().Correlation(ctx, token)
}

type memUploads struct {
	mu    sync.Mutex
	files map[string]telegram.UploadFile
}

func (m *memUploads) Save(_ context.Context, _ uuid.UUID, f telegram.UploadFile) (telegram.Stored, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.files == nil {
		m.files = map[string]telegram.UploadFile{}
	}
	id := uuid.NewString()
	m.files[id] = f
	return telegram.Stored{ID: id, URL: "/uploads/" + id}, nil
}

func (m *memUploads) Open(_ context.Context, _ uuid.UUID, id string) (telegram.UploadFile, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if f, ok := m.files[id]; ok {
		return f, nil
	}
	return telegram.UploadFile{}, errors.New("not found")
}

type tgEnv struct {
	*env
	api      *telegramtest.Server
	tg       *telegram.Connector
	bot      *telegramtest.Bot
	platform *telegramtest.Bot
	store    connectors.Store
	dir      connectors.Directory
	conn     connectors.Connection
	srv      *httptest.Server
	late     *lateInbound
	uploads  *memUploads
	hook     atomic.Value // http.Handler
}

func newTgEnv(t *testing.T) *tgEnv {
	t.Helper()
	te := &tgEnv{api: telegramtest.New(), late: &lateInbound{}, uploads: &memUploads{}}
	t.Cleanup(te.api.Close)
	keys, err := connectors.NewKeyring("k1", map[string][]byte{"k1": []byte("0123456789abcdef0123456789abcdef")})
	require.NoError(t, err)
	ld := &lazyDir{}
	te.env = newEnvWith(t, func(db *pgxpool.Pool) catalog.Availability {
		platform := connectors.NewPlatformCredentials(ld, map[catalog.ConnectorKind]connectors.PlatformCredential{
			telegram.Kind: {Credentials: connectors.Credentials{"token": tgPlatformToken}}})
		te.store = connectors.NewStore(db, keys, nil, platform, nil)
		return te.store
	})
	te.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h, _ := te.hook.Load().(http.Handler)
		if h == nil {
			http.NotFound(w, r)
			return
		}
		h.ServeHTTP(w, r)
	}))
	t.Cleanup(te.srv.Close)
	te.tg = telegram.New(telegram.Config{
		PlatformToken: tgPlatformToken, PublicOrigin: te.srv.URL, Inbound: te.late, APIURL: te.api.URL(),
		Uploads: te.uploads, Connections: te.store,
	})
	te.dir = connectors.NewDirectory(te.reg)
	ld.d.Store(&te.dir)
	require.NoError(t, te.dir.Register(te.tg))
	te.bot = te.api.Bot(tgToken, "tenant_bot")
	te.platform = te.api.Bot(tgPlatformToken, "platform_bot")
	te.conn, err = te.store.Save(context.Background(), connectors.Connection{TenantID: te.tenant, Kind: telegram.Kind, Name: "bot", DisplayName: "Bot"},
		connectors.Credentials{"token": tgToken})
	require.NoError(t, err)
	return te
}

func (te *tgEnv) proc(tune ...func(*engine.Config)) *proc {
	p := te.env.proc(append([]func(*engine.Config){func(c *engine.Config) {
		c.Credentials = te.store
		c.Usage = connectors.NewUsageRecorder(te.db)
		c.CallLimits.BackoffBase = 10 * time.Millisecond
	}}, tune...)...)
	te.late.in.Store(p.rt.Inbound())
	te.hook.Store(engine.InboundHandler(engine.InboundConfig{
		Connections: te.store, Registry: te.reg, Inbound: p.rt.Inbound(), DB: te.db, Connectors: te.dir,
		Platform: connectors.NewPlatformCredentials(te.dir, map[catalog.ConnectorKind]connectors.PlatformCredential{
			telegram.Kind: {Credentials: connectors.Credentials{"token": tgPlatformToken}}}),
	}))
	return p
}

// provision sets the webhooks as the integrations service and the application's start do.
func (te *tgEnv) provision() {
	te.t.Helper()
	ctx := context.Background()
	state, err := te.tg.Provision(ctx, te.conn, connectors.Credentials{"token": tgToken}, []string{telegram.WarnPollingUnknown + "@" + te.tg.ExternalID(connectors.Credentials{"token": tgToken})}, nil)
	require.NoError(te.t, err)
	// The secret Provision generated is kept in the connection's inbound state, as the service does.
	require.NoError(te.t, te.store.UpdateInbound(ctx, te.tenant, te.conn.ID, func(cur json.RawMessage) (json.RawMessage, error) {
		return connectors.MergeInbound(cur, state)
	}))
	require.NoError(te.t, te.tg.StartPlatform(ctx, connectors.Credentials{"token": tgPlatformToken}))
}

// ok requires that a delivery of Telegram's reached the webhook and was accepted (200).
func (te *tgEnv) ok(d telegramtest.Delivery, err error) {
	te.t.Helper()
	require.NoError(te.t, err)
	require.Equal(te.t, http.StatusOK, d.Status, "the update was not accepted")
}

// messages is the number of message events in the outbox.
func (te *tgEnv) messages() int {
	return te.count(`SELECT count(*) FROM extruntime.outbox_events WHERE event = 'telegram.message_received.v1'`)
}

func (te *tgEnv) chat(id int64) { te.api.Chat(telegramtest.Chat{ID: id, Type: "private"}) }

var tgAlice = telegramtest.User{ID: 501, Username: "alice", FirstName: "Alice"}

// tgAskGraph asks with two buttons and notes the pressed one.
const tgAskGraph = `{"name": "Approval", "triggers": [{"id": "created", "event": "acme.order.created.v1"}],
 "nodes": [
  {"id": "ask", "type": "telegram.ask", "params": {"chat_id": 700, "text": "Approve the order of {{ event.note }}?",
     "options": [{"outcome": "approve", "label": "Approve {{ event.note }}"}, {"outcome": "reject", "label": "Reject"}], "timeout": "1h"}},
  {"id": "yes", "type": "acme.note", "params": {"text": "approved by {{ nodes.ask.output.from.username }} as {{ nodes.ask.output.label }} ({{ nodes.ask.output.choice }})"}},
  {"id": "no", "type": "acme.note", "params": {"text": "rejected {{ nodes.ask.output.choice }}"}},
  {"id": "late", "type": "acme.note", "params": {"text": "timed out"}},
  {"id": "bad", "type": "acme.note", "params": {"text": "failed {{ nodes.ask.output.error.code }}"}},
  {"id": "fin", "type": "end", "params": {}}
 ],
 "edges": [
  {"from": {"node": "created", "outcome": "fired"}, "to": "ask"},
  {"from": {"node": "ask", "outcome": "approve"}, "to": "yes"},
  {"from": {"node": "ask", "outcome": "reject"}, "to": "no"},
  {"from": {"node": "ask", "outcome": "timeout"}, "to": "late"},
  {"from": {"node": "ask", "outcome": "failed"}, "to": "bad"},
  {"from": {"node": "yes", "outcome": "done"}, "to": "fin"}, {"from": {"node": "yes", "outcome": "failed"}, "to": "fin"},
  {"from": {"node": "no", "outcome": "done"}, "to": "fin"}, {"from": {"node": "no", "outcome": "failed"}, "to": "fin"},
  {"from": {"node": "late", "outcome": "done"}, "to": "fin"}, {"from": {"node": "late", "outcome": "failed"}, "to": "fin"},
  {"from": {"node": "bad", "outcome": "done"}, "to": "fin"}, {"from": {"node": "bad", "outcome": "failed"}, "to": "fin"}
 ]}`

// Acceptance: a graph asks with two buttons, the user presses the second, the run goes by its outcome, the
// keyboard is gone; the press again is answered without a transition.
func TestTelegramAskTakesThePressedButtonAsTheOutcome(t *testing.T) {
	te := newTgEnv(t)
	p := te.proc()
	te.provision()
	te.chat(700)
	te.create(p, te.human(), tgAskGraph)
	te.publish(te.tenant, orderCreated, "o-1", order{Note: "#42"}, 0)
	p.until(func() bool {
		return len(te.bot.Sent()) == 1 && te.count(`SELECT count(*) FROM automation.waits WHERE status = 'open' AND correlation IS NOT NULL`) == 1
	}, "the question is sent and its answer wait is open")
	q := te.bot.Sent()[0]
	assert.Equal(t, "Approve the order of #42?", q.Text)
	require.Len(t, q.Markup, 2)
	assert.Equal(t, "Approve #42", q.Markup[0][0].Text)
	assert.Equal(t, "Reject", q.Markup[1][0].Text)

	// The second button is pressed through the webhook.
	d, query, err := te.bot.PressData(700, q.ID, tgAlice, q.Markup[1][0].CallbackData, q)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, d.Status)
	p.until(func() bool { return len(noteTexts(t, te.env)) == 1 }, "the press moved the run")
	run := runOf(t, te.env, p)
	p.until(func() bool { return te.status(p, run.RunID) == engine.RunDone }, "the run is done")
	assert.Equal(t, []string{"rejected reject"}, noteTexts(t, te.env), "the second button's outcome")

	_, answered := te.bot.Answered(query)
	assert.True(t, answered, "the press is acknowledged")
	after, afterFound := te.bot.Message(700, q.ID)
	require.True(t, afterFound, "the original message is there")
	assert.Empty(t, after.Markup, "the keyboard is removed")
	assert.Equal(t, "Approve the order of #42?\n\n→ Reject", after.Text, "the text carries the chosen option")

	// The first button of the same (old) keyboard pressed again: answered, nothing moves.
	d, query, err = te.bot.PressData(700, q.ID, tgAlice, q.Markup[0][0].CallbackData, q)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, d.Status)
	text, answered := te.bot.Answered(query)
	assert.True(t, answered)
	assert.Equal(t, telegram.AlreadyAnswered, text)
	p.pass()
	assert.Equal(t, []string{"rejected reject"}, noteTexts(t, te.env), "no second transition")
	assert.Equal(t, 1, te.count(`SELECT count(*) FROM automation.runs`))
	final, finalFound := te.bot.Message(700, q.ID)
	require.True(t, finalFound, "the original message is there")
	assert.Equal(t, after.Text, final.Text, "the text stays as it was")
	assert.Equal(t, 1, te.count(`SELECT count(*) FROM automation.connector_usage WHERE kind = 'telegram_bot' AND unit = 'message' AND connection_id = $1`, te.conn.ID.String()))
}

// A press of the ask whose time ran out is answered the same way, and the keyboard goes then; the
// timeout itself leaves the message alone.
func TestTelegramAskTimeout(t *testing.T) {
	te := newTgEnv(t)
	p := te.proc()
	te.provision()
	te.chat(700)
	te.create(p, te.human(), strings.Replace(tgAskGraph, `"timeout": "1h"`, `"timeout": "5s"`, 1))
	te.publish(te.tenant, orderCreated, "o-1", order{Note: "#1"}, 0)
	p.until(func() bool { return len(te.bot.Sent()) == 1 }, "the question is sent")
	q := te.bot.Sent()[0]
	p.until(func() bool { return len(noteTexts(t, te.env)) == 1 }, "the ask times out")
	assert.Equal(t, []string{"timed out"}, noteTexts(t, te.env))
	same, sameFound := te.bot.Message(700, q.ID)
	require.True(t, sameFound, "the original message is there")
	assert.Len(t, same.Markup, 2, "the timeout does not change the message")
	assert.Equal(t, q.Text, same.Text)

	_, query, err := te.bot.PressData(700, q.ID, tgAlice, q.Markup[0][0].CallbackData, q)
	require.NoError(t, err)
	text, _ := te.bot.Answered(query)
	assert.Equal(t, telegram.AlreadyAnswered, text)
	late, lateFound := te.bot.Message(700, q.ID)
	require.True(t, lateFound, "the original message is there")
	assert.Empty(t, late.Markup, "the keyboard is removed at the late press")
	assert.Equal(t, q.Text, late.Text)
	assert.Equal(t, []string{"timed out"}, noteTexts(t, te.env))
}

// tgMessageGraph notes the messages its trigger takes.
func tgMessageGraph(trigger string) string {
	return graphJSON("Inbound", trigger,
		`{"id": "note", "type": "acme.note", "params": {"text": "{{ event.text }}|{{ event.command }}|{{ event.chatId }}|{{ string(size(event.files)) }}"}},
		 {"id": "fin", "type": "end", "params": {}}`,
		`{"from": {"node": "m", "outcome": "fired"}, "to": "note"},
		 {"from": {"node": "note", "outcome": "done"}, "to": "fin"}, {"from": {"node": "note", "outcome": "failed"}, "to": "fin"}`)
}

func TestTelegramWebhookSecretAndDuplicateUpdates(t *testing.T) {
	te := newTgEnv(t)
	p := te.proc()
	te.provision()
	te.chat(700)
	te.create(p, te.human(), tgMessageGraph(`{"id": "m", "event": "telegram.message_received.v1"}`))
	hook := te.bot.Webhook()
	require.Equal(t, te.srv.URL+"/automations/hooks/"+te.conn.ID.String(), hook.URL)

	u := te.bot.Update(700, tgAlice, "hello")
	// A wrong secret is 404, and nothing runs.
	d, err := te.bot.Post(u, "not-the-secret-not-the-secret-12")
	require.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, d.Status)
	d, err = te.bot.Post(u, "")
	require.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, d.Status)
	p.pass()
	assert.Zero(t, te.count(`SELECT count(*) FROM automation.runs`))

	// The right one runs the graph; the same update_id again is the same message.
	d, err = te.bot.Redeliver(u)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, d.Status)
	d, err = te.bot.Redeliver(u)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, d.Status)
	p.until(func() bool { return len(noteTexts(t, te.env)) >= 1 }, "the message ran the graph")
	p.pass()
	assert.Equal(t, 1, te.count(`SELECT count(*) FROM automation.runs`), "a repeat of the update_id creates no second run")
	assert.Equal(t, []string{"hello||700|0"}, noteTexts(t, te.env))
}

// A message to the platform bot belongs to nobody: no graph starts. A command is ignored by default and a
// trigger that takes commands gets it.
func TestTelegramPlatformMessagesAndCommands(t *testing.T) {
	te := newTgEnv(t)
	p := te.proc()
	te.provision()
	te.chat(800)
	te.chat(700)
	te.create(p, te.human(), tgMessageGraph(`{"id": "m", "event": "telegram.message_received.v1"}`))

	d, err := te.platform.SendText(800, tgAlice, "hello platform")
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, d.Status)
	d, err = te.platform.SendText(800, tgAlice, "/start something")
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, d.Status)
	p.pass()
	assert.Zero(t, te.count(`SELECT count(*) FROM automation.runs`), "the platform bot starts nobody's graph")
	assert.Zero(t, te.count(`SELECT count(*) FROM extruntime.outbox_events WHERE event = 'telegram.message_received.v1'`))

	// The tenant's own bot: a command is ignored by the default trigger…
	te.ok(te.bot.SendText(700, tgAlice, "/help me"))
	te.ok(te.bot.SendText(700, tgAlice, "plain"))
	p.until(func() bool { return len(noteTexts(t, te.env)) >= 1 }, "the plain message runs")
	p.pass()
	assert.Equal(t, []string{"plain||700|0"}, noteTexts(t, te.env), "commands: ignore by default")
	assert.Equal(t, 2, te.count(`SELECT count(*) FROM extruntime.outbox_events WHERE event = 'telegram.message_received.v1'`), "the command is an event, no graph takes it")
}

func TestTelegramCommandsOnlyTrigger(t *testing.T) {
	te := newTgEnv(t)
	p := te.proc()
	te.provision()
	te.chat(700)
	te.create(p, te.human(), tgMessageGraph(`{"id": "m", "event": "telegram.message_received.v1", "params": {"commands": "only", "command_names": ["help"]}}`))
	for _, text := range []string{"plain", "/other", "/help topic"} {
		te.ok(te.bot.SendText(700, tgAlice, text))
	}
	p.until(func() bool { return len(noteTexts(t, te.env)) >= 1 }, "the command runs")
	p.pass()
	assert.Equal(t, []string{"/help topic|help|700|0"}, noteTexts(t, te.env))
}

// Acceptance: a photo without a caption starts the trigger, the file is at files[0].url.
func TestTelegramPhotoWithoutCaptionStartsTheTrigger(t *testing.T) {
	te := newTgEnv(t)
	p := te.proc()
	te.provision()
	te.chat(700)
	g := graphJSON("Photo", `{"id": "m", "event": "telegram.message_received.v1"}`,
		`{"id": "note", "type": "acme.note", "params": {"text": "[{{ event.text }}] {{ event.files[0].url }} {{ event.files[0].mime }}"}},
		 {"id": "fin", "type": "end", "params": {}}`,
		`{"from": {"node": "m", "outcome": "fired"}, "to": "note"},
		 {"from": {"node": "note", "outcome": "done"}, "to": "fin"}, {"from": {"node": "note", "outcome": "failed"}, "to": "fin"}`)
	te.create(p, te.human(), g)
	te.ok(te.bot.SendPhoto(700, tgAlice, "", []byte("JPEGDATA")))
	p.until(func() bool { return len(noteTexts(t, te.env)) == 1 }, "the photo starts the graph")
	te.uploads.mu.Lock()
	defer te.uploads.mu.Unlock()
	require.Len(t, te.uploads.files, 1)
	for id, f := range te.uploads.files {
		assert.Equal(t, []string{"[] /uploads/" + id + " image/jpeg"}, noteTexts(t, te.env))
		assert.Equal(t, []byte("JPEGDATA"), f.Data)
	}
}

// send in a graph: a topic, files, a blocked chat (failed with the code rejected, the connection stays
// connected), and a 429 that is repeated after its delay with no second message.
func TestTelegramSendOutcomes(t *testing.T) {
	te := newTgEnv(t)
	p := te.proc()
	te.provision()
	te.chat(700)
	te.api.Chat(telegramtest.Chat{ID: -1003, Type: "supergroup", Forum: true, Topics: map[int64]bool{77: true}})
	te.uploads.files = map[string]telegram.UploadFile{"f1": {Name: "r.pdf", Mime: "application/pdf", Data: []byte("%PDF")}}
	g := func(params string) string {
		return graphJSON("Send", `{"id": "created", "event": "acme.order.created.v1"}`,
			`{"id": "send", "type": "telegram.send", "params": `+params+`},
			 {"id": "ok", "type": "acme.note", "params": {"text": "sent {{ nodes.send.output.chat_id }}"}},
			 {"id": "bad", "type": "acme.note", "params": {"text": "{{ nodes.send.output.error.code }}"}},
			 {"id": "fin", "type": "end", "params": {}}`,
			`{"from": {"node": "created", "outcome": "fired"}, "to": "send"},
			 {"from": {"node": "send", "outcome": "sent"}, "to": "ok"}, {"from": {"node": "send", "outcome": "failed"}, "to": "bad"},
			 {"from": {"node": "ok", "outcome": "done"}, "to": "fin"}, {"from": {"node": "ok", "outcome": "failed"}, "to": "fin"},
			 {"from": {"node": "bad", "outcome": "done"}, "to": "fin"}, {"from": {"node": "bad", "outcome": "failed"}, "to": "fin"}`)
	}
	am := te.create(p, te.human(), g(`{"chat_id": -1003, "thread_id": 77, "text": "to the topic {{ event.note }}", "files": ["f1"]}`))
	te.publish(te.tenant, orderCreated, "o-1", order{Note: "a"}, 0)
	p.until(func() bool { return len(noteTexts(t, te.env)) == 1 }, "sent into the topic")
	sent := te.bot.Sent()
	require.Len(t, sent, 1)
	assert.Equal(t, int64(77), sent[0].ThreadID)
	assert.Equal(t, []string{"r.pdf"}, sent[0].Files)
	assert.Equal(t, "to the topic a", sent[0].Caption)
	assert.Equal(t, []string{"sent -1003"}, noteTexts(t, te.env))

	// A blocked chat: failed, rejected; the connection is still connected.
	_, err := te.db.Exec(context.Background(), `DELETE FROM test_effects`)
	require.NoError(t, err)
	_, err = p.rt.Service().Disable(context.Background(), te.human(), am.ID)
	require.NoError(t, err)
	te.bot.Block(700)
	te.create(p, te.human(), g(`{"chat_id": 700, "text": "hi"}`))
	te.publish(te.tenant, orderCreated, "o-2", order{Note: "b"}, 0)
	p.until(func() bool { return len(noteTexts(t, te.env)) == 1 }, "the blocked send fails")
	assert.Equal(t, []string{"rejected"}, noteTexts(t, te.env))
	c, _, err := te.store.Get(context.Background(), te.tenant, te.conn.ID)
	require.NoError(t, err)
	assert.Equal(t, connectors.StatusConnected, c.Status)
}

func TestTelegramSendRepeatsAfterTheRateLimitWithoutASecondMessage(t *testing.T) {
	te := newTgEnv(t)
	p := te.proc()
	te.provision()
	te.chat(700)
	te.bot.Fail(telegramtest.Failure{Method: "sendMessage", Code: 429, Description: "Too Many Requests: retry after 1", RetryAfter: 1})
	g := graphJSON("Send", `{"id": "created", "event": "acme.order.created.v1"}`,
		`{"id": "send", "type": "telegram.send", "params": {"chat_id": 700, "text": "once"}},
		 {"id": "ok", "type": "acme.note", "params": {"text": "sent"}}, {"id": "fin", "type": "end", "params": {}}`,
		`{"from": {"node": "created", "outcome": "fired"}, "to": "send"}, {"from": {"node": "send", "outcome": "sent"}, "to": "ok"},
		 {"from": {"node": "send", "outcome": "failed"}, "to": "ignore"},
		 {"from": {"node": "ok", "outcome": "done"}, "to": "fin"}, {"from": {"node": "ok", "outcome": "failed"}, "to": "fin"}`)
	te.create(p, te.human(), g)
	te.publish(te.tenant, orderCreated, "o-1", order{}, 0)
	p.until(func() bool { return len(noteTexts(t, te.env)) == 1 }, "the send is repeated")
	assert.Len(t, te.bot.Sent(), 1, "one message was sent")
	calls := te.bot.CallsOf("sendMessage")
	require.Len(t, calls, 2, "the refused try and the repeat")
	assert.GreaterOrEqual(t, calls[1].At.Sub(calls[0].At), time.Second, "the repeat came after retry_after, measured between the two requests")
}

// A send whose answer is lost is outcome_unknown, never a second message.
func TestTelegramLostAnswerIsOutcomeUnknown(t *testing.T) {
	te := newTgEnv(t)
	p := te.proc()
	te.provision()
	te.chat(700)
	te.bot.Fail(telegramtest.Failure{Method: "sendMessage", Lost: true})
	g := graphJSON("Send", `{"id": "created", "event": "acme.order.created.v1"}`,
		`{"id": "send", "type": "telegram.send", "params": {"chat_id": 700, "text": "once"}},
		 {"id": "bad", "type": "acme.note", "params": {"text": "{{ nodes.send.output.error.code }}"}}, {"id": "fin", "type": "end", "params": {}}`,
		`{"from": {"node": "created", "outcome": "fired"}, "to": "send"}, {"from": {"node": "send", "outcome": "sent"}, "to": "ignore"},
		 {"from": {"node": "send", "outcome": "failed"}, "to": "bad"},
		 {"from": {"node": "bad", "outcome": "done"}, "to": "fin"}, {"from": {"node": "bad", "outcome": "failed"}, "to": "fin"}`)
	te.create(p, te.human(), g)
	te.publish(te.tenant, orderCreated, "o-1", order{}, 0)
	p.until(func() bool { return len(noteTexts(t, te.env)) == 1 }, "the unknown outcome takes failed")
	assert.Equal(t, []string{"outcome_unknown"}, noteTexts(t, te.env))
	assert.Len(t, te.bot.Sent(), 1, "the message was sent once")
}

// A refused token marks the connection needs_reauth and the node fails with unauthorized.
func TestTelegramUnauthorizedNeedsReauth(t *testing.T) {
	te := newTgEnv(t)
	p := te.proc()
	te.provision()
	te.chat(700)
	te.bot.Fail(telegramtest.Failure{Method: "sendMessage", Code: 401, Description: "Unauthorized"})
	g := graphJSON("Send", `{"id": "created", "event": "acme.order.created.v1"}`,
		`{"id": "send", "type": "telegram.send", "params": {"chat_id": 700, "text": "x"}},
		 {"id": "bad", "type": "acme.note", "params": {"text": "{{ nodes.send.output.error.code }}"}}, {"id": "fin", "type": "end", "params": {}}`,
		`{"from": {"node": "created", "outcome": "fired"}, "to": "send"}, {"from": {"node": "send", "outcome": "sent"}, "to": "ignore"},
		 {"from": {"node": "send", "outcome": "failed"}, "to": "bad"},
		 {"from": {"node": "bad", "outcome": "done"}, "to": "fin"}, {"from": {"node": "bad", "outcome": "failed"}, "to": "fin"}`)
	te.create(p, te.human(), g)
	te.publish(te.tenant, orderCreated, "o-1", order{}, 0)
	p.until(func() bool { return len(noteTexts(t, te.env)) == 1 }, "unauthorized takes failed")
	assert.Equal(t, []string{"unauthorized"}, noteTexts(t, te.env))
	c, _, err := te.store.Get(context.Background(), te.tenant, te.conn.ID)
	require.NoError(t, err)
	assert.Equal(t, connectors.StatusNeedsReauth, c.Status)
}

// telegram.read waits for the text of the user the question was put to, in that chat.
func TestTelegramReadWaitsForTheAskedUsersMessage(t *testing.T) {
	te := newTgEnv(t)
	p := te.proc()
	te.provision()
	te.chat(700)
	g := graphJSON("Dialog", `{"id": "m", "event": "telegram.message_received.v1"}`,
		`{"id": "ask", "type": "telegram.ask", "params": {"chat_id": "event.chatId", "text": "Ready?", "options": [{"outcome": "yes", "label": "Yes"}, {"outcome": "no", "label": "No"}]}},
		 {"id": "read", "type": "telegram.read", "params": {"chat_id": "event.chatId", "from_user_id": "nodes.ask.output.from.user_id", "timeout": "1h"}},
		 {"id": "got", "type": "acme.note", "params": {"text": "got {{ nodes.read.output.event.text }} from {{ nodes.read.output.event.from.username }}"}},
		 {"id": "late", "type": "acme.note", "params": {"text": "late"}},
		 {"id": "fin", "type": "end", "params": {}}`,
		`{"from": {"node": "m", "outcome": "fired"}, "to": "ask"},
		 {"from": {"node": "ask", "outcome": "yes"}, "to": "read"}, {"from": {"node": "ask", "outcome": "no"}, "to": "ignore"},
		 {"from": {"node": "ask", "outcome": "timeout"}, "to": "ignore"}, {"from": {"node": "ask", "outcome": "failed"}, "to": "ignore"},
		 {"from": {"node": "read", "outcome": "received"}, "to": "got"}, {"from": {"node": "read", "outcome": "timeout"}, "to": "late"},
		 {"from": {"node": "got", "outcome": "done"}, "to": "fin"}, {"from": {"node": "got", "outcome": "failed"}, "to": "fin"},
		 {"from": {"node": "late", "outcome": "done"}, "to": "fin"}, {"from": {"node": "late", "outcome": "failed"}, "to": "fin"}`)
	te.create(p, te.human(), g)
	te.ok(te.bot.SendText(700, tgAlice, "start"))
	p.until(func() bool {
		return len(te.bot.Sent()) == 1 && te.count(`SELECT count(*) FROM automation.waits WHERE status = 'open' AND correlation IS NOT NULL`) == 1
	}, "the question is sent and its answer is awaited")
	q := te.bot.Sent()[0]
	_, _, err := te.bot.PressData(700, q.ID, tgAlice, q.Markup[0][0].CallbackData, q)
	require.NoError(t, err)
	p.until(func() bool {
		return te.count(`SELECT count(*) FROM automation.waits WHERE status = 'open' AND kind = 'event' AND event_kind = 'telegram.message_received.v1'`) == 1
	}, "the read waits for a message")
	bob := telegramtest.User{ID: 502, Username: "bob", FirstName: "Bob"}
	before := te.messages()
	te.ok(te.bot.SendText(700, bob, "from the wrong person"))
	assert.Equal(t, before+1, te.messages(), "the message reached the outbox")
	p.pass()
	p.pass()
	assert.Empty(t, noteTexts(t, te.env), "another user's message does not wake the read")
	te.ok(te.bot.SendText(700, tgAlice, "my answer"))
	p.until(func() bool { return len(noteTexts(t, te.env)) == 1 }, "the asked user's message wakes the read")
	assert.Equal(t, []string{"got my answer from alice"}, noteTexts(t, te.env))
}

// A /start with a registered prefix on the platform bot reaches its handler, and starts no graph.
type tgStart struct{ got []telegram.StartUpdate }

func (h *tgStart) Prefix() string { return "u_" }
func (h *tgStart) TenantOf(context.Context, string) (uuid.UUID, bool, error) {
	return uuid.Nil, false, nil
}
func (h *tgStart) Handle(_ context.Context, u telegram.StartUpdate) (string, error) {
	h.got = append(h.got, u)
	return "unknown link", nil
}

func TestTelegramPlatformStartGoesToItsHandler(t *testing.T) {
	te := newTgEnv(t)
	p := te.proc()
	te.provision()
	te.chat(800)
	h := &tgStart{}
	require.NoError(t, te.tg.RegisterStart(h))
	te.create(p, te.human(), tgMessageGraph(`{"id": "m", "event": "telegram.message_received.v1", "params": {"commands": "include"}}`))
	te.ok(te.platform.SendText(800, tgAlice, "/start u_abc"))
	te.ok(te.platform.SendText(800, tgAlice, "/start zz_abc"))
	p.pass()
	require.Len(t, h.got, 1)
	assert.Equal(t, "platform:telegram_bot", h.got[0].ConnectionID)
	assert.Zero(t, te.count(`SELECT count(*) FROM extruntime.outbox_events WHERE event = 'telegram.message_received.v1'`), "/start is no event")
	assert.Zero(t, te.count(`SELECT count(*) FROM automation.runs`))
	assert.Len(t, te.platform.Sent(), 1, "the handler's reply only; the unknown prefix is not answered")
}

// A press on a question asked through the platform bot belongs to the question's tenant.
func TestTelegramPlatformAskIsAnsweredByItsTenant(t *testing.T) {
	te := newTgEnv(t)
	p := te.proc()
	te.provision()
	te.chat(800)
	_, err := te.db.Exec(context.Background(), `DELETE FROM automation.connections`)
	require.NoError(t, err)
	g := strings.Replace(strings.Replace(tgAskGraph, `"chat_id": 700`, `"chat_id": 800, "connection": "platform:telegram_bot"`, 1), `"timeout": "1h"`, `"timeout": "1h"`, 1)
	te.create(p, te.human(), g)
	te.publish(te.tenant, orderCreated, "o-1", order{Note: "#9"}, 0)
	p.until(func() bool {
		return len(te.platform.Sent()) == 1 && te.count(`SELECT count(*) FROM automation.waits WHERE status = 'open' AND correlation IS NOT NULL`) == 1
	}, "the question goes out through the platform bot and its answer is awaited")
	q := te.platform.Sent()[0]
	assert.Equal(t, 1, te.count(`SELECT count(*) FROM automation.connector_usage WHERE connection_id = 'platform:telegram_bot'`), "counted for the platform")
	d, _, err := te.platform.PressData(800, q.ID, tgAlice, q.Markup[0][0].CallbackData, q)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, d.Status)
	p.until(func() bool { return len(noteTexts(t, te.env)) == 1 }, "the press reached the tenant's run")
	assert.Equal(t, []string{"approved by alice as Approve #9 (approve)"}, noteTexts(t, te.env))
}

// After the platform's bot is replaced (the application runs with another APP_TELEGRAMBOTTOKEN), the retired bot's
// webhook still points to the platform route, but nothing can authenticate it any more: its secret is derived from a
// token the application no longer has. Its updates — even the press of a question it asked before the change — are
// refused like any request without the route's secret (404, nothing published), and the question waits for its
// timeout. The new bot's updates are taken.
func TestTelegramRetiredPlatformBotIsRefused(t *testing.T) {
	te := newTgEnv(t)
	p := te.proc()
	te.provision()
	te.chat(800)
	_, err := te.db.Exec(context.Background(), `DELETE FROM automation.connections`)
	require.NoError(t, err)
	g := strings.Replace(tgAskGraph, `"chat_id": 700`, `"chat_id": 800, "connection": "platform:telegram_bot"`, 1)
	te.create(p, te.human(), g)
	te.publish(te.tenant, orderCreated, "o-1", order{Note: "#9"}, 0)
	p.until(func() bool {
		return len(te.platform.Sent()) == 1 && te.count(`SELECT count(*) FROM automation.waits WHERE status = 'open' AND correlation IS NOT NULL`) == 1
	}, "the question goes out through the platform bot")
	q := te.platform.Sent()[0]

	// The platform bot is rotated: the route now verifies the new bot's secret.
	const rotated = "333333:CCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCC"
	successor := te.api.Bot(rotated, "successor_bot")
	te.hook.Store(engine.InboundHandler(engine.InboundConfig{
		Connections: te.store, Registry: te.reg, Inbound: p.rt.Inbound(), DB: te.db, Connectors: te.dir,
		Platform: connectors.NewPlatformCredentials(te.dir, map[catalog.ConnectorKind]connectors.PlatformCredential{
			telegram.Kind: {Credentials: connectors.Credentials{"token": rotated}}}),
	}))
	require.NoError(t, te.tg.StartPlatform(context.Background(), connectors.Credentials{"token": rotated}))

	d, _, err := te.platform.PressData(800, q.ID, tgAlice, q.Markup[0][0].CallbackData, q)
	require.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, d.Status, "the retired bot cannot be authenticated")
	assert.Equal(t, 1, te.count(`SELECT count(*) FROM automation.waits WHERE status = 'open' AND correlation IS NOT NULL`), "the question still waits")
	assert.Empty(t, noteTexts(t, te.env), "no answer reached the run")

	d, err = successor.SendText(800, tgAlice, "hello")
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, d.Status, "the new platform bot is served")
}

// A platform setWebhook that Telegram applied late — from an instance that crashed during the call, after the bot
// stopped being the platform's and a tenant's connection took it, however long after — points the tenant's bot back
// to the platform route.
// The route recognises the stray update (its secret is that bot's platform secret, checked with the connection's
// token), points the webhook back to the connection and answers 503; Telegram's next delivery reaches the connection
// and runs its graph. A request no retired account verifies stays a plain 404.
func TestTelegramStrayPlatformWebhookIsPointedBackToItsConnection(t *testing.T) {
	te := newTgEnv(t)
	p := te.proc()
	te.provision()
	te.chat(700)
	te.create(p, te.human(), tgMessageGraph(`{"id": "m", "event": "telegram.message_received.v1"}`))
	ctx := context.Background()
	account := te.tg.ExternalID(connectors.Credentials{"token": tgToken})
	// The tenant's bot was the platform's: its reservation's lease ran out and the prune released it (recorded as
	// retired). Days pass with nobody holding it, prunes keep running, and only then a tenant's connection takes it.
	creds := connectors.Credentials{"token": tgToken}
	require.NoError(t, connectors.StartPlatformIdentity(ctx, te.db, telegram.Kind, account, te.tg.PlatformStrayKey(creds), nil))
	_, err := te.db.Exec(ctx, `UPDATE automation.exclusive_identities SET lease_until = now() - interval '1 second' WHERE platform AND external = $1`, account)
	require.NoError(t, err)
	require.NoError(t, connectors.PrunePlatformIdentities(ctx, te.db, telegram.Kind))
	_, err = te.db.Exec(ctx, `UPDATE automation.retired_platform_identities SET retired_at = now() - interval '3 days' WHERE external = $1`, account)
	require.NoError(t, err)
	require.NoError(t, connectors.PrunePlatformIdentities(ctx, te.db, telegram.Kind))
	require.NoError(t, pgx.BeginFunc(ctx, te.db, func(tx pgx.Tx) error {
		return connectors.ReserveIdentity(ctx, tx, telegram.Kind, account, te.conn.ID)
	}))
	// Many other retired platform bots, retired later, are held by connections too: every one is considered.
	for i := range 25 {
		token := fmt.Sprintf("%d:%s", 600000+i, strings.Repeat("E", 35))
		other, err := te.store.Save(ctx, connectors.Connection{TenantID: te.tenant, Kind: telegram.Kind, Name: fmt.Sprintf("retired_%d", i), DisplayName: "Retired"},
			connectors.Credentials{"token": token})
		require.NoError(t, err)
		ext := te.tg.ExternalID(connectors.Credentials{"token": token})
		_, err = te.db.Exec(ctx, `INSERT INTO automation.retired_platform_identities (kind, external) VALUES ($1, $2)`, string(telegram.Kind), ext)
		require.NoError(t, err)
		require.NoError(t, pgx.BeginFunc(ctx, te.db, func(tx pgx.Tx) error {
			return connectors.ReserveIdentity(ctx, tx, telegram.Kind, ext, other.ID)
		}))
	}
	own := te.bot.Webhook()

	// The route runs on a pool of one connection: the recovery holds the account's lock on it, and reads nothing else
	// meanwhile.
	cfg := te.db.Config().Copy()
	cfg.MaxConns = 1
	single, err := pgxpool.NewWithConfig(ctx, cfg)
	require.NoError(t, err)
	t.Cleanup(single.Close)
	keys, err := connectors.NewKeyring("k1", map[string][]byte{"k1": []byte("0123456789abcdef0123456789abcdef")})
	require.NoError(t, err)
	platform := connectors.NewPlatformCredentials(te.dir, map[catalog.ConnectorKind]connectors.PlatformCredential{
		telegram.Kind: {Credentials: connectors.Credentials{"token": tgPlatformToken}}})
	singleStore := connectors.NewStore(single, keys, nil, platform, nil)
	singleTg := telegram.New(telegram.Config{PlatformToken: tgPlatformToken, PublicOrigin: te.srv.URL, Inbound: te.late, APIURL: te.api.URL(), Uploads: te.uploads, Connections: singleStore})
	te.hook.Store(engine.InboundHandler(engine.InboundConfig{
		Connections: singleStore, Registry: te.reg, Inbound: p.rt.Inbound(), DB: single, Connectors: oneConnector{singleTg}, Platform: platform,
	}))

	// The late platform request lands.
	platformRoute := te.srv.URL + "/automations/hooks/platform/" + string(telegram.Kind)
	te.bot.SetWebhook(platformRoute, telegram.WebhookSecret(tgToken, catalog.PlatformConnectionID(telegram.Kind)))
	u := te.bot.Update(700, tgAlice, "after the stray webhook")
	d, err := te.bot.Redeliver(u)
	require.NoError(t, err)
	assert.Equal(t, http.StatusServiceUnavailable, d.Status, "delivered again")
	assert.Equal(t, own, te.bot.Webhook(), "the webhook is the connection's again, with its own secret")
	p.pass()
	assert.Zero(t, te.count(`SELECT count(*) FROM automation.runs`), "nothing ran on the platform route")

	d, err = te.bot.Redeliver(u)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, d.Status)
	p.until(func() bool { return len(noteTexts(t, te.env)) == 1 }, "the next delivery reaches the connection and runs its graph")
	stat := single.Stat()
	assert.Equal(t, int32(1), stat.MaxConns(), "all of it on one connection")

	// On the platform route a request without the secret header is refused at once, and another bot's with a made-up
	// secret matches no account.
	stranger := te.api.Bot("444444:DDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDD", "stranger_bot")
	stranger.SetWebhook(platformRoute, "")
	d, err = stranger.Redeliver(stranger.Update(700, tgAlice, "no header"))
	require.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, d.Status)
	stranger.SetWebhook(platformRoute, "not-the-secret-not-the-secret-12")
	d, err = stranger.Redeliver(stranger.Update(700, tgAlice, "hi"))
	require.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, d.Status)
	assert.Equal(t, own, te.bot.Webhook())
}

// oneConnector is a directory of one connector.
type oneConnector struct{ c connectors.Connector }

func (o oneConnector) Register(connectors.Connector) error { return nil }
func (o oneConnector) List() []connectors.Connector        { return []connectors.Connector{o.c} }
func (o oneConnector) Get(k catalog.ConnectorKind) (connectors.Connector, bool) {
	return o.c, k == o.c.Kind()
}

type failingStart struct{ tgStart }

func (failingStart) Handle(context.Context, telegram.StartUpdate) (string, error) {
	return "", errors.New("the link store is down")
}

// A /start whose handler fails is a 5xx for Telegram, which redelivers the update.
func TestTelegramStartHandlerFailureIs5xx(t *testing.T) {
	te := newTgEnv(t)
	te.proc()
	te.provision()
	te.chat(800)
	require.NoError(t, te.tg.RegisterStart(&failingStart{}))
	d, err := te.platform.SendText(800, tgAlice, "/start u_abc")
	require.NoError(t, err)
	assert.Equal(t, http.StatusServiceUnavailable, d.Status)
}

// telegram.read resolves the tenant's only bot when its params leave the connection out, and listens on
// the conversation it names: a run begun in one chat waits in another, and a run of no conversation at all
// has a bot scope too. A message without text is not the reply.
func TestTelegramReadImplicitConnectionAndChat(t *testing.T) {
	te := newTgEnv(t)
	p := te.proc()
	te.provision()
	te.chat(700)
	te.chat(701)
	g := graphJSON("Relay", `{"id": "m", "event": "telegram.message_received.v1"}`,
		`{"id": "read", "type": "telegram.read", "params": {"chat_id": 701, "timeout": "1h"}},
		 {"id": "got", "type": "acme.note", "params": {"text": "got {{ nodes.read.output.event.text }} in {{ nodes.read.output.event.chatId }}"}},
		 {"id": "fin", "type": "end", "params": {}}`,
		`{"from": {"node": "m", "outcome": "fired"}, "to": "read"},
		 {"from": {"node": "read", "outcome": "received"}, "to": "got"}, {"from": {"node": "read", "outcome": "timeout"}, "to": "ignore"},
		 {"from": {"node": "got", "outcome": "done"}, "to": "fin"}, {"from": {"node": "got", "outcome": "failed"}, "to": "fin"}`)
	te.create(p, te.human(), g)
	te.ok(te.bot.SendText(700, tgAlice, "start in 700"))
	p.until(func() bool {
		return te.count(`SELECT count(*) FROM automation.waits WHERE status = 'open' AND kind = 'event' AND event_kind = 'telegram.message_received.v1'`) == 1
	}, "the read waits")
	var waitID string
	var runID uuid.UUID
	require.NoError(t, te.db.QueryRow(context.Background(), `SELECT wait_id, run_id FROM automation.waits WHERE status = 'open' AND kind = 'event' AND event_kind = 'telegram.message_received.v1'`).Scan(&waitID, &runID))
	stillOpen := func() int {
		return te.count(`SELECT count(*) FROM automation.waits WHERE wait_id = $1 AND status = 'open'`, waitID)
	}
	fired := func() int {
		return te.count(`SELECT count(*) FROM automation.waits WHERE wait_id = $1 AND status = 'fired'`, waitID)
	}
	// The chat the run began in is not the chat it waits in; a message in 700 does not answer it (it
	// starts another run, which waits in 701 as well).
	before := te.messages()
	te.ok(te.bot.SendText(700, tgAlice, "wrong chat"))
	// A photo without a caption in the awaited chat is not a reply either.
	te.ok(te.bot.SendPhoto(701, tgAlice, "", []byte("JPEG")))
	assert.Equal(t, before+2, te.messages(), "both messages reached the outbox, and neither answered the read")
	p.pass()
	p.pass()
	assert.Empty(t, noteTexts(t, te.env), "neither the other chat nor a message without text answers the read")
	assert.Equal(t, 1, stillOpen(), "the original wait is still open after the negative deliveries")
	te.ok(te.bot.SendText(701, tgAlice, "the reply"))
	p.until(func() bool { return fired() == 1 && te.status(p, runID) == engine.RunDone }, "a text in the awaited chat fires the original wait, and its run finishes")
	assert.Contains(t, noteTexts(t, te.env), "got the reply in 701")
	assert.Zero(t, stillOpen(), "and it was fired, not cancelled or timed out")
	// The effect is the original run's own: its "got" node wrote it (not another run's, and not the failed branch).
	var written []string
	rows, err := te.db.Query(context.Background(), `SELECT text FROM test_effects WHERE run_id = $1 AND node_id = 'got'`, runID)
	require.NoError(t, err)
	written, err = pgx.CollectRows(rows, pgx.RowTo[string])
	require.NoError(t, err)
	assert.Equal(t, []string{"got the reply in 701"}, written, "the original run's got node wrote the reply")
}

// A run of no conversation (an order was created) waits for a message in a chat of the tenant's bot.
func TestTelegramReadOfARunWithoutAConversation(t *testing.T) {
	te := newTgEnv(t)
	p := te.proc()
	te.provision()
	te.chat(701)
	g := graphJSON("Order dialog", `{"id": "created", "event": "acme.order.created.v1"}`,
		`{"id": "read", "type": "telegram.read", "params": {"chat_id": 701, "timeout": "1h"}},
		 {"id": "got", "type": "acme.note", "params": {"text": "got {{ nodes.read.output.event.text }}"}},
		 {"id": "fin", "type": "end", "params": {}}`,
		`{"from": {"node": "created", "outcome": "fired"}, "to": "read"},
		 {"from": {"node": "read", "outcome": "received"}, "to": "got"}, {"from": {"node": "read", "outcome": "timeout"}, "to": "ignore"},
		 {"from": {"node": "got", "outcome": "done"}, "to": "fin"}, {"from": {"node": "got", "outcome": "failed"}, "to": "fin"}`)
	te.create(p, te.human(), g)
	te.publish(te.tenant, orderCreated, "o-1", order{}, 0)
	p.until(func() bool {
		return te.count(`SELECT count(*) FROM automation.waits WHERE status = 'open' AND kind = 'event' AND subject_key = $1`, te.conn.ID.String()+":701") == 1
	}, "the wait is on the bot's conversation")
	te.ok(te.bot.SendText(701, tgAlice, "hello order"))
	p.until(func() bool { return len(noteTexts(t, te.env)) == 1 }, "the message wakes the run")
	assert.Equal(t, []string{"got hello order"}, noteTexts(t, te.env))
}

// A read that cannot name the conversation it waits in (a chat that an expression gives as @username, which has no
// id to key by; a literal @username is refused when the graph is checked) fails; it never falls back to the
// conversation of the run, where another chat's message could answer.
func TestTelegramReadThatCannotBeKeyedFailsInsteadOfFallingBack(t *testing.T) {
	te := newTgEnv(t)
	p := te.proc()
	te.provision()
	te.chat(700)
	g := graphJSON("Dialog", `{"id": "m", "event": "telegram.message_received.v1"}`,
		`{"id": "read", "type": "telegram.read", "params": {"chat_id": "\"@sales\" + \"team\"", "timeout": "1h"}},
		 {"id": "fin", "type": "end", "params": {}}`,
		`{"from": {"node": "m", "outcome": "fired"}, "to": "read"},
		 {"from": {"node": "read", "outcome": "received"}, "to": "fin"}, {"from": {"node": "read", "outcome": "timeout"}, "to": "fin"}`)
	te.create(p, te.human(), g)
	te.ok(te.bot.SendText(700, tgAlice, "start"))
	p.until(func() bool {
		runs := te.runs(p, te.human(), engine.RunFilter{})
		return len(runs) == 1 && te.status(p, runs[0].RunID) == engine.RunFailed
	}, "the run fails")
	assert.Zero(t, te.count(`SELECT count(*) FROM automation.waits WHERE status = 'open' AND kind = 'event'`), "no wait was left listening")
}

// A follow-up of an accepted press that cannot be completed (Telegram keeps failing) is answered 503: the run
// moved on exactly once, and the redelivered press takes the keyboard off.
func TestTelegramAKeyboardThatStaysIsRedelivered(t *testing.T) {
	te := newTgEnv(t)
	p := te.proc()
	te.provision()
	te.chat(700)
	te.create(p, te.human(), tgAskGraph)
	te.publish(te.tenant, orderCreated, "o-1", order{Note: "#7"}, 0)
	p.until(func() bool {
		return len(te.bot.Sent()) == 1 && te.count(`SELECT count(*) FROM automation.waits WHERE status = 'open' AND correlation IS NOT NULL`) == 1
	}, "the question is sent")
	q := te.bot.Sent()[0]
	te.bot.Fail(telegramtest.Failure{Method: "editMessageText", Code: 502, Description: "Bad Gateway", Times: 3})
	te.bot.Fail(telegramtest.Failure{Method: "editMessageReplyMarkup", Code: 502, Description: "Bad Gateway", Times: 3})
	update := te.bot.PressUpdate(q, tgAlice, q.Markup[0][0].CallbackData)
	d, err := te.bot.Redeliver(update)
	require.NoError(t, err)
	assert.Equal(t, http.StatusServiceUnavailable, d.Status, "Telegram is asked to deliver the press again")
	p.until(func() bool { return len(noteTexts(t, te.env)) == 1 }, "the answer moved the run")
	d, err = te.bot.Redeliver(update)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, d.Status)
	got, found := te.bot.Message(700, q.ID)
	require.True(t, found, "the original message is there")
	assert.Empty(t, got.Markup, "the redelivery took the keyboard off")
	p.pass()
	assert.Len(t, noteTexts(t, te.env), 1, "and moved nothing twice")
}
