package engine_test

import (
	"context"
	"encoding/json"
	"errors"
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
	state, err := te.tg.Provision(ctx, te.conn, connectors.Credentials{"token": tgToken}, []string{telegram.WarnPollingUnknown}, nil)
	require.NoError(te.t, err)
	// The secret Provision generated is kept in the connection's inbound state, as the service does.
	require.NoError(te.t, te.store.UpdateInbound(ctx, te.tenant, te.conn.ID, func(cur json.RawMessage) (json.RawMessage, error) {
		return connectors.MergeInbound(cur, state)
	}))
	require.NoError(te.t, te.tg.StartPlatform(ctx, connectors.Credentials{"token": tgPlatformToken}))
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
	after, _ := te.bot.Message(700, q.ID)
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
	final, _ := te.bot.Message(700, q.ID)
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
	same, _ := te.bot.Message(700, q.ID)
	assert.Len(t, same.Markup, 2, "the timeout does not change the message")
	assert.Equal(t, q.Text, same.Text)

	_, query, err := te.bot.PressData(700, q.ID, tgAlice, q.Markup[0][0].CallbackData, q)
	require.NoError(t, err)
	text, _ := te.bot.Answered(query)
	assert.Equal(t, telegram.AlreadyAnswered, text)
	late, _ := te.bot.Message(700, q.ID)
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
	_, err = te.bot.SendText(700, tgAlice, "/help me")
	require.NoError(t, err)
	_, err = te.bot.SendText(700, tgAlice, "plain")
	require.NoError(t, err)
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
		_, err := te.bot.SendText(700, tgAlice, text)
		require.NoError(t, err)
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
	_, err := te.bot.SendPhoto(700, tgAlice, "", []byte("JPEGDATA"))
	require.NoError(t, err)
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
	started := time.Now()
	p.until(func() bool { return len(noteTexts(t, te.env)) == 1 }, "the send is repeated")
	assert.GreaterOrEqual(t, time.Since(started), time.Second, "after retry_after")
	assert.Len(t, te.bot.Sent(), 1, "one message was sent")
	assert.Len(t, te.bot.CallsOf("sendMessage"), 2, "the refused try and the repeat")
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
	_, err := te.bot.SendText(700, tgAlice, "start")
	require.NoError(t, err)
	p.until(func() bool { return len(te.bot.Sent()) == 1 }, "the question is sent")
	q := te.bot.Sent()[0]
	_, _, err = te.bot.PressData(700, q.ID, tgAlice, q.Markup[0][0].CallbackData, q)
	require.NoError(t, err)
	p.until(func() bool {
		return te.count(`SELECT count(*) FROM automation.waits WHERE status = 'open' AND kind = 'event' AND event_kind = 'telegram.message_received.v1'`) == 1
	}, "the read waits for a message")
	bob := telegramtest.User{ID: 502, Username: "bob", FirstName: "Bob"}
	_, err = te.bot.SendText(700, bob, "from the wrong person")
	require.NoError(t, err)
	p.pass()
	p.pass()
	assert.Empty(t, noteTexts(t, te.env), "another user's message does not wake the read")
	_, err = te.bot.SendText(700, tgAlice, "my answer")
	require.NoError(t, err)
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
	_, err := te.platform.SendText(800, tgAlice, "/start u_abc")
	require.NoError(t, err)
	_, err = te.platform.SendText(800, tgAlice, "/start zz_abc")
	require.NoError(t, err)
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
	p.until(func() bool { return len(te.platform.Sent()) == 1 }, "the question goes out through the platform bot")
	q := te.platform.Sent()[0]
	assert.Equal(t, 1, te.count(`SELECT count(*) FROM automation.connector_usage WHERE connection_id = 'platform:telegram_bot'`), "counted for the platform")
	d, _, err := te.platform.PressData(800, q.ID, tgAlice, q.Markup[0][0].CallbackData, q)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, d.Status)
	p.until(func() bool { return len(noteTexts(t, te.env)) == 1 }, "the press reached the tenant's run")
	assert.Equal(t, []string{"approved by alice as Approve #9 (approve)"}, noteTexts(t, te.env))
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
	_, err := te.bot.SendText(700, tgAlice, "start in 700")
	require.NoError(t, err)
	p.until(func() bool {
		return te.count(`SELECT count(*) FROM automation.waits WHERE status = 'open' AND kind = 'event' AND event_kind = 'telegram.message_received.v1'`) == 1
	}, "the read waits")
	// The chat the run began in is not the chat it waits in; a message in 700 does not answer it (it
	// starts another run, which waits in 701 as well).
	_, err = te.bot.SendText(700, tgAlice, "wrong chat")
	require.NoError(t, err)
	// A photo without a caption in the awaited chat is not a reply either.
	_, err = te.bot.SendPhoto(701, tgAlice, "", []byte("JPEG"))
	require.NoError(t, err)
	p.pass()
	p.pass()
	assert.Empty(t, noteTexts(t, te.env), "neither the other chat nor a message without text answers the read")
	_, err = te.bot.SendText(701, tgAlice, "the reply")
	require.NoError(t, err)
	p.until(func() bool { return len(noteTexts(t, te.env)) >= 1 }, "a text in the awaited chat wakes the read")
	assert.Contains(t, noteTexts(t, te.env), "got the reply in 701")
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
	_, err := te.bot.SendText(701, tgAlice, "hello order")
	require.NoError(t, err)
	p.until(func() bool { return len(noteTexts(t, te.env)) == 1 }, "the message wakes the run")
	assert.Equal(t, []string{"got hello order"}, noteTexts(t, te.env))
}

// A read that cannot name the conversation it waits in (a chat given by @username, which has no id to key
// by) fails; it never falls back to the conversation of the run, where another chat's message could answer.
func TestTelegramReadThatCannotBeKeyedFailsInsteadOfFallingBack(t *testing.T) {
	te := newTgEnv(t)
	p := te.proc()
	te.provision()
	te.chat(700)
	g := graphJSON("Dialog", `{"id": "m", "event": "telegram.message_received.v1"}`,
		`{"id": "read", "type": "telegram.read", "params": {"chat_id": "\"@salesteam\"", "timeout": "1h"}},
		 {"id": "fin", "type": "end", "params": {}}`,
		`{"from": {"node": "m", "outcome": "fired"}, "to": "read"},
		 {"from": {"node": "read", "outcome": "received"}, "to": "fin"}, {"from": {"node": "read", "outcome": "timeout"}, "to": "fin"}`)
	te.create(p, te.human(), g)
	_, err := te.bot.SendText(700, tgAlice, "start")
	require.NoError(t, err)
	p.until(func() bool {
		runs := te.runs(p, te.human(), engine.RunFilter{})
		return len(runs) == 1 && te.status(p, runs[0].RunID) == engine.RunFailed
	}, "the run fails")
	assert.Zero(t, te.count(`SELECT count(*) FROM automation.waits WHERE status = 'open' AND kind = 'event'`), "no wait was left listening")
}
