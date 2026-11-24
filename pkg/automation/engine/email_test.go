package engine_test

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/pkg/automation/catalog"
	"github.com/iota-uz/iota-sdk/pkg/automation/connectors"
	"github.com/iota-uz/iota-sdk/pkg/automation/connectors/email"
	"github.com/iota-uz/iota-sdk/pkg/automation/connectors/email/emailtest"
	"github.com/iota-uz/iota-sdk/pkg/automation/engine"
)

// The email connectors in the engine: a real database, the fake mail server, the engine's poller and the
// outbox that wakes waits.

const (
	mailUser = "robot@example.com"
	mailPass = "app-password-1"
)

type emailUploads struct {
	mu    sync.Mutex
	files []email.UploadFile
}

func (u *emailUploads) Save(_ context.Context, _ uuid.UUID, f email.UploadFile) (email.Stored, error) {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.files = append(u.files, f)
	id := fmt.Sprint(len(u.files))
	return email.Stored{ID: id, URL: "/uploads/" + id}, nil
}

type emailEnv struct {
	*env
	srv      *emailtest.Server
	store    connectors.Store
	dir      connectors.Directory
	smtpConn connectors.Connection
	imapConn connectors.Connection
	now      time.Time
}

func newEmailEnv(t *testing.T) *emailEnv {
	t.Helper()
	srv := emailtest.New(t, emailtest.ModeTLS, emailtest.Account{User: mailUser, Password: mailPass})
	keys, err := connectors.NewKeyring("k1", map[string][]byte{"k1": []byte("0123456789abcdef0123456789abcdef")})
	require.NoError(t, err)
	ee := &emailEnv{srv: srv, now: time.Now()}
	ee.env = newEnvWith(t, func(db *pgxpool.Pool) catalog.Availability {
		ee.store = connectors.NewStore(db, keys, nil, nil, nil)
		return ee.store
	})
	ee.dir = connectors.NewDirectory(ee.reg)
	for _, c := range email.New(email.Config{TLSConfig: srv.TLS, Uploads: &emailUploads{}, Connections: ee.store}) {
		require.NoError(t, ee.dir.Register(c))
	}
	_, smtpPort := emailtest.HostPort(srv.SMTPAddr)
	_, imapPort := emailtest.HostPort(srv.IMAPAddr)
	ctx := context.Background()
	ee.smtpConn, err = ee.store.Save(ctx, connectors.Connection{TenantID: ee.tenant, Kind: email.KindSMTP, Name: "smtp", DisplayName: "Mail"},
		connectors.Credentials{"preset": "custom", "host": "127.0.0.1", "port": smtpPort, "security": "tls", "username": mailUser, "password": mailPass, "from_address": mailUser})
	require.NoError(t, err)
	ee.imapConn, err = ee.store.Save(ctx, connectors.Connection{TenantID: ee.tenant, Kind: email.KindIMAP, Name: "imap", DisplayName: "Mail", Identity: connectors.Identity{Display: mailUser, External: mailUser, Kind: connectors.IdentityEmail}},
		connectors.Credentials{"preset": "custom", "host": "127.0.0.1", "port": imapPort, "security": "tls", "username": mailUser, "password": mailPass})
	require.NoError(t, err)
	return ee
}

func (ee *emailEnv) proc() *proc {
	return ee.env.proc(func(c *engine.Config) {
		c.Credentials = ee.store
		c.Usage = connectors.NewUsageRecorder(ee.db)
		c.MatchHooks = email.MatchHooks()
		c.CallLimits.BackoffBase = 10 * time.Millisecond
	})
}

// poll polls the mailbox once, a minute after the previous poll.
func (ee *emailEnv) poll(t *testing.T, p *proc, poller *engine.InboundPoller) {
	t.Helper()
	ee.now = ee.now.Add(61 * time.Second)
	_, err := poller.PollOnce(context.Background())
	require.NoError(t, err)
}

func (ee *emailEnv) poller(p *proc) *engine.InboundPoller {
	return p.rt.NewPoller(engine.PollerConfig{Connections: ee.store, Connectors: ee.dir, Clock: func() time.Time { return ee.now }})
}

func reply(id, from, subject, body string, headers ...string) []byte {
	h := append([]string{"From: " + from, "To: " + mailUser, "Subject: " + subject, "Message-ID: " + id, "MIME-Version: 1.0", "Content-Type: text/plain; charset=utf-8"}, headers...)
	return []byte(strings.Join(h, "\r\n") + "\r\n\r\n" + body + "\r\n")
}

const sendWaitGraph = `{"name": "Ask by mail", "triggers": [{"id": "created", "event": "acme.order.created.v1"}],
 "nodes": [
  {"id": "send", "type": "email.send", "params": {"to": ["boss@example.org"], "subject": "Order {{ event.note }}", "text": "Please confirm {{ event.note }}"}},
  {"id": "wait", "type": "email.wait_reply", "params": {"message_id": "nodes.send.output.message_id", "timeout": "1h"}},
  {"id": "got", "type": "acme.note", "params": {"text": "reply from {{ nodes.wait.output.event.from.address }}: {{ nodes.wait.output.event.text }}"}},
  {"id": "late", "type": "acme.note", "params": {"text": "no reply"}},
  {"id": "bad", "type": "acme.note", "params": {"text": "send failed {{ nodes.send.output.error.code }}"}},
  {"id": "fin", "type": "end", "params": {}}
 ],
 "edges": [
  {"from": {"node": "created", "outcome": "fired"}, "to": "send"},
  {"from": {"node": "send", "outcome": "sent"}, "to": "wait"},
  {"from": {"node": "send", "outcome": "failed"}, "to": "bad"},
  {"from": {"node": "wait", "outcome": "replied"}, "to": "got"},
  {"from": {"node": "wait", "outcome": "timeout"}, "to": "late"},
  {"from": {"node": "got", "outcome": "done"}, "to": "fin"}, {"from": {"node": "got", "outcome": "failed"}, "to": "fin"},
  {"from": {"node": "late", "outcome": "done"}, "to": "fin"}, {"from": {"node": "late", "outcome": "failed"}, "to": "fin"},
  {"from": {"node": "bad", "outcome": "done"}, "to": "fin"}, {"from": {"node": "bad", "outcome": "failed"}, "to": "fin"}
 ]}`

var messageIDHeader = regexp.MustCompile(`(?mi)^Message-ID: (<[^>]+>)\r?$`)

// Acceptance: email.send then email.wait_reply. A reply from the user's mail client (with References) wakes
// exactly this run; a reply to another message of the mailbox wakes nobody.
func TestEmailReplyWakesTheRunThatSentTheMessage(t *testing.T) {
	ee := newEmailEnv(t)
	p := ee.proc()
	ee.create(p, ee.human(), sendWaitGraph)
	poller := ee.poller(p)
	ee.poll(t, p, poller) // the mailbox is known before anything is sent: its cursor is set

	ee.publish(ee.tenant, orderCreated, "o-1", order{Note: "#42"}, 0)
	ee.publish(ee.tenant, orderCreated, "o-2", order{Note: "#43"}, 0)
	p.until(func() bool {
		return len(ee.srv.Sent()) == 2 && ee.count(`SELECT count(*) FROM automation.waits WHERE status = 'open' AND kind = 'event'`) == 2
	}, "both runs sent a message and wait for its reply")
	ids := map[string]string{}
	for _, s := range ee.srv.Sent() {
		m := messageIDHeader.FindStringSubmatch(string(s.Data))
		require.NotNil(t, m, "%s", s.Data)
		subject := regexp.MustCompile(`(?m)^Subject: (.*)$`).FindStringSubmatch(string(s.Data))[1]
		ids[strings.TrimSpace(subject)] = m[1]
	}
	require.Len(t, ids, 2)
	assert.Equal(t, 2, ee.count(`SELECT count(*) FROM automation.connector_usage WHERE kind = 'smtp' AND unit = 'message' AND connection_id = $1`, ee.smtpConn.ID.String()))

	// The user answers the first message from a mail client: In-Reply-To and References name it.
	ee.srv.Deliver(mailUser, "INBOX", reply("<reply-1@mail.gmail.example>", "Boss <boss@example.org>", "Re: Order #42", "Confirmed.",
		"In-Reply-To: "+ids["Order #42"], "References: "+ids["Order #42"]))
	// A reply to some other message of the mailbox, and a new message of no thread.
	ee.srv.Deliver(mailUser, "INBOX", reply("<other-1@mail.gmail.example>", "boss@example.org", "Re: Lunch", "Yes.",
		"In-Reply-To: <lunch@example.org>", "References: <lunch@example.org>"))
	ee.poll(t, p, poller)

	p.until(func() bool { return len(noteTexts(t, ee.env)) == 1 }, "the reply woke its run")
	assert.Equal(t, []string{"reply from boss@example.org: Confirmed."}, noteTexts(t, ee.env))
	for i := 0; i < 5; i++ {
		p.pass()
	}
	assert.Equal(t, 1, ee.count(`SELECT count(*) FROM automation.waits WHERE status = 'open' AND kind = 'event'`), "the other run still waits: only its own reply wakes it")

	ee.srv.Deliver(mailUser, "INBOX", reply("<reply-2@mail.gmail.example>", "boss@example.org", "Re: Order #43", "Also confirmed.",
		"In-Reply-To: "+ids["Order #43"], "References: "+ids["Order #43"]))
	ee.poll(t, p, poller)
	p.until(func() bool { return len(noteTexts(t, ee.env)) == 2 }, "the second reply woke the second run")
	assert.Equal(t, []string{"reply from boss@example.org: Also confirmed.", "reply from boss@example.org: Confirmed."}, noteTexts(t, ee.env))
}

func TestEmailReplyFromTheWrongSenderDoesNotWake(t *testing.T) {
	ee := newEmailEnv(t)
	p := ee.proc()
	ee.create(p, ee.human(), strings.Replace(sendWaitGraph, `"timeout": "1h"`, `"timeout": "1h", "from": "Boss@Example.org"`, 1))
	poller := ee.poller(p)
	ee.poll(t, p, poller)
	ee.publish(ee.tenant, orderCreated, "o-1", order{Note: "#1"}, 0)
	p.until(func() bool {
		return len(ee.srv.Sent()) == 1 && ee.count(`SELECT count(*) FROM automation.waits WHERE status = 'open' AND kind = 'event'`) == 1
	}, "waiting")
	id := messageIDHeader.FindStringSubmatch(string(ee.srv.Sent()[0].Data))[1]
	ee.srv.Deliver(mailUser, "INBOX", reply("<x-1@mail>", "intern@example.org", "Re: Order #1", "Looks fine to me.", "In-Reply-To: "+id, "References: "+id))
	ee.poll(t, p, poller)
	for i := 0; i < 5; i++ {
		p.pass()
	}
	assert.Empty(t, noteTexts(t, ee.env), "a reply from another address does not wake a wait that names its sender")
	ee.srv.Deliver(mailUser, "INBOX", reply("<x-2@mail>", "BOSS@example.org", "Re: Order #1", "Confirmed.", "In-Reply-To: "+id, "References: "+id))
	ee.poll(t, p, poller)
	p.until(func() bool { return len(noteTexts(t, ee.env)) == 1 }, "the named sender's reply wakes it, whatever its case")
}

func TestEmailWaitTimesOut(t *testing.T) {
	ee := newEmailEnv(t)
	p := ee.proc()
	ee.create(p, ee.human(), strings.Replace(sendWaitGraph, `"timeout": "1h"`, `"timeout": "5s"`, 1))
	ee.publish(ee.tenant, orderCreated, "o-1", order{Note: "#1"}, 0)
	p.until(func() bool { return len(noteTexts(t, ee.env)) == 1 }, "the wait timed out")
	assert.Equal(t, []string{"no reply"}, noteTexts(t, ee.env))
}

// Acceptance: a drop after the message was handed over is failed with outcome_unknown; it is not sent
// again.
func TestEmailDropAfterDataIsOutcomeUnknownAndNotSentAgain(t *testing.T) {
	ee := newEmailEnv(t)
	p := ee.proc()
	ee.create(p, ee.human(), sendWaitGraph)
	ee.srv.DropAfterData(true)
	ee.publish(ee.tenant, orderCreated, "o-1", order{Note: "#1"}, 0)
	p.until(func() bool { return len(noteTexts(t, ee.env)) == 1 }, "the send failed")
	assert.Equal(t, []string{"send failed outcome_unknown"}, noteTexts(t, ee.env))
	for i := 0; i < 10; i++ {
		p.pass()
	}
	assert.Len(t, ee.srv.Sent(), 1, "no second message")
}

func TestEmailSendFailsWithUnavailableOn4xxAndIsNotRepeated(t *testing.T) {
	ee := newEmailEnv(t)
	p := ee.proc()
	ee.create(p, ee.human(), sendWaitGraph)
	ee.srv.RejectRecipient("boss@example.org", 451)
	ee.publish(ee.tenant, orderCreated, "o-1", order{Note: "#1"}, 0)
	p.until(func() bool { return len(noteTexts(t, ee.env)) == 1 }, "the send failed")
	assert.Equal(t, []string{"send failed unavailable"}, noteTexts(t, ee.env))
	ee.srv.RejectRecipient("boss@example.org", 0)
	for i := 0; i < 10; i++ {
		p.pass()
	}
	assert.Empty(t, ee.srv.Sent(), "a non-idempotent send is not repeated by the engine")
}

const receivedGraph = `{"name": "Inbox", "triggers": [{"id": "mail", "event": "email.received.v1" %s}],
 "nodes": [
  {"id": "note", "type": "acme.note", "params": {"text": "{{ event.subject }}|{{ event.from.address }}"}},
  {"id": "fin", "type": "end", "params": {}}
 ],
 "edges": [{"from": {"node": "mail", "outcome": "fired"}, "to": "note"}, {"from": {"node": "note", "outcome": "done"}, "to": "fin"}, {"from": {"node": "note", "outcome": "failed"}, "to": "fin"}]}`

// Acceptance: after the mailbox's UIDVALIDITY changes the old messages start no graph; a message that
// arrives after that does.
func TestEmailUIDValidityChangeStartsNoGraphForOldMail(t *testing.T) {
	ee := newEmailEnv(t)
	p := ee.proc()
	ee.srv.Deliver(mailUser, "INBOX", reply("<before@x>", "ann@example.org", "From before the connection", "x"))
	ee.create(p, ee.human(), fmt.Sprintf(receivedGraph, ""))
	poller := ee.poller(p)
	ee.poll(t, p, poller)
	for i := 0; i < 3; i++ {
		p.pass()
	}
	assert.Empty(t, noteTexts(t, ee.env), "mail from before the connection starts nothing")

	ee.srv.Deliver(mailUser, "INBOX", reply("<a@x>", "ann@example.org", "First", "x"))
	ee.poll(t, p, poller)
	p.until(func() bool { return len(noteTexts(t, ee.env)) == 1 }, "a new message starts the graph")

	ee.srv.ResetMailbox(mailUser, "INBOX")
	ee.srv.Deliver(mailUser, "INBOX", reply("<b@x>", "ann@example.org", "After the rebuild, before the poll", "x"))
	ee.poll(t, p, poller)
	for i := 0; i < 5; i++ {
		p.pass()
	}
	assert.Equal(t, []string{"First|ann@example.org"}, noteTexts(t, ee.env), "the messages of the rebuilt mailbox are not published")

	ee.srv.Deliver(mailUser, "INBOX", reply("<c@x>", "ann@example.org", "Third", "x"))
	ee.poll(t, p, poller)
	p.until(func() bool { return len(noteTexts(t, ee.env)) == 2 }, "a message after the rebuild starts it")
	assert.Equal(t, []string{"First|ann@example.org", "Third|ann@example.org"}, noteTexts(t, ee.env))
}

// Acceptance: a reply in a thread and a mailing list message do not start a trigger with the default
// options; the options turn the skips off, and from and subject_contains narrow the trigger.
func TestEmailTriggerSkipsRepliesAndAutomaticMailByDefault(t *testing.T) {
	ee := newEmailEnv(t)
	p := ee.proc()
	ee.create(p, ee.human(), fmt.Sprintf(receivedGraph, ""))
	poller := ee.poller(p)
	ee.poll(t, p, poller)
	ee.srv.Deliver(mailUser, "INBOX", reply("<n1@x>", "ann@example.org", "Plain", "x"))
	ee.srv.Deliver(mailUser, "INBOX", reply("<n2@x>", "ann@example.org", "Re: Plain", "x"))
	ee.srv.Deliver(mailUser, "INBOX", reply("<n3@x>", "ann@example.org", "Ответ: Plain", "x"))
	ee.srv.Deliver(mailUser, "INBOX", reply("<n4@x>", "ann@example.org", "A threaded answer", "x", "In-Reply-To: <n1@x>"))
	ee.srv.Deliver(mailUser, "INBOX", reply("<n5@x>", "news@example.org", "Weekly digest", "x", "List-Id: <news.example.org>"))
	ee.srv.Deliver(mailUser, "INBOX", reply("<n6@x>", "noreply@example.org", "Out of office", "x", "Auto-Submitted: auto-replied"))
	ee.srv.Deliver(mailUser, "INBOX", reply("<n7@x>", "promo@example.org", "Sale", "x", "Precedence: bulk"))
	ee.poll(t, p, poller)
	p.until(func() bool { return len(noteTexts(t, ee.env)) == 1 }, "only the plain message starts the graph")
	for i := 0; i < 5; i++ {
		p.pass()
	}
	assert.Equal(t, []string{"Plain|ann@example.org"}, noteTexts(t, ee.env))
}

func TestEmailTriggerOptions(t *testing.T) {
	ee := newEmailEnv(t)
	p := ee.proc()
	ee.create(p, ee.human(), fmt.Sprintf(receivedGraph, `, "params": {"skip_replies": false, "skip_auto": false, "from": ["ANN@example.org"], "subject_contains": "invoice"}`))
	poller := ee.poller(p)
	ee.poll(t, p, poller)
	ee.srv.Deliver(mailUser, "INBOX", reply("<o1@x>", "Ann <ann@example.org>", "Re: Your INVOICE", "x"))
	ee.srv.Deliver(mailUser, "INBOX", reply("<o2@x>", "bob@example.org", "Invoice", "x"))
	ee.srv.Deliver(mailUser, "INBOX", reply("<o3@x>", "ann@example.org", "Hello", "x"))
	ee.srv.Deliver(mailUser, "INBOX", reply("<o4@x>", "ann@example.org", "Invoice list", "x", "List-Id: <l.example.org>"))
	ee.poll(t, p, poller)
	p.until(func() bool { return len(noteTexts(t, ee.env)) == 2 }, "the replies and lists are let in; the sender and subject narrow")
	assert.Equal(t, []string{"Invoice list|ann@example.org", "Re: Your INVOICE|ann@example.org"}, noteTexts(t, ee.env))
}

// A refused login stops the polling: the connection goes to needs_reauth.
func TestEmailRefusedLoginStopsPolling(t *testing.T) {
	ee := newEmailEnv(t)
	p := ee.proc()
	ee.create(p, ee.human(), fmt.Sprintf(receivedGraph, ""))
	poller := ee.poller(p)
	ee.poll(t, p, poller)
	// The password was changed at the provider.
	_, creds, err := ee.store.Get(context.Background(), ee.tenant, ee.imapConn.ID)
	require.NoError(t, err)
	creds["password"] = "changed-at-the-provider"
	c := ee.imapConn
	_, err = ee.store.Save(context.Background(), c, creds)
	require.NoError(t, err)
	ee.poll(t, p, poller)
	got, _, err := ee.store.Get(context.Background(), ee.tenant, ee.imapConn.ID)
	require.NoError(t, err)
	assert.Equal(t, connectors.StatusNeedsReauth, got.Status)
}

// A wait that names its connection listens on the conversation "<connection>:<Message-ID>" of the message it
// waits for the reply to.
func TestEmailWaitListensOnTheConversationOfItsMessage(t *testing.T) {
	ee := newEmailEnv(t)
	p := ee.proc()
	ee.create(p, ee.human(), strings.Replace(sendWaitGraph, `"timeout": "1h"`, fmt.Sprintf(`"timeout": "1h", "connection": %q`, ee.imapConn.ID.String()), 1))
	poller := ee.poller(p)
	ee.poll(t, p, poller)
	ee.publish(ee.tenant, orderCreated, "o-1", order{Note: "#1"}, 0)
	p.until(func() bool {
		return len(ee.srv.Sent()) == 1 && ee.count(`SELECT count(*) FROM automation.waits WHERE status = 'open' AND kind = 'event'`) == 1
	}, "waiting")
	id := messageIDHeader.FindStringSubmatch(string(ee.srv.Sent()[0].Data))[1]
	assert.Equal(t, 1, ee.count(`SELECT count(*) FROM automation.waits WHERE status = 'open' AND subject_entity = 'conversation' AND subject_key = $1`,
		email.ConversationKey(ee.imapConn.ID.String(), id)), "the wait is keyed by the connection and the Message-ID it sent")

	ee.srv.Deliver(mailUser, "INBOX", reply("<k-1@mail>", "boss@example.org", "Re: Order #1", "Confirmed.", "In-Reply-To: "+id, "References: "+id))
	ee.poll(t, p, poller)
	p.until(func() bool { return len(noteTexts(t, ee.env)) == 1 }, "the reply woke the run")
}

const threadedGraph = `{"name": "Answer in a thread", "triggers": [{"id": "created", "event": "acme.order.created.v1"}],
 "nodes": [
  {"id": "send", "type": "email.send", "params": {"to": ["boss@example.org"], "subject": "Order {{ event.note }}", "text": "for {{ event.note }}", "in_reply_to": "'<parent@example.org>'"}},
  {"id": "wait", "type": "email.wait_reply", "params": {"message_id": "nodes.send.output.message_id", "thread_id": "'<parent@example.org>'", "timeout": "1h"}},
  {"id": "got", "type": "acme.note", "params": {"text": "reply {{ nodes.wait.output.event.text }}"}},
  {"id": "fin", "type": "end", "params": {}}
 ],
 "edges": [
  {"from": {"node": "created", "outcome": "fired"}, "to": "send"},
  {"from": {"node": "send", "outcome": "sent"}, "to": "wait"}, {"from": {"node": "send", "outcome": "failed"}, "to": "fin"},
  {"from": {"node": "wait", "outcome": "replied"}, "to": "got"}, {"from": {"node": "wait", "outcome": "timeout"}, "to": "fin"},
  {"from": {"node": "got", "outcome": "done"}, "to": "fin"}, {"from": {"node": "got", "outcome": "failed"}, "to": "fin"}
 ]}`

// Two runs send in one conversation (both answer the same parent): a reply names the conversation's first
// message, and wakes only the run whose message it answers.
func TestEmailTwoRunsInOneConversationAreWokenByTheirOwnReply(t *testing.T) {
	ee := newEmailEnv(t)
	p := ee.proc()
	ee.create(p, ee.human(), threadedGraph)
	poller := ee.poller(p)
	ee.poll(t, p, poller)
	ee.publish(ee.tenant, orderCreated, "o-1", order{Note: "#1"}, 0)
	ee.publish(ee.tenant, orderCreated, "o-2", order{Note: "#2"}, 0)
	p.until(func() bool {
		return len(ee.srv.Sent()) == 2 && ee.count(`SELECT count(*) FROM automation.waits WHERE status = 'open' AND kind = 'event'`) == 2
	}, "two runs wait")
	ids := map[string]string{}
	for _, s := range ee.srv.Sent() {
		m := messageIDHeader.FindStringSubmatch(string(s.Data))
		require.NotNil(t, m)
		subject := regexp.MustCompile(`(?m)^Subject: (.*)$`).FindStringSubmatch(string(s.Data))[1]
		ids[strings.TrimSpace(subject)] = m[1]
		assert.Contains(t, string(s.Data), "In-Reply-To: <parent@example.org>")
	}
	require.Len(t, ids, 2)

	// A reply to the second message only: its References name the parent first, as a client's do.
	ee.srv.Deliver(mailUser, "INBOX", reply("<r2@mail>", "boss@example.org", "Re: Order #2", "answer to two",
		"In-Reply-To: "+ids["Order #2"], "References: <parent@example.org> "+ids["Order #1"]+" "+ids["Order #2"]))
	ee.poll(t, p, poller)
	p.until(func() bool { return len(noteTexts(t, ee.env)) == 1 }, "the reply woke a run")
	for i := 0; i < 5; i++ {
		p.pass()
	}
	assert.Equal(t, []string{"reply answer to two"}, noteTexts(t, ee.env), "one run, the one whose message was answered: the older message in References is history")
	assert.Equal(t, 1, ee.count(`SELECT count(*) FROM automation.waits WHERE status = 'open' AND kind = 'event'`), "the other still waits")

	ee.srv.Deliver(mailUser, "INBOX", reply("<r1@mail>", "boss@example.org", "Re: Order #1", "answer to one",
		"In-Reply-To: "+ids["Order #1"], "References: <parent@example.org> "+ids["Order #1"]))
	ee.poll(t, p, poller)
	p.until(func() bool { return len(noteTexts(t, ee.env)) == 2 }, "the first run is woken by its own reply")
}

// A graph that is enabled after the mailbox was connected starts from a cursor that was kept at the end of
// the mailbox: a reply that arrives before the graph's first poll is not skipped, and mail that was in the
// mailbox before the graph is not published.
func TestEmailCursorIsKeptWhileNoGraphUsesTheMailbox(t *testing.T) {
	ee := newEmailEnv(t)
	p := ee.proc()
	poller := ee.poller(p)
	ee.srv.Deliver(mailUser, "INBOX", reply("<before@x>", "ann@example.org", "Before the connection", "x"))
	ee.poll(t, p, poller) // no graph yet: the poller only keeps the cursor at the end of the mailbox
	ee.srv.Deliver(mailUser, "INBOX", reply("<idle@x>", "ann@example.org", "While no graph used it", "x"))
	ee.poll(t, p, poller)

	ee.create(p, ee.human(), sendWaitGraph)
	ee.publish(ee.tenant, orderCreated, "o-1", order{Note: "#1"}, 0)
	p.until(func() bool {
		return len(ee.srv.Sent()) == 1 && ee.count(`SELECT count(*) FROM automation.waits WHERE status = 'open' AND kind = 'event'`) == 1
	}, "the run sent and waits")
	id := messageIDHeader.FindStringSubmatch(string(ee.srv.Sent()[0].Data))[1]
	// The reply arrives before the first poll the graph's connection has had since the graph was enabled.
	ee.srv.Deliver(mailUser, "INBOX", reply("<fast@x>", "boss@example.org", "Re: Order #1", "Done.", "In-Reply-To: "+id, "References: "+id))
	ee.poll(t, p, poller)
	p.until(func() bool { return len(noteTexts(t, ee.env)) == 1 }, "the reply woke the run")
	assert.Equal(t, 1, ee.count(`SELECT count(*) FROM extruntime.outbox_events WHERE event = 'email.received.v1'`), "only the reply was published: not the mail from before the graph")
}

// A failed read in the middle of a batch publishes nothing and keeps the cursor: the engine reads the batch
// again at its next poll.
func TestEmailFetchFailureInABatchIsRetriedFromTheSameCursor(t *testing.T) {
	ee := newEmailEnv(t)
	p := ee.proc()
	ee.create(p, ee.human(), fmt.Sprintf(receivedGraph, ""))
	poller := ee.poller(p)
	ee.poll(t, p, poller)
	for i := 1; i <= 3; i++ {
		ee.srv.Deliver(mailUser, "INBOX", reply(fmt.Sprintf("<f%d@x>", i), "ann@example.org", fmt.Sprintf("Mail %d", i), "x"))
	}
	ee.srv.FailBodyFetch(2)
	ee.poll(t, p, poller)
	for i := 0; i < 5; i++ {
		p.pass()
	}
	assert.Empty(t, noteTexts(t, ee.env), "the failed batch published nothing")
	ee.poll(t, p, poller)
	p.until(func() bool { return len(noteTexts(t, ee.env)) == 3 }, "the next poll read all three from the old cursor")
	assert.Equal(t, 3, ee.count(`SELECT count(*) FROM extruntime.outbox_events WHERE event = 'email.received.v1'`))
}

// A mailbox connected and used before the poller ever polled it: the message that is awaited seeds the
// cursor of its mailbox before it is sent, so a reply that arrives before the first poll is not jumped
// over, and mail from before the send is not published.
func TestEmailSendSeedsTheMailboxCursorBeforeTheFirstPoll(t *testing.T) {
	ee := newEmailEnv(t)
	p := ee.proc()
	ee.srv.Deliver(mailUser, "INBOX", reply("<old@x>", "ann@example.org", "From before", "x"))
	ee.create(p, ee.human(), sendWaitGraph)
	ee.publish(ee.tenant, orderCreated, "o-1", order{Note: "#1"}, 0) // no poll has happened yet
	p.until(func() bool {
		return len(ee.srv.Sent()) == 1 && ee.count(`SELECT count(*) FROM automation.waits WHERE status = 'open' AND kind = 'event'`) == 1
	}, "the run sent and waits")
	id := messageIDHeader.FindStringSubmatch(string(ee.srv.Sent()[0].Data))[1]
	ee.srv.Deliver(mailUser, "INBOX", reply("<quick@x>", "boss@example.org", "Re: Order #1", "Immediately.", "In-Reply-To: "+id, "References: "+id))
	poller := ee.poller(p)
	ee.poll(t, p, poller) // the first poll of the mailbox
	p.until(func() bool { return len(noteTexts(t, ee.env)) == 1 }, "the reply woke the run")
	assert.Equal(t, 1, ee.count(`SELECT count(*) FROM extruntime.outbox_events WHERE event = 'email.received.v1'`), "only the reply: not the mail from before")
}

// A baseline of an idle mailbox gives way to a graph enabled while it was taken: its cursor stays, and what
// arrived meanwhile is read by the poll of the graph that uses the mailbox now.
func TestEmailIdleBaselineGivesWayToAGraphEnabledMeanwhile(t *testing.T) {
	ee := newEmailEnv(t)
	p := ee.proc()
	slow := p.rt.NewPoller(engine.PollerConfig{Connections: ee.store, Connectors: ee.dir, Rescan: time.Hour, Clock: func() time.Time { return ee.now }})
	ee.now = ee.now.Add(61 * time.Second)
	_, err := slow.PollOnce(context.Background()) // idle: the cursor is kept at the end of the mailbox
	require.NoError(t, err)

	ee.srv.Deliver(mailUser, "INBOX", reply("<arrived@x>", "ann@example.org", "Arrived meanwhile", "x"))
	ee.create(p, ee.human(), fmt.Sprintf(receivedGraph, "")) // the graph is enabled: the mailbox is used now
	// The graph predates the next baseline by more than the clock tolerance of the fence: it is the scan the
	// target comes from that counts, not the moment the baseline began.
	time.Sleep(6 * time.Second)
	ee.now = ee.now.Add(61 * time.Second)
	_, err = slow.PollOnce(context.Background()) // the old scan still calls it idle
	require.NoError(t, err)
	for i := 0; i < 5; i++ {
		p.pass()
	}

	fresh := ee.poller(p) // a scan that knows the graph
	ee.poll(t, p, fresh)
	p.until(func() bool { return len(noteTexts(t, ee.env)) == 1 }, "the cursor did not jump over the message that arrived while the graph was enabled")
	assert.Equal(t, []string{"Arrived meanwhile|ann@example.org"}, noteTexts(t, ee.env))
}

const replyToGraph = `{"name": "Ask, answers to another mailbox", "triggers": [{"id": "created", "event": "acme.order.created.v1"}],
 "nodes": [
  {"id": "send", "type": "email.send", "params": {"to": ["boss@example.org"], "subject": "Order {{ event.note }}", "text": "Please confirm", "reply_to": "replies@example.com"}},
  {"id": "wait", "type": "email.wait_reply", "params": {"message_id": "nodes.send.output.message_id", "timeout": "1h"}},
  {"id": "got", "type": "acme.note", "params": {"text": "reply {{ nodes.wait.output.event.text }}"}},
  {"id": "fin", "type": "end", "params": {}}
 ],
 "edges": [
  {"from": {"node": "created", "outcome": "fired"}, "to": "send"},
  {"from": {"node": "send", "outcome": "sent"}, "to": "wait"}, {"from": {"node": "send", "outcome": "failed"}, "to": "fin"},
  {"from": {"node": "wait", "outcome": "replied"}, "to": "got"}, {"from": {"node": "wait", "outcome": "timeout"}, "to": "fin"},
  {"from": {"node": "got", "outcome": "done"}, "to": "fin"}, {"from": {"node": "got", "outcome": "failed"}, "to": "fin"}
 ]}`

// The reply goes to the Reply-To mailbox, not the sender's: that mailbox is seeded before the message is
// sent, so a reply before its first poll is not jumped over.
func TestEmailSendSeedsTheReplyToMailbox(t *testing.T) {
	ee := newEmailEnv(t)
	// The mailbox that receives is another address than the one that sends.
	_, creds, err := ee.store.Get(context.Background(), ee.tenant, ee.imapConn.ID)
	require.NoError(t, err)
	c := ee.imapConn
	c.Identity = connectors.Identity{Display: "replies@example.com", External: "replies@example.com", Kind: connectors.IdentityEmail}
	_, err = ee.store.Save(context.Background(), c, creds)
	require.NoError(t, err)
	p := ee.proc()
	ee.create(p, ee.human(), replyToGraph)
	ee.publish(ee.tenant, orderCreated, "o-1", order{Note: "#1"}, 0)
	p.until(func() bool {
		return len(ee.srv.Sent()) == 1 && ee.count(`SELECT count(*) FROM automation.waits WHERE status = 'open' AND kind = 'event'`) == 1
	}, "the run sent and waits")
	id := messageIDHeader.FindStringSubmatch(string(ee.srv.Sent()[0].Data))[1]
	ee.srv.Deliver(mailUser, "INBOX", reply("<rt@x>", "boss@example.org", "Re: Order #1", "To the other mailbox.", "In-Reply-To: "+id, "References: "+id))
	poller := ee.poller(p)
	ee.poll(t, p, poller) // the first poll of that mailbox
	p.until(func() bool { return len(noteTexts(t, ee.env)) == 1 }, "the reply woke the run")
}
