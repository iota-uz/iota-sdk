package email

import (
	"bytes"
	"context"
	"encoding/base64"
	"strings"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/pkg/automation/catalog"
	"github.com/iota-uz/iota-sdk/pkg/automation/connectors"
	"github.com/iota-uz/iota-sdk/pkg/automation/connectors/email/emailtest"
	"github.com/iota-uz/iota-sdk/pkg/automation/engine"
	"github.com/iota-uz/iota-sdk/pkg/automation/expr"
	"github.com/iota-uz/iota-sdk/pkg/automation/graph"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/smtpconfig"
)

// connected is an Availability with one connection of each kind.
type connected struct {
	refs map[catalog.ConnectorKind][]catalog.ConnectionRef
}

func (c connected) Connected(context.Context, uuid.UUID) (map[catalog.ConnectorKind][]catalog.ConnectionRef, error) {
	return c.refs, nil
}

func TestConnectorsRegisterAndTheCatalogOfATenantIsValid(t *testing.T) {
	avail := connected{refs: map[catalog.ConnectorKind][]catalog.ConnectionRef{
		KindSMTP: {{ID: uuid.NewString(), Name: "smtp"}}, KindIMAP: {{ID: uuid.NewString(), Name: "imap"}},
	}}
	reg := catalog.NewRegistry(avail)
	dir := connectors.NewDirectory(reg)
	for _, c := range New(Config{}) {
		require.NoError(t, dir.Register(c))
	}
	require.NoError(t, reg.RegisterTriggers(engine.Triggers()))
	view, err := reg.ForTenant(context.Background(), tenant)
	require.NoError(t, err)
	assert.Empty(t, view.Unavailable)
	for _, typ := range []string{NodeSend, NodeWaitReply, NodeLabel} {
		assert.Contains(t, view.Catalog.Nodes, typ)
	}
	assert.Contains(t, view.Catalog.Events, EventReceived)
	assert.Equal(t, []catalog.ConnectorKind{KindSMTP}, view.NodeRequires[NodeSend])
	assert.Equal(t, []catalog.ConnectorKind{KindIMAP}, view.NodeRequires[NodeWaitReply])
	assert.Equal(t, []catalog.ConnectorKind{KindIMAP}, view.NodeRequires[NodeLabel])
	assert.Equal(t, []catalog.ConnectorKind{KindIMAP}, view.EventRequires[EventReceived])
	assert.Empty(t, graph.ValidateCatalog(view.Catalog, nil), "the catalog is one a graph can be saved against")

	// The node types as the issue's contract has them.
	nodes := view.Catalog.Nodes
	assert.Equal(t, graph.BehaviorCall, nodes[NodeSend].Behavior)
	assert.Equal(t, []string{"sent", "failed"}, nodes[NodeSend].Outcomes)
	assert.Equal(t, graph.BehaviorWait, nodes[NodeWaitReply].Behavior)
	assert.Equal(t, EventReceived, nodes[NodeWaitReply].Wait.Event.Value)
	assert.Equal(t, "replied", nodes[NodeWaitReply].Wait.Matched)
	assert.Equal(t, "timeout", nodes[NodeWaitReply].Wait.TimeoutOutcome)
	assert.Equal(t, []string{"done", "not_found", "failed"}, nodes[NodeLabel].Outcomes)
	ev := view.Catalog.Events[EventReceived]
	assert.Equal(t, graph.SubjectConversation, ev.Subject)
	for _, f := range []string{"connectionId", "mailbox", "messageId", "inReplyTo", "references", "from", "to", "cc", "subject", "text", "hasHtml", "attachments", "receivedAt"} {
		_, ok := ev.Payload.Get(f)
		assert.True(t, ok, f)
	}
	for _, f := range []string{"html", "body"} {
		_, ok := ev.Payload.Get(f)
		assert.False(t, ok, "the HTML of a message is not in the payload: %s", f)
	}

	assert.False(t, (&sendNode{}).Idempotent())
	assert.True(t, (&labelNode{}).Idempotent())
}

func TestDescriptors(t *testing.T) {
	cs := New(Config{})
	smtp, imap := cs[0].Descriptor(), cs[1].Descriptor()
	assert.Equal(t, KindSMTP, smtp.Kind)
	assert.Equal(t, KindIMAP, imap.Kind)
	assert.Equal(t, connectors.PlatformAuto, smtp.Platform, "the application's SMTP is offered to every tenant")
	assert.Equal(t, connectors.PlatformNone, imap.Platform)
	assert.True(t, imap.Inbound)
	assert.False(t, smtp.Inbound)
	for _, d := range []connectors.Descriptor{smtp, imap} {
		assert.Equal(t, connectors.CategoryEmail, d.Category)
		assert.Equal(t, map[string]bool{"password": true, "access_token": true, "refresh_token": true}, connectors.SecretFields(d))
		g := d.OAuth2Presets[PresetGmail]
		assert.Equal(t, []string{"https://mail.google.com/", "openid", "email"}, g.Scopes)
		assert.Equal(t, GoogleClient, g.ClientCredential)
		m := d.OAuth2Presets[PresetM365]
		assert.Equal(t, MicrosoftClient, m.ClientCredential)
		assert.Subset(t, m.Scopes, []string{"https://outlook.office.com/SMTP.Send", "https://outlook.office.com/IMAP.AccessAsUser.All", "offline_access"})
		assert.Contains(t, string(d.Credentials), `"preset"`)
	}
	reg := connectors.NewDirectory(nil)
	for _, c := range cs {
		require.NoError(t, reg.Register(c))
	}
}

func TestCredentialsSchema(t *testing.T) {
	cs := New(Config{})
	smtp, imap := cs[0].Descriptor(), cs[1].Descriptor()
	valid := func(d connectors.Descriptor, c connectors.Credentials) error {
		return connectors.ValidateCredentials(d, c)
	}

	assert.NoError(t, valid(smtp, connectors.Credentials{"preset": "custom", "host": "smtp.example.com", "port": "465", "username": "u", "password": "p", "from_address": "a@example.com", "from_name": "A"}))
	assert.NoError(t, valid(smtp, connectors.Credentials{"host": "smtp.example.com", "port": "587", "from_address": "a@example.com"}), "no preset is custom; a relay needs no login")
	require.Error(t, valid(smtp, connectors.Credentials{"preset": "custom", "port": "587", "from_address": "a@example.com"}), "a custom server has a host")
	require.Error(t, valid(smtp, connectors.Credentials{"preset": "custom", "host": "h", "port": "99999", "from_address": "a@example.com"}))
	require.Error(t, valid(smtp, connectors.Credentials{"preset": "yahoo", "host": "h", "port": "587", "from_address": "a@example.com"}))
	assert.NoError(t, valid(smtp, connectors.Credentials{"preset": "gmail", "username": "a@gmail.com", "access_token": "t", "refresh_token": "r", "login_id": "l"}), "a sign-in has no host to type")
	assert.NoError(t, valid(smtp, connectors.Credentials{"preset": "m365", "username": "a@corp.com", "access_token": "t"}))

	assert.NoError(t, valid(imap, connectors.Credentials{"preset": "custom", "host": "imap.example.com", "port": "993", "username": "u", "password": "p", "mailbox": "INBOX"}))
	require.Error(t, valid(imap, connectors.Credentials{"preset": "custom", "host": "imap.example.com", "port": "993", "username": "u"}), "a custom server has a password")
	assert.NoError(t, valid(imap, connectors.Credentials{"preset": "gmail", "username": "a@gmail.com", "access_token": "t"}))
}

func TestPlatformCredentialFromTheApplicationsSMTP(t *testing.T) {
	_, ok := PlatformCredential(nil)
	assert.False(t, ok)
	_, ok = PlatformCredential(&smtpconfig.Config{})
	assert.False(t, ok, "without a host there is none")
	pc, ok := PlatformCredential(&smtpconfig.Config{Host: "smtp.corp", Port: 465, Username: "bot", Password: "pw", From: "IOTA <noreply@corp.example>"})
	require.True(t, ok)
	assert.Equal(t, connectors.Credentials{"preset": "custom", "host": "smtp.corp", "port": "465", "username": "bot", "password": "pw", "from_address": "IOTA <noreply@corp.example>"}, pc.Credentials)
	assert.Equal(t, "noreply@corp.example", pc.Identity.Display)

	cs := New(Config{Platform: &smtpconfig.Config{Host: "smtp.corp", From: "noreply@corp.example"}})
	got, ok := cs[0].(interface {
		PlatformCredential() (connectors.PlatformCredential, bool)
	})
	require.True(t, ok)
	pc, ok = got.PlatformCredential()
	require.True(t, ok)
	assert.Equal(t, "587", pc.Credentials["port"], "the default port of SMTP_PORT")
	require.NoError(t, connectors.ValidateCredentials(cs[0].Descriptor(), pc.Credentials))

	// The platform credential works as a connection: a platform relay sends.
	f := newFx(t, emailtest.ModeSTARTTLS)
	pc2, _ := PlatformCredential(&smtpconfig.Config{Host: "127.0.0.1", Port: atoi(port(f.srv.SMTPAddr)), Username: mailbox, Password: password, From: mailbox})
	pc2.Credentials["security"] = "starttls"
	res, err := (&sendNode{f.smtp.hub}).Execute(context.Background(), f.sendInput(params(nil), pc2.Credentials))
	require.NoError(t, err)
	assert.Equal(t, "sent", res.Outcome)
}

func TestTriggerParams(t *testing.T) {
	p := newTriggerParams()
	out, issues := p.Parse(map[string]any{})
	require.Empty(t, issues)
	assert.Equal(t, true, out["skip_replies"], "both skips are on by default")
	assert.Equal(t, true, out["skip_auto"])
	assert.Equal(t, false, out["is_reply"])
	assert.Equal(t, false, out["is_auto"])

	out, issues = p.Parse(map[string]any{"skip_replies": false, "skip_auto": false, "from": []any{"a@x.com"}, "subject_contains": "Invoice"})
	require.Empty(t, issues)
	assert.NotContains(t, out, "is_reply")
	assert.NotContains(t, out, "is_auto")

	_, issues = p.Parse(map[string]any{"from": []any{"a@x.com", "A@X.COM"}})
	assert.NotEmpty(t, issues, "an address once, whatever its case")
	_, issues = p.Parse(map[string]any{"unknown": true})
	assert.NotEmpty(t, issues)
}

func TestSubjectContainsHook(t *testing.T) {
	hook := MatchHooks()[HookSubjectContains]
	require.NotNil(t, hook)
	ev := graph.Event{"kind": EventReceived, "subject": "Your INVOICE #4"}
	assert.True(t, hook(map[string]any{"subject_contains": "invoice"}, ev))
	assert.False(t, hook(map[string]any{"subject_contains": "receipt"}, ev))
	assert.True(t, hook(map[string]any{}, ev), "no condition does not narrow")
	assert.True(t, hook(map[string]any{"subject_contains": "  "}, ev))
}

func TestWaitReplyParamsDeriveTheFilterAndTheKey(t *testing.T) {
	// An id is compared in its angle-bracket form, however the author or the run wrote it.
	id := func(expr string) string {
		return `(string(` + expr + `).startsWith("<") ? string(` + expr + `) : "<" + string(` + expr + `) + ">")`
	}
	filter := func(thread, msg string) string {
		return `event.threadId == ` + id(thread) + ` && ((has(event.inReplyTo) && event.inReplyTo != "") ? event.inReplyTo == ` + id(msg) + ` : (size(event.references) > 0 && event.references[size(event.references) - 1] == ` + id(msg) + `))`
	}
	n := newWaitReplyNode(&hub{})
	out, issues := n.params.Parse(map[string]any{"message_id": "nodes.send.output.message_id"})
	require.Empty(t, issues)
	assert.Equal(t, "3d", out["timeout"], "three days unless the author says")
	assert.Equal(t, filter("nodes.send.output.message_id", "nodes.send.output.message_id"), out["filter"])

	out, issues = n.params.Parse(map[string]any{"message_id": "nodes.send.output.message_id", "from": "{{ nodes.who.output.address }}"})
	require.Empty(t, issues)
	assert.Equal(t, filter("nodes.send.output.message_id", "nodes.send.output.message_id")+` && event.from.address.lowerAscii() == string((string(nodes.who.output.address))).lowerAscii()`, out["filter"])

	out, issues = n.params.Parse(map[string]any{"message_id": "nodes.send.output.message_id", "thread_id": "nodes.first.output.thread", "from": "Boss@Example.org"})
	require.Empty(t, issues)
	assert.Equal(t, filter("nodes.first.output.thread", "nodes.send.output.message_id")+` && event.from.address.lowerAscii() == string(("Boss@Example.org")).lowerAscii()`, out["filter"])

	_, issues = n.params.Parse(map[string]any{"message_id": "event.messageId"})
	require.NotEmpty(t, issues, "event in the filter is the incoming message, not the run's trigger")
	_, issues = n.params.Parse(map[string]any{"message_id": "nodes.s.output.id", "timeout": "31d"})
	assert.NotEmpty(t, issues, "at most 30 days")
	_, issues = n.params.Parse(map[string]any{})
	assert.NotEmpty(t, issues)

	conn := uuid.NewString()
	key, ok := n.WaitSubject(map[string]any{"connection": conn, "message_id": "<abc@example.com>"})
	require.True(t, ok)
	assert.Equal(t, graph.SubjectConversation, key.Entity)
	assert.Equal(t, conn+":<abc@example.com>", key.Key, "<imap connection>:<Message-ID of the sent message>")
	assert.Equal(t, conn+":<abc@example.com>", ConversationKey(conn, "abc@example.com"))
	key, _ = n.WaitSubject(map[string]any{"connection": conn, "message_id": "<abc@example.com>", "thread_id": "<root@example.com>"})
	assert.Equal(t, conn+":<root@example.com>", key.Key)
	_, ok = n.WaitSubject(map[string]any{"message_id": "<abc@example.com>"})
	assert.False(t, ok, "without a connection the filter alone narrows the wait")
}

func TestSendParams(t *testing.T) {
	spec := (&sendNode{}).Spec()
	_, issues := spec.Params.Parse(map[string]any{"to": []any{"a@x.com"}, "subject": "s", "text": "t"})
	assert.Empty(t, issues)
	_, issues = spec.Params.Parse(map[string]any{"to": []any{}, "subject": "s", "text": "t"})
	assert.NotEmpty(t, issues, "1 to 50 recipients")
	many := make([]any, 51)
	for i := range many {
		many[i] = "a@x.com"
	}
	_, issues = spec.Params.Parse(map[string]any{"to": many, "subject": "s", "text": "t"})
	assert.NotEmpty(t, issues)
	_, issues = spec.Params.Parse(map[string]any{"to": []any{"a@x.com"}, "subject": "s"})
	assert.NotEmpty(t, issues, "a text")
	assert.Equal(t, []string{"to[]", "cc[]", "bcc[]", "subject", "text", "html", "reply_to"}, spec.Templates)
	assert.Equal(t, []string{"in_reply_to", "references"}, spec.Values)
}

func TestLabelParams(t *testing.T) {
	spec := (&labelNode{}).Spec()
	_, issues := spec.Params.Parse(map[string]any{"message_id": "event.messageId", "mark_seen": true, "keywords": []any{"Done", "Invoice-1"}, "move_to": "Archive"})
	assert.Empty(t, issues)
	_, issues = spec.Params.Parse(map[string]any{"message_id": "event.messageId", "keywords": []any{`\Seen`}})
	assert.NotEmpty(t, issues, "a keyword is not a system flag")
	_, issues = spec.Params.Parse(map[string]any{"keywords": []any{"x"}})
	assert.NotEmpty(t, issues)
}

func TestLabelMarksAndMoves(t *testing.T) {
	ctx := context.Background()
	f := newFx(t, emailtest.ModeTLS)
	f.srv.Mailbox(mailbox, "Archive")
	f.srv.Deliver(mailbox, "INBOX", mailText("<a@x>", "ann@example.org", "one", "x"))
	f.srv.Deliver(mailbox, "INBOX", mailText("<b@x>", "ann@example.org", "two", "x"))
	n := &labelNode{f.imap.hub}
	call := func(p map[string]any) (catalog.CallResult, error) {
		return n.Execute(ctx, catalog.CallInput{TenantID: tenant, Params: p, Credentials: f.imapCreds(), Connection: &catalog.ConnectionInfo{ID: uuid.NewString(), Kind: KindIMAP}})
	}

	res, err := call(map[string]any{"message_id": "a@x", "mark_seen": true, "keywords": []any{"Handled"}})
	require.NoError(t, err)
	assert.Equal(t, "done", res.Outcome)
	inbox := f.srv.Fetch(mailbox, "INBOX")
	require.Len(t, inbox, 2)
	assert.ElementsMatch(t, []string{`\seen`, "handled"}, lower(inbox[0].Flags), "flags are not case sensitive")
	assert.Empty(t, inbox[1].Flags, "only the named message")

	res, err = call(map[string]any{"message_id": "<a@x>", "mark_seen": true, "keywords": []any{"Handled"}})
	require.NoError(t, err)
	assert.Equal(t, "done", res.Outcome, "repeated: the same state")

	res, err = call(map[string]any{"message_id": "<b@x>", "move_to": "Archive"})
	require.NoError(t, err)
	assert.Equal(t, "done", res.Outcome)
	assert.Len(t, f.srv.Fetch(mailbox, "INBOX"), 1)
	moved := f.srv.Fetch(mailbox, "Archive")
	require.Len(t, moved, 1)
	assert.Contains(t, string(moved[0].Raw), "Message-ID: <b@x>")

	res, err = call(map[string]any{"message_id": "<nobody@x>", "mark_seen": true})
	require.NoError(t, err)
	assert.Equal(t, "not_found", res.Outcome)

	_, err = call(map[string]any{"message_id": "<a@x>", "move_to": "Nowhere"})
	assert.Equal(t, catalog.CallInvalidRequest, callError(t, err).Code)

	creds := f.imapCreds()
	creds["password"] = "wrong-password"
	_, err = n.Execute(ctx, catalog.CallInput{TenantID: tenant, Params: map[string]any{"message_id": "<a@x>"}, Credentials: creds, Connection: &catalog.ConnectionInfo{ID: uuid.NewString()}})
	assert.Equal(t, catalog.CallUnauthorized, callError(t, err).Code)
	creds = f.imapCreds()
	creds["port"] = "1"
	_, err = n.Execute(ctx, catalog.CallInput{TenantID: tenant, Params: map[string]any{"message_id": "<a@x>"}, Credentials: creds, Connection: &catalog.ConnectionInfo{ID: uuid.NewString()}})
	ce := callError(t, err)
	assert.Equal(t, catalog.CallUnavailable, ce.Code)
	assert.True(t, ce.Retryable, "the node is idempotent: the engine repeats it")
}

// Only the identifier event is the incoming message of a wait's filter: a word in a quoted text, and a member
// of that name of an earlier node's output, are not references to it.
func TestMentionsEventIsAboutTheIdentifier(t *testing.T) {
	for src, want := range map[string]bool{
		"event.messageId":                              true,
		"size(event.references) > 0":                   true,
		"x == event":                                   true,
		"(event.from.address)":                         true,
		`("event@example.org")`:                        false,
		`string(nodes.wait.output.event.from.address)`: false,
		"nodes.wait.output.event.messageId":            false,
		`nodes.a.output["event"]`:                      false,
		"prevent.x":                                    false,
		"events":                                       false,
		"":                                             false,
	} {
		assert.Equal(t, want, mentionsEvent(src), src)
	}
	n := newWaitReplyNode(&hub{})
	_, issues := n.params.Parse(map[string]any{"message_id": "nodes.send.output.message_id", "from": "{{ nodes.wait.output.event.from.address }}"})
	assert.Empty(t, issues, "the address an earlier wait's reply came from")
}

// The wait's filter evaluated against the payload of an incoming message, as the poller makes it: a reply
// that has no In-Reply-To key is matched by the last of its References, a reply that lists the sent id only
// earlier in the thread answers another message, and a different thread is not this wait's.
func TestWaitReplyFilterAgainstEventPayloads(t *testing.T) {
	n := newWaitReplyNode(&hub{})
	out, issues := n.params.Parse(map[string]any{"message_id": "nodes.send.output.message_id", "thread_id": "nodes.first.output.thread"})
	require.Empty(t, issues)
	scope := expr.Scope{Vars: map[string]expr.Field{
		"event": expr.ObjectOf(receivedPayload()),
		"nodes": expr.ObjectOf(expr.Fields{
			{Name: "send", Field: expr.ObjectOf(expr.Fields{{Name: "output", Field: expr.ObjectOf(expr.Fields{{Name: "message_id", Field: expr.StringField}})}})},
			{Name: "first", Field: expr.ObjectOf(expr.Fields{{Name: "output", Field: expr.ObjectOf(expr.Fields{{Name: "thread", Field: expr.StringField}})}})},
		}),
	}}
	env, err := expr.NewEnv(scope, expr.Limits{})
	require.NoError(t, err)
	prog, errs := env.Check(out["filter"].(string), expr.TypeBool)
	require.Empty(t, errs)
	nodes := map[string]any{
		"send":  map[string]any{"output": map[string]any{"message_id": "<sent@x>"}},
		"first": map[string]any{"output": map[string]any{"thread": "<root@x>"}},
	}
	eval := func(ev map[string]any) bool {
		got, err := prog.Eval(map[string]any{"event": ev, "nodes": nodes})
		require.NoError(t, err)
		return got == true
	}
	reply := func(extra map[string]any) map[string]any {
		ev := map[string]any{"threadId": "<root@x>", "references": []any{}, "messageId": "<r@x>"}
		for k, v := range extra {
			ev[k] = v
		}
		return ev
	}
	assert.True(t, eval(reply(map[string]any{"inReplyTo": "<sent@x>", "references": []any{"<root@x>", "<sent@x>"}})), "In-Reply-To names the sent message")
	assert.True(t, eval(reply(map[string]any{"references": []any{"<root@x>", "<sent@x>"}})), "no In-Reply-To key: the last of References")
	assert.True(t, eval(reply(map[string]any{"inReplyTo": "", "references": []any{"<root@x>", "<sent@x>"}})), "an empty In-Reply-To: the last of References")
	assert.False(t, eval(reply(map[string]any{"inReplyTo": "<other@x>", "references": []any{"<root@x>", "<sent@x>", "<other@x>"}})), "answers another message of the conversation")
	assert.False(t, eval(reply(map[string]any{"references": []any{"<root@x>", "<sent@x>", "<later@x>"}})), "the sent id is only history in References")
	assert.False(t, eval(reply(map[string]any{"threadId": "<elsewhere@x>", "inReplyTo": "<sent@x>"})), "another conversation")
	assert.False(t, eval(reply(map[string]any{"inReplyTo": "sent@x", "references": []any{"<root@x>"}})), "an id without brackets in a header is not this message's")
}

func TestTemplateToCEL(t *testing.T) {
	for in, want := range map[string]string{
		"a@x.com":                   `("a@x.com")`,
		"{{ nodes.a.output.mail }}": `(string(nodes.a.output.mail))`,
		"boss+{{ x }}@{{ y }}.com":  `("boss+" + string(x) + "@" + string(y) + ".com")`,
		"":                          `""`,
	} {
		got, err := templateToCEL(in)
		require.NoError(t, err)
		assert.Equal(t, want, got, in)
	}
}

func TestDialogueLogHasNoSecrets(t *testing.T) {
	var log bytes.Buffer
	f := newFx(t, emailtest.ModeTLS)
	f.smtp.cfg.Debug, f.imap.cfg.Debug = &log, &log
	f.srv.AcceptToken(mailbox, "ya29.very-secret-token")
	_, err := f.smtp.Test(context.Background(), f.smtpCreds())
	require.NoError(t, err)
	_, err = f.imap.Test(context.Background(), f.imapCreds())
	require.NoError(t, err)
	oc := f.oauthCreds("", "ya29.very-secret-token")
	_, err = f.smtp.Test(context.Background(), oc)
	require.NoError(t, err)
	_, err = f.imap.Test(context.Background(), oc)
	require.NoError(t, err)

	out := log.String()
	require.NotEmpty(t, out, "the dialogue is logged when asked for")
	assert.Contains(t, out, "AUTH PLAIN", "the command stays")
	assert.Contains(t, out, "AUTH XOAUTH2")
	for _, secret := range []string{password, "ya29.very-secret-token"} {
		assert.NotContains(t, out, secret)
		for _, form := range []string{secret, "\x00" + mailbox + "\x00" + secret, "user=" + mailbox + "\x01auth=Bearer " + secret + "\x01\x01"} {
			assert.NotContains(t, out, base64Std(form), "the SASL forms of %q", secret)
		}
	}
	assert.NotContains(t, out, "AUTHENTICATE PLAIN "+base64Std("\x00"+mailbox+"\x00"+password))
	assert.NotContains(t, out, "Bearer ya29")
}

// An SMTP send and an IMAP poll that log into one destination at once do not race on it (run with -race).
func TestDialogueLogIsSafeForConcurrentSessions(t *testing.T) {
	var log bytes.Buffer
	f := newFx(t, emailtest.ModeTLS)
	f.smtp.cfg.Debug = &log
	var wg sync.WaitGroup
	for i := 0; i < 6; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			_, _ = f.smtp.Test(context.Background(), f.smtpCreds())
		}()
		go func() {
			defer wg.Done()
			_, _ = f.imap.Test(context.Background(), f.imapCreds())
		}()
	}
	wg.Wait()
	assert.NotContains(t, log.String(), password)
	assert.Contains(t, log.String(), "AUTH")
}

func TestNothingIsLoggedByDefault(t *testing.T) {
	f := newFx(t, emailtest.ModeTLS)
	assert.Nil(t, f.smtp.cfg.Debug)
	assert.Nil(t, newMaskedWriter(nil, "x"))
}

func base64Std(s string) string { return base64.StdEncoding.EncodeToString([]byte(s)) }

func lower(l []string) []string {
	out := make([]string, len(l))
	for i, s := range l {
		out[i] = strings.ToLower(s)
	}
	return out
}

// A subject matches by Unicode case folding: the final and the medial sigma are one letter.
func TestSubjectContainsFoldsUnicodeCase(t *testing.T) {
	hook := MatchHooks()[HookSubjectContains]
	for _, c := range []struct {
		subject, want string
		match         bool
	}{
		{"ΟΣ", "ος", true},
		{"Your INVOICE #4", "invoice", true},
		{"Re: Счёт №4", "СЧЁТ", true},
		{"Straße", "STRASSE", false}, // simple folding only: ß stays itself
		{"invoice", "receipt", false},
	} {
		got := hook(map[string]any{"subject_contains": c.want}, graph.Event{"kind": EventReceived, "subject": c.subject})
		assert.Equal(t, c.match, got, "%q in %q", c.want, c.subject)
	}
}
