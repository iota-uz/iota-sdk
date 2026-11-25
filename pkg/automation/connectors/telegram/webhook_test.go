package telegram_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/pkg/automation/catalog"
	"github.com/iota-uz/iota-sdk/pkg/automation/connectors"
	"github.com/iota-uz/iota-sdk/pkg/automation/connectors/telegram"
	"github.com/iota-uz/iota-sdk/pkg/automation/connectors/telegram/telegramtest"
	"github.com/iota-uz/iota-sdk/pkg/automation/engine"
)

func payloadOf(t *testing.T, ev engine.InboundEvent) map[string]any {
	t.Helper()
	var p map[string]any
	require.NoError(t, json.Unmarshal(ev.Payload, &p))
	return p
}

func (r *rig) tenantUpdate(u map[string]any) (int, []engine.InboundEvent) {
	return r.post(r.conn, r.creds(), r.secret(r.conn, tenantToken), u)
}

func (r *rig) platformUpdate(u map[string]any) (int, []engine.InboundEvent) {
	return r.post(r.platformConn(), r.platformCreds(), r.secret(r.platformConn(), platformToken), u)
}

func TestTenantMessageBecomesAnEvent(t *testing.T) {
	r := newRig(t)
	r.api.Chat(telegramtest.Chat{ID: 700, Type: "private", Title: ""})
	code, evs := r.tenantUpdate(r.bot.Update(700, alice, "hello there"))
	require.Equal(t, http.StatusOK, code)
	require.Len(t, evs, 1)
	ev := evs[0]
	assert.Equal(t, telegram.EventMessage, ev.Event)
	assert.Equal(t, "conversation", ev.Subject.Entity)
	assert.Equal(t, r.conn.ID.String()+":700", ev.Subject.Key)
	assert.NotEmpty(t, ev.DedupKey)
	assert.False(t, ev.BySelf)
	p := payloadOf(t, ev)
	assert.Equal(t, "hello there", p["text"])
	assert.Equal(t, "700", p["chatId"])
	assert.Equal(t, "private", p["chatType"])
	assert.Equal(t, false, p["isCommand"])
	assert.Equal(t, false, p["hasMedia"])
	assert.Equal(t, map[string]any{"userId": "501", "username": "alice", "firstName": "Alice"}, p["from"])
	assert.NotContains(t, p, "threadId")
	assert.NotContains(t, p, "command")
}

func TestCommandsAreParsed(t *testing.T) {
	r := newRig(t)
	_, evs := r.tenantUpdate(r.bot.Update(700, alice, "/Help@tenant_bot  topic one"))
	require.Len(t, evs, 1)
	p := payloadOf(t, evs[0])
	assert.Equal(t, true, p["isCommand"])
	assert.Equal(t, "help", p["command"])
	assert.Equal(t, "topic one", p["args"])
	assert.Equal(t, "/Help@tenant_bot  topic one", p["text"])
}

func TestTopicMessagesCarryTheThread(t *testing.T) {
	r := newRig(t)
	u := r.bot.Update(-1001, alice, "in a topic")
	msg := u["message"].(map[string]any)
	msg["message_thread_id"], msg["is_topic_message"] = 42, true
	msg["chat"].(map[string]any)["type"] = "supergroup"
	_, evs := r.tenantUpdate(u)
	require.Len(t, evs, 1)
	p := payloadOf(t, evs[0])
	assert.Equal(t, "42", p["threadId"])
	assert.Equal(t, "supergroup", p["chatType"])
}

// The bot's own messages start nothing: the engine reads by_self.
func TestMessagesOfTheBotItselfAreBySelf(t *testing.T) {
	r := newRig(t)
	u := r.bot.Update(700, alice, "echo")
	u["message"].(map[string]any)["from"] = map[string]any{"id": 111111, "is_bot": true, "first_name": "bot"}
	_, evs := r.tenantUpdate(u)
	require.Len(t, evs, 1)
	assert.True(t, evs[0].BySelf)
}

func TestServiceMessagesAndOtherUpdatesAreDropped(t *testing.T) {
	r := newRig(t)
	u := r.bot.Update(700, alice, "")
	delete(u["message"].(map[string]any), "text")
	u["message"].(map[string]any)["new_chat_members"] = []any{map[string]any{"id": 9, "first_name": "x"}}
	code, evs := r.tenantUpdate(u)
	assert.Equal(t, http.StatusOK, code)
	assert.Empty(t, evs)

	code, evs = r.tenantUpdate(map[string]any{"update_id": 5, "edited_message": map[string]any{"message_id": 1}})
	assert.Equal(t, http.StatusOK, code, "an update of another kind is answered 200")
	assert.Empty(t, evs)
}

func TestPhotoWithoutCaptionIsAnEventWithItsFile(t *testing.T) {
	r := newRig(t)
	u := r.photoUpdate(700, "", []byte("PNGDATA"))
	code, evs := r.tenantUpdate(u)
	require.Equal(t, http.StatusOK, code)
	require.Len(t, evs, 1)
	p := payloadOf(t, evs[0])
	assert.Equal(t, true, p["hasMedia"])
	assert.NotContains(t, p, "text", "an empty text: event.text reads as empty")
	files, _ := p["files"].([]any)
	require.Len(t, files, 1)
	f := files[0].(map[string]any)
	assert.NotEmpty(t, f["upload_id"])
	assert.Equal(t, "/uploads/"+f["upload_id"].(string), f["url"])
	assert.Equal(t, "image/jpeg", f["mime"])
	assert.NotContains(t, f, "error")
	stored := r.up.files[f["upload_id"].(string)]
	assert.Equal(t, []byte("PNGDATA"), stored.Data, "the file is stored at arrival")
}

func TestDocumentWithCaption(t *testing.T) {
	r := newRig(t)
	u := r.documentUpdate(700, "the report", "report.pdf", "application/pdf", []byte("%PDF"))
	_, evs := r.tenantUpdate(u)
	require.Len(t, evs, 1)
	p := payloadOf(t, evs[0])
	assert.Equal(t, "the report", p["text"], "the caption is the text")
	f := p["files"].([]any)[0].(map[string]any)
	assert.Equal(t, "report.pdf", f["name"])
	assert.Equal(t, "application/pdf", f["mime"])
}

// A file that cannot be stored does not lose the event: the file says why.
func TestAFileThatCannotBeStoredKeepsTheEvent(t *testing.T) {
	r := newRig(t)
	r.up.fail = true
	_, evs := r.tenantUpdate(r.photoUpdate(700, "see", []byte("x")))
	require.Len(t, evs, 1)
	f := payloadOf(t, evs[0])["files"].([]any)[0].(map[string]any)
	assert.Equal(t, telegram.FileStorage, f["error"])
	assert.NotContains(t, f, "upload_id")

	r.up.fail = false
	big := r.documentUpdate(700, "", "big.bin", "application/octet-stream", []byte("x"))
	big["message"].(map[string]any)["document"].(map[string]any)["file_size"] = 21 << 20
	asked := len(r.bot.CallsOf("getFile"))
	_, evs = r.tenantUpdate(big)
	require.Len(t, evs, 1)
	f = payloadOf(t, evs[0])["files"].([]any)[0].(map[string]any)
	assert.Equal(t, telegram.FileTooLarge, f["error"])
	assert.Len(t, r.bot.CallsOf("getFile"), asked, "a file over 20 MB is not even asked for")
}

func (r *rig) photoUpdate(chat int64, caption string, data []byte) map[string]any {
	r.t.Helper()
	return r.bot.PhotoUpdate(chat, alice, caption, data)
}

func (r *rig) documentUpdate(chat int64, caption, name, mime string, data []byte) map[string]any {
	r.t.Helper()
	return r.bot.DocumentUpdate(chat, alice, caption, name, mime, data)
}

// ---------------------------------------------------------------------------------------------
// the platform bot

func TestPlatformBotPublishesNoPlainMessage(t *testing.T) {
	r := newRig(t)
	code, evs := r.platformUpdate(r.platform.Update(800, alice, "hello platform"))
	assert.Equal(t, http.StatusOK, code)
	assert.Empty(t, evs, "a message to the platform bot starts nobody's graph")
	code, evs = r.platformUpdate(r.platform.Update(800, alice, "/help"))
	assert.Equal(t, http.StatusOK, code)
	assert.Empty(t, evs)
	assert.Empty(t, r.platform.Sent(), "and is not answered")
}

type linkHandler struct {
	prefix string
	tenant uuid.UUID
	known  map[string]bool
	got    []telegram.StartUpdate
	err    error
	tenErr error
}

func (h *linkHandler) Prefix() string { return h.prefix }
func (h *linkHandler) TenantOf(_ context.Context, payload string) (uuid.UUID, bool, error) {
	if h.tenErr != nil {
		return uuid.Nil, false, h.tenErr
	}
	if h.known[payload] {
		return h.tenant, true, nil
	}
	return uuid.Nil, false, nil
}
func (h *linkHandler) Handle(_ context.Context, u telegram.StartUpdate) (string, error) {
	h.got = append(h.got, u)
	if h.err != nil {
		return "", h.err
	}
	if u.TenantID == uuid.Nil {
		return "The link is unknown or used.", nil
	}
	return "Linked, " + u.FirstName, nil
}

func TestStartWithARegisteredPrefixGoesToItsHandlerAndPublishesNothing(t *testing.T) {
	r := newRig(t)
	h := &linkHandler{prefix: "u_", tenant: r.tenant, known: map[string]bool{"u_abc": true}}
	require.NoError(t, r.c.RegisterStart(h))
	require.Error(t, r.c.RegisterStart(&linkHandler{prefix: "u_"}), "a prefix twice")
	require.Error(t, r.c.RegisterStart(&linkHandler{prefix: "users_"}), "a prefix is 1-4 letters and an underscore")

	r.api.Chat(telegramtest.Chat{ID: 800, Type: "private"})
	code, evs := r.platformUpdate(r.platform.Update(800, alice, "/start u_abc"))
	assert.Equal(t, http.StatusOK, code)
	assert.Empty(t, evs, "no telegram.message_received.v1 for /start")
	require.Len(t, h.got, 1)
	got := h.got[0]
	assert.Equal(t, r.tenant, got.TenantID)
	assert.Equal(t, "platform:telegram_bot", got.ConnectionID)
	assert.Equal(t, int64(800), got.ChatID)
	assert.Equal(t, int64(501), got.UserID)
	assert.Equal(t, "private", got.ChatType)
	assert.Equal(t, "alice", got.Username)
	assert.Equal(t, "Alice", got.FirstName)
	assert.Equal(t, "u_abc", got.Payload)
	sent := r.platform.Sent()
	require.Len(t, sent, 1)
	assert.Equal(t, "Linked, Alice", sent[0].Text, "the reply is sent to the chat")

	// A used payload: the handler is called with the nil tenant and only replies.
	_, evs = r.platformUpdate(r.platform.Update(800, alice, "/start u_used"))
	assert.Empty(t, evs)
	require.Len(t, h.got, 2)
	assert.Equal(t, uuid.Nil, h.got[1].TenantID)
	assert.Equal(t, "The link is unknown or used.", r.platform.Sent()[1].Text)
}

func TestStartWithAnUnknownPrefixIsDroppedWithoutAnswer(t *testing.T) {
	r := newRig(t)
	r.api.Chat(telegramtest.Chat{ID: 800, Type: "private"})
	h := &linkHandler{prefix: "u_"}
	require.NoError(t, r.c.RegisterStart(h))
	code, evs := r.platformUpdate(r.platform.Update(800, alice, "/start zz_1"))
	assert.Equal(t, http.StatusOK, code)
	assert.Empty(t, evs)
	assert.Empty(t, h.got)
	assert.Empty(t, r.platform.Sent())
	_, evs = r.platformUpdate(r.platform.Update(800, alice, "/start"))
	assert.Empty(t, evs)
}

func TestStartHandlerFailureIsRedelivered(t *testing.T) {
	r := newRig(t)
	h := &linkHandler{prefix: "u_", tenant: r.tenant, known: map[string]bool{"u_a": true}, err: assert.AnError}
	require.NoError(t, r.c.RegisterStart(h))
	code, _ := r.platformUpdate(r.platform.Update(800, alice, "/start u_a"))
	assert.Equal(t, http.StatusServiceUnavailable, code, "a 5xx: Telegram redelivers")
	h.err, h.tenErr = nil, assert.AnError
	code, _ = r.platformUpdate(r.platform.Update(800, alice, "/start u_a"))
	assert.Equal(t, http.StatusServiceUnavailable, code)
}

// On a tenant's own bot a /start with a prefix is the handler's too, and never an event; the payload
// must be the connection's tenant's, else it is unknown.
func TestStartOnATenantBot(t *testing.T) {
	r := newRig(t)
	h := &linkHandler{prefix: "u_", tenant: r.tenant, known: map[string]bool{"u_mine": true}}
	require.NoError(t, r.c.RegisterStart(h))
	_, evs := r.tenantUpdate(r.bot.Update(700, alice, "/start u_mine"))
	assert.Empty(t, evs)
	require.Len(t, h.got, 1)
	assert.Equal(t, r.tenant, h.got[0].TenantID)
	assert.Equal(t, r.conn.ID.String(), h.got[0].ConnectionID)

	h.tenant = uuid.New() // the payload is another tenant's
	_, evs = r.tenantUpdate(r.bot.Update(700, alice, "/start u_mine"))
	assert.Empty(t, evs)
	assert.Equal(t, uuid.Nil, h.got[1].TenantID, "treated as unknown")

	// An unknown prefix on a tenant bot is a plain command message.
	_, evs = r.tenantUpdate(r.bot.Update(700, alice, "/start promo"))
	require.Len(t, evs, 1)
	assert.Equal(t, "start", payloadOf(t, evs[0])["command"])
	assert.Equal(t, "promo", payloadOf(t, evs[0])["args"])
}

func TestKnownChatsAreRecordedFromMessages(t *testing.T) {
	r := newRig(t)
	r.api.Chat(telegramtest.Chat{ID: 700, Type: "private"})
	g := r.bot.Update(-1001, alice, "hi")
	g["message"].(map[string]any)["chat"] = map[string]any{"id": -1001, "type": "supergroup", "title": "Sales team"}
	r.tenantUpdate(g)
	r.tenantUpdate(r.bot.Update(700, alice, "hello"))
	got, err := r.c.KnownChats(context.Background(), r.tenant, r.conn.ID, "", 10)
	require.NoError(t, err)
	require.Len(t, got, 2)
	assert.Equal(t, int64(700), got[0].ID, "most recent first")
	assert.Equal(t, "Alice", got[0].Title)
	assert.Equal(t, "supergroup", got[1].Type)
	got, err = r.c.KnownChats(context.Background(), r.tenant, r.conn.ID, "sales", 10)
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, int64(-1001), got[0].ID)
}

func TestChatOptionsAreGroupedPrivateAndGroups(t *testing.T) {
	r := newRig(t)
	g := r.bot.Update(-1001, alice, "hi")
	g["message"].(map[string]any)["chat"] = map[string]any{"id": -1001, "type": "supergroup", "title": "Sales team", "username": "salesteam"}
	r.tenantUpdate(g)
	r.tenantUpdate(r.bot.Update(700, alice, "hello"))
	opts, err := r.c.Options(context.Background(), r.tenant, r.conn.ID, telegram.SourceChats, "")
	require.NoError(t, err)
	require.Len(t, opts, 2)
	assert.Equal(t, "private", opts[0].Group)
	assert.Equal(t, "700", opts[0].Value)
	assert.Equal(t, "group", opts[1].Group)
	assert.Equal(t, "Sales team (@salesteam)", opts[1].Label)
	none, err := r.c.Options(context.Background(), r.tenant, r.conn.ID, "other.source", "")
	require.NoError(t, err)
	assert.Empty(t, none)
}

// A command addressed to another bot of a group is that bot's: this connector neither dispatches nor
// publishes it.
func TestACommandForAnotherBotIsNotOurs(t *testing.T) {
	r := newRig(t)
	h := &linkHandler{prefix: "u_", tenant: r.tenant, known: map[string]bool{"u_abc": true}}
	require.NoError(t, r.c.RegisterStart(h))
	r.api.Chat(telegramtest.Chat{ID: 700, Type: "private"})
	_, evs := r.tenantUpdate(r.bot.Update(700, alice, "/start@other_bot u_abc"))
	assert.Empty(t, evs)
	assert.Empty(t, h.got, "not this bot's /start")
	_, evs = r.tenantUpdate(r.bot.Update(700, alice, "/help@other_bot"))
	assert.Empty(t, evs, "nor a command message")
	_, evs = r.tenantUpdate(r.bot.Update(700, alice, "/start@tenant_bot u_abc"))
	assert.Empty(t, evs)
	assert.Len(t, h.got, 1, "addressed to this bot")
	_, evs = r.tenantUpdate(r.bot.Update(700, alice, "/help@tenant_bot"))
	assert.Len(t, evs, 1)
}

// A /start that was handled completely is not handled again when Telegram redelivers it.
func TestAHandledStartIsNotHandledTwice(t *testing.T) {
	r := newRig(t)
	h := &linkHandler{prefix: "u_", tenant: r.tenant, known: map[string]bool{"u_abc": true}}
	require.NoError(t, r.c.RegisterStart(h))
	r.api.Chat(telegramtest.Chat{ID: 800, Type: "private"})
	u := r.platform.Update(800, alice, "/start u_abc")
	r.platformUpdate(u)
	r.platformUpdate(u)
	assert.Len(t, h.got, 1, "the handler ran once")
	assert.Len(t, r.platform.Sent(), 1, "and its reply was sent once")

	// A handler that failed is repeated: it did not complete.
	h2 := &linkHandler{prefix: "v_", tenant: r.tenant, known: map[string]bool{"v_x": true}, err: assert.AnError}
	require.NoError(t, r.c.RegisterStart(h2))
	u2 := r.platform.Update(800, alice, "/start v_x")
	code, _ := r.platformUpdate(u2)
	assert.Equal(t, 503, code)
	h2.err = nil
	r.platformUpdate(u2)
	assert.Len(t, h2.got, 2)
}

// A failed write of a chat does not hold the next message of the chat back.
func TestAFailedChatWriteIsRetriedWithTheNextMessage(t *testing.T) {
	r := newRig(t)
	r.st.failWrites = 1
	r.tenantUpdate(r.bot.Update(700, alice, "one"))
	got, err := r.c.KnownChats(context.Background(), r.tenant, r.conn.ID, "", 10)
	require.NoError(t, err)
	assert.Empty(t, got, "the storage was down")
	r.tenantUpdate(r.bot.Update(700, alice, "two"))
	got, err = r.c.KnownChats(context.Background(), r.tenant, r.conn.ID, "", 10)
	require.NoError(t, err)
	assert.Len(t, got, 1, "the next message wrote it")
	r.tenantUpdate(r.bot.Update(700, alice, "three"))
	got, err = r.c.KnownChats(context.Background(), r.tenant, r.conn.ID, "", 10)
	require.NoError(t, err)
	assert.Len(t, got, 1, "and a repeat of a written chat adds nothing")
}

// The keyboard of an answered question is taken off even when Telegram fails a few times.
func TestKeyboardCleanupSurvivesTransientFailures(t *testing.T) {
	r := newRig(t)
	m := r.asked()
	r.in.open[askToken] = r.tenant
	u := r.pressUpdate(m, alice, 0)
	_, evs := r.tenantUpdate(u)
	require.Len(t, evs, 1)
	r.bot.Fail(telegramtest.Failure{Method: "editMessageText", Code: 502, Description: "Bad Gateway", Times: 3})
	r.bot.Fail(telegramtest.Failure{Method: "editMessageReplyMarkup", Code: 502, Description: "Bad Gateway", Times: 1})
	require.NoError(t, r.c.AnswerPublished(context.Background(), evs[0], r.conn, r.creds()))
	got, found := r.bot.Message(700, m.ID)
	require.True(t, found, "the original message is there")
	assert.Empty(t, got.Markup, "the keyboard is gone")
}

// Two documents with one name are two files.
func TestTwoDocumentsWithOneNameAreTwoFiles(t *testing.T) {
	r := newRig(t)
	a := r.documentUpdate(700, "", "report.pdf", "application/pdf", []byte("first"))
	b := r.documentUpdate(700, "", "report.pdf", "application/pdf", []byte("second"))
	_, evA := r.tenantUpdate(a)
	_, evB := r.tenantUpdate(b)
	require.Len(t, evA, 1)
	require.Len(t, evB, 1)
	idA := payloadOf(t, evA[0])["files"].([]any)[0].(map[string]any)["upload_id"].(string)
	idB := payloadOf(t, evB[0])["files"].([]any)[0].(map[string]any)["upload_id"].(string)
	assert.Equal(t, []byte("first"), r.up.files[idA].Data)
	assert.Equal(t, []byte("second"), r.up.files[idB].Data)
	assert.Equal(t, "report.pdf", r.up.files[idB].Name, "the name is kept")
}

// One-use link: the first delivery consumes the payload, so the redelivery no longer resolves to a tenant. It
// is still recognised as handled, and answered with nothing, not with "unknown or used".
type oneUseLink struct {
	linkHandler
	used bool
}

func (h *oneUseLink) TenantOf(ctx context.Context, payload string) (uuid.UUID, bool, error) {
	if h.used {
		return uuid.Nil, false, nil
	}
	return h.linkHandler.TenantOf(ctx, payload)
}

func (h *oneUseLink) Handle(ctx context.Context, u telegram.StartUpdate) (string, error) {
	reply, err := h.linkHandler.Handle(ctx, u)
	h.used = true // consumed by the handler
	return reply, err
}

func TestARedeliveredConsumedStartIsNotAnsweredUnknown(t *testing.T) {
	r := newRig(t)
	h := &oneUseLink{linkHandler: linkHandler{prefix: "u_", tenant: r.tenant, known: map[string]bool{"u_abc": true}}}
	require.NoError(t, r.c.RegisterStart(h))
	r.api.Chat(telegramtest.Chat{ID: 800, Type: "private"})
	u := r.platform.Update(800, alice, "/start u_abc")
	r.platformUpdate(u)
	require.Len(t, r.platform.Sent(), 1)
	assert.Equal(t, "Linked, Alice", r.platform.Sent()[0].Text)
	r.platformUpdate(u)
	assert.Len(t, h.got, 1, "the handler ran once")
	assert.Len(t, r.platform.Sent(), 1, "no contradictory second reply")
}

// When the keyboard cannot be taken off even with retries, the update is answered 503 so that Telegram
// redelivers the press; the redelivery (its answer being closed) takes the keyboard off.
func TestAKeyboardThatStaysMakesTheUpdateBeRedelivered(t *testing.T) {
	r := newRig(t)
	m := r.asked()
	r.in.open[askToken] = r.tenant
	press := r.pressUpdate(m, alice, 0) // one update: Telegram delivers the same one again
	_, evs := r.tenantUpdate(press)
	require.Len(t, evs, 1)
	r.bot.Fail(telegramtest.Failure{Method: "editMessageText", Code: 502, Description: "Bad Gateway", Times: 3})
	r.bot.Fail(telegramtest.Failure{Method: "editMessageReplyMarkup", Code: 502, Description: "Bad Gateway", Times: 3})
	require.Error(t, r.c.AnswerPublished(context.Background(), evs[0], r.conn, r.creds()), "the follow-up did not complete")

	// The same press comes again: its answer is closed now, and the keyboard goes once Telegram works again.
	// (The engine answers 503 for a connector's error; here the connector's own replay is checked.)
	delete(r.in.open, askToken)
	r.bot.Fail(telegramtest.Failure{Method: "editMessageReplyMarkup", Code: 502, Description: "Bad Gateway", Times: 3})
	code, _ := r.tenantUpdate(press)
	assert.Equal(t, http.StatusServiceUnavailable, code, "still failing: redelivered again")
	code, _ = r.tenantUpdate(press)
	assert.Equal(t, http.StatusOK, code)
	got, found := r.bot.Message(700, m.ID)
	require.True(t, found, "the original message is there")
	assert.Empty(t, got.Markup)
}

func TestAFileNameWithAPercentIsKeptWhole(t *testing.T) {
	r := newRig(t)
	_, evs := r.tenantUpdate(r.documentUpdate(700, "", "report%s%d.pdf", "application/pdf", []byte("x")))
	require.Len(t, evs, 1)
	f := payloadOf(t, evs[0])["files"].([]any)[0].(map[string]any)
	assert.Equal(t, "report%s%d.pdf", f["name"])
	assert.Equal(t, []byte("x"), r.up.files[f["upload_id"].(string)].Data)
}

// A replay of an update with a file (Telegram did not get the answer) finds the file saved by the delivery
// before it: one upload, referenced by the event the engine keeps.
func TestAReplayedDocumentIsSavedOnce(t *testing.T) {
	r := newRig(t)
	u := r.documentUpdate(700, "", "report.pdf", "application/pdf", []byte("x"))
	_, first := r.tenantUpdate(u)
	_, second := r.tenantUpdate(u)
	require.Len(t, first, 1)
	require.Len(t, second, 1)
	assert.Len(t, r.up.files, 1, "saved once")
	assert.Equal(t, payloadOf(t, first[0])["files"], payloadOf(t, second[0])["files"])
}

// When the bot's own name cannot be had, a command that names a bot is not dropped: the update is redelivered.
func TestAnAddressedCommandIsRedeliveredWhenTheBotCannotBeNamed(t *testing.T) {
	r := newRig(t)
	h := &linkHandler{prefix: "u_", tenant: r.tenant, known: map[string]bool{"u_abc": true}}
	require.NoError(t, r.c.RegisterStart(h))
	r.api.Chat(telegramtest.Chat{ID: 700, Type: "private"})
	r.bot.Fail(telegramtest.Failure{Method: "getMe", Code: 502, Description: "Bad Gateway", Times: 1})
	u := r.bot.Update(700, alice, "/start@tenant_bot u_abc")
	code, evs := r.tenantUpdate(u)
	assert.Equal(t, http.StatusServiceUnavailable, code)
	assert.Empty(t, evs)
	assert.Empty(t, h.got, "nothing was decided")
	code, _ = r.tenantUpdate(u)
	assert.Equal(t, http.StatusOK, code)
	assert.Len(t, h.got, 1, "the redelivery was handled")
}

// The result of a completed /start that could not be kept leaves the claim: the handler has consumed its
// link, so the redelivery (once the claim is abandoned) is the only way on, and nothing was sent meanwhile.
func TestAStartWhoseResultCannotBeKeptIsRedelivered(t *testing.T) {
	r := newRig(t)
	h := &oneUseLink{linkHandler: linkHandler{prefix: "u_", tenant: r.tenant, known: map[string]bool{"u_abc": true}}}
	require.NoError(t, r.c.RegisterStart(h))
	r.api.Chat(telegramtest.Chat{ID: 800, Type: "private"})
	r.st.failHandled = 3 // the write is tried three times
	u := r.platform.Update(800, alice, "/start u_abc")
	code, _ := r.platformUpdate(u)
	assert.Equal(t, http.StatusServiceUnavailable, code, "Telegram is asked to deliver it again")
	assert.Empty(t, r.platform.Sent(), "nothing was sent before the result was kept")
	code, _ = r.platformUpdate(u)
	assert.Equal(t, http.StatusServiceUnavailable, code, "the claim is fresh: the other delivery may still be running")
	r.st.abandon()
	code, _ = r.platformUpdate(u)
	assert.Equal(t, http.StatusOK, code)
	require.Len(t, r.platform.Sent(), 1)
	assert.Len(t, h.got, 2, "the handler was asked again: its first result was never kept")
	assert.Equal(t, "The link is unknown or used.", r.platform.Sent()[0].Text, "its link was consumed by the first run: the reply is the one the second gives")
}

// A failure to keep the result that passes is tried again at once: the link is spent, and the reply that goes out is
// the handler's own.
func TestAStartResultThatIsKeptOnASecondTryIsSentOnce(t *testing.T) {
	r := newRig(t)
	h := &oneUseLink{linkHandler: linkHandler{prefix: "u_", tenant: r.tenant, known: map[string]bool{"u_abc": true}}}
	require.NoError(t, r.c.RegisterStart(h))
	r.api.Chat(telegramtest.Chat{ID: 800, Type: "private"})
	r.st.failHandled = 1
	code, _ := r.platformUpdate(r.platform.Update(800, alice, "/start u_abc"))
	assert.Equal(t, http.StatusOK, code)
	require.Len(t, r.platform.Sent(), 1)
	assert.Equal(t, "Linked, Alice", r.platform.Sent()[0].Text)
	assert.Len(t, h.got, 1, "the handler ran once")
}

// A reply that Telegram refused for now (429) is sent again by the redelivery, from the result that was kept: the
// handler, which consumed the one-use link, is not asked again.
func TestAStartReplyThatWasRateLimitedIsSentAgain(t *testing.T) {
	r := newRig(t)
	h := &oneUseLink{linkHandler: linkHandler{prefix: "u_", tenant: r.tenant, known: map[string]bool{"u_abc": true}}}
	require.NoError(t, r.c.RegisterStart(h))
	r.api.Chat(telegramtest.Chat{ID: 800, Type: "private"})
	r.platform.Fail(telegramtest.Failure{Method: "sendMessage", Code: 429, Description: "Too Many Requests", RetryAfter: 1})
	u := r.platform.Update(800, alice, "/start u_abc")
	code, _ := r.platformUpdate(u)
	assert.Equal(t, http.StatusServiceUnavailable, code)
	assert.Empty(t, r.platform.Sent())
	code, _ = r.platformUpdate(u)
	assert.Equal(t, http.StatusOK, code)
	require.Len(t, r.platform.Sent(), 1)
	assert.Equal(t, "Linked, Alice", r.platform.Sent()[0].Text, "the kept reply, not \"unknown or used\"")
	assert.Len(t, h.got, 1, "the handler ran once")
	code, _ = r.platformUpdate(u)
	assert.Equal(t, http.StatusOK, code)
	assert.Len(t, r.platform.Sent(), 1, "and a finished update is not answered again")
}

// A chat that blocked the bot cannot be written to ever: the update is finished, not redelivered for ever.
func TestAStartReplyToABlockedChatIsGivenUp(t *testing.T) {
	r := newRig(t)
	h := &linkHandler{prefix: "u_", tenant: r.tenant, known: map[string]bool{"u_abc": true}}
	require.NoError(t, r.c.RegisterStart(h))
	r.api.Chat(telegramtest.Chat{ID: 800, Type: "private"})
	r.platform.Fail(telegramtest.Failure{Method: "sendMessage", Code: 403, Description: "Forbidden: bot was blocked by the user"})
	code, _ := r.platformUpdate(r.platform.Update(800, alice, "/start u_abc"))
	assert.Equal(t, http.StatusOK, code)
}

// A redelivery that overlaps the first request while it sends the reply does not send it again: the reply's
// delivery is claimed, and the redelivery is asked to come back.
func TestAnOverlappingStartDeliveryDoesNotSendTheReplyTwice(t *testing.T) {
	r := newRig(t)
	h := &linkHandler{prefix: "u_", tenant: r.tenant, known: map[string]bool{"u_abc": true}}
	require.NoError(t, r.c.RegisterStart(h))
	r.api.Chat(telegramtest.Chat{ID: 800, Type: "private"})
	u := r.platform.Update(800, alice, "/start u_abc")
	var overlapped int
	var second int
	r.platform.After(func(method string) {
		if method != "sendMessage" || overlapped > 0 {
			return
		}
		overlapped++ // Telegram has the message; the first request has not recorded it yet
		second, _ = r.platformUpdate(u)
	})
	code, _ := r.platformUpdate(u)
	assert.Equal(t, http.StatusOK, code)
	assert.Equal(t, http.StatusServiceUnavailable, second, "the overlapping delivery is asked to come back")
	assert.Len(t, r.platform.Sent(), 1, "the reply was sent once")
	assert.Len(t, h.got, 1)
	code, _ = r.platformUpdate(u)
	assert.Equal(t, http.StatusOK, code, "and the finished update is recognised")
	assert.Len(t, r.platform.Sent(), 1)
}

// Telegram's update ids are those of one bot: a bot that replaces the old one behind the same route starts its
// numbers again, and its first /start is not taken for a completed update of the old bot.
func TestAReplacementBotsStartIsNotSkippedForTheSameUpdateID(t *testing.T) {
	r := newRig(t)
	h := &linkHandler{prefix: "u_", tenant: r.tenant, known: map[string]bool{"u_abc": true, "u_def": true}}
	require.NoError(t, r.c.RegisterStart(h))
	r.api.Chat(telegramtest.Chat{ID: 800, Type: "private"})
	first := r.platform.Update(800, alice, "/start u_abc")
	code, _ := r.platformUpdate(first)
	assert.Equal(t, http.StatusOK, code)
	require.Len(t, r.platform.Sent(), 1)

	next := "333333:CCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCC"
	nb := r.api.Bot(next, "next_platform_bot")
	u := nb.Update(800, alice, "/start u_def")
	require.Equal(t, first["update_id"], u["update_id"], "the same numeric id on the other bot")
	code, _ = r.post(r.platformConn(), connectors.Credentials{"token": next}, r.secret(r.platformConn(), next), u)
	assert.Equal(t, http.StatusOK, code)
	assert.Len(t, h.got, 2, "the replacement's link was handled")
	require.Len(t, nb.Sent(), 1, "and answered")
	assert.Equal(t, "Linked, Alice", nb.Sent()[0].Text)
}

// A handler that outlasts its claim is fenced: its result is not recorded over the successor's, and the update
// is asked to come back, so the reply of the claimant that counts is the only one.
func TestAStartHandlerThatLostItsClaimDoesNotOverwriteItsSuccessor(t *testing.T) {
	r := newRig(t)
	h := &numbered{linkHandler: linkHandler{prefix: "u_", tenant: r.tenant, known: map[string]bool{"u_abc": true}}}
	require.NoError(t, r.c.RegisterStart(h))
	r.api.Chat(telegramtest.Chat{ID: 800, Type: "private"})
	u := r.platform.Update(800, alice, "/start u_abc")
	var second int
	h.during = func() {
		r.st.abandon() // the claim of the running handler lapses
		second, _ = r.platformUpdate(u)
	}
	code, _ := r.platformUpdate(u)
	assert.Equal(t, http.StatusServiceUnavailable, code, "the stale claimant's write is refused")
	assert.Equal(t, http.StatusOK, second, "the successor completed")
	require.Len(t, r.platform.Sent(), 1, "one reply")
	assert.Equal(t, "run 2", r.platform.Sent()[0].Text, "the reply of the successor, not of the handler that lost its claim")
	code, _ = r.platformUpdate(u)
	assert.Equal(t, http.StatusOK, code)
	require.Len(t, r.platform.Sent(), 1)
	assert.Equal(t, "run 2", r.platform.Sent()[0].Text, "and the replay does not bring the stale result back")
	// The record itself: the successor's result, finished, and nothing of the stale run in it.
	r.st.mu.Lock()
	defer r.st.mu.Unlock()
	require.Len(t, r.st.actions, 1)
	for _, a := range r.st.actions {
		assert.Equal(t, connectors.InboundDone, a.State, "the update is finished")
		assert.Equal(t, "run 2", a.Reply, "with the successor's reply, not the stale handler's")
	}
}

// numbered answers "run N" to its Nth call, and runs during once, inside its first call.
type numbered struct {
	linkHandler
	calls  int
	during func()
}

func (h *numbered) Handle(_ context.Context, u telegram.StartUpdate) (string, error) {
	h.calls++
	n := h.calls
	if fn := h.during; fn != nil {
		h.during = nil
		fn()
		n = 1 // this call started first: its reply is "run 1" even though the successor ran inside it
	}
	h.got = append(h.got, u)
	return "run " + strconv.Itoa(n), nil
}

// handle runs Handle without the secret check: the update of a bot as the engine hands it over.
func (r *rig) handle(creds connectors.Credentials, update map[string]any) []engine.InboundEvent {
	r.t.Helper()
	body, err := json.Marshal(update)
	require.NoError(r.t, err)
	evs, err := r.c.Handle(context.Background(), httptest.NewRecorder(), body, r.conn, creds)
	require.NoError(r.t, err)
	return evs
}

// Telegram numbers the updates of each bot: the bot that replaces another behind the same connection starts at
// 1 again, and its message is not the old bot's replay. A new token of the same bot is.
func TestInboundDedupKeysBelongToTheBot(t *testing.T) {
	r := newRig(t)
	r.api.Chat(telegramtest.Chat{ID: 700, Type: "private"})
	next := "444444:DDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDD"
	r.api.Bot(next, "next_tenant_bot")
	u := r.bot.Update(700, alice, "hello")
	old := r.handle(r.creds(), u)
	replaced := r.handle(connectors.Credentials{"token": next}, u)
	require.Len(t, old, 1)
	require.Len(t, replaced, 1)
	assert.NotEqual(t, old[0].DedupKey, replaced[0].DedupKey, "the same number of another bot is another update")
	refreshed := r.handle(connectors.Credentials{"token": "111111:ZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZ"}, u)
	require.Len(t, refreshed, 1)
	assert.Equal(t, old[0].DedupKey, refreshed[0].DedupKey, "a new token of the same bot is the same bot: a replay is recognised")
}

// A press's acknowledgement that failed for a moment is tried again: Telegram does not leave the spinner.
func TestAPressAcknowledgementIsRetried(t *testing.T) {
	r := newRig(t)
	m := r.asked()
	r.in.open[askToken] = r.tenant
	u := r.pressUpdate(m, alice, 0)
	_, evs := r.tenantUpdate(u)
	require.Len(t, evs, 1)
	r.bot.Fail(telegramtest.Failure{Method: "answerCallbackQuery", Code: 502, Description: "Bad Gateway"})
	require.NoError(t, r.c.AnswerPublished(context.Background(), evs[0], r.conn, r.creds()))
	queryID := u["callback_query"].(map[string]any)["id"].(string)
	_, answered := r.bot.Answered(queryID)
	assert.True(t, answered, "acknowledged by the second try")
}

// An answer that is not a Bot API answer with a message is not a sent message: the outcome is unknown, not
// "sent" with ids of zero.
func TestAnAnswerWithoutAMessageIsNotASend(t *testing.T) {
	for name, body := range map[string]string{
		"no result":   `{"ok":true}`,
		"null result": `{"ok":true,"result":null}`,
		"no code":     `{}`,
	} {
		t.Run(name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				if name == "no code" {
					w.WriteHeader(http.StatusBadGateway)
				}
				_, _ = w.Write([]byte(body))
			}))
			defer srv.Close()
			c := telegram.New(telegram.Config{APIURL: srv.URL})
			_, err := c.Send(context.Background(), catalog.ConnectionInfo{}, connectors.Credentials{"token": tenantToken}, telegram.Message{ChatID: 700, Text: "hi"})
			require.Error(t, err)
			var refused *catalog.CallError
			assert.NotErrorAs(t, err, &refused, "not a refusal: the message may have been sent")
		})
	}
}

// A file belongs to the bot that got it: another bot cannot resolve its id or download its path, the owner can.
func TestAFileCanBeReachedOnlyByTheBotThatGotIt(t *testing.T) {
	r := newRig(t)
	u := r.documentUpdate(700, "", "report.pdf", "application/pdf", []byte("%PDF"))
	fileID := u["message"].(map[string]any)["document"].(map[string]any)["file_id"].(string)
	get := func(token, method, path string, query url.Values) (int, string) {
		var resp *http.Response
		var err error
		if method == "" {
			resp, err = http.Get(r.api.URL() + "/file/bot" + token + "/" + path) //nolint:noctx // a local fake
		} else {
			resp, err = http.Post(r.api.URL()+"/bot"+token+"/"+method, "application/json", strings.NewReader(`{"file_id":"`+query.Get("file_id")+`"}`)) //nolint:noctx // a local fake
		}
		require.NoError(t, err)
		defer func() { _ = resp.Body.Close() }()
		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		return resp.StatusCode, string(body)
	}
	code, body := get(tenantToken, "getFile", "", url.Values{"file_id": {fileID}})
	require.Equal(t, http.StatusOK, code)
	var meta struct {
		Result struct {
			FilePath string `json:"file_path"`
		} `json:"result"`
	}
	require.NoError(t, json.Unmarshal([]byte(body), &meta))
	require.NotEmpty(t, meta.Result.FilePath)

	code, _ = get(platformToken, "getFile", "", url.Values{"file_id": {fileID}})
	assert.Equal(t, http.StatusBadRequest, code, "another bot cannot resolve the file id")
	code, _ = get(platformToken, "", meta.Result.FilePath, nil)
	assert.Equal(t, http.StatusNotFound, code, "nor download its path")
	code, content := get(tenantToken, "", meta.Result.FilePath, nil)
	assert.Equal(t, http.StatusOK, code, "the owner downloads it")
	assert.Equal(t, "%PDF", content)
}

// Only Telegram's refusal of a file by its size is "too large": a 400 about the file being unavailable for now is
// a failed download, whatever the size says.
func TestOnlyASizeRefusalMakesAFileTooLarge(t *testing.T) {
	for name, tc := range map[string]struct{ description, want string }{
		"unavailable for now": {"Bad Request: wrong file_id or the file is temporarily unavailable", telegram.FileDownload},
		"too big":             {"Bad Request: file is too big", telegram.FileTooLarge},
	} {
		t.Run(name, func(t *testing.T) {
			r := newRig(t)
			u := r.documentUpdate(700, "", "report.pdf", "application/pdf", []byte("x"))
			u["message"].(map[string]any)["document"].(map[string]any)["file_size"] = 0 // the size is not given
			r.bot.Fail(telegramtest.Failure{Method: "getFile", Code: 400, Description: tc.description})
			_, evs := r.tenantUpdate(u)
			require.Len(t, evs, 1)
			f := payloadOf(t, evs[0])["files"].([]any)[0].(map[string]any)
			assert.Equal(t, tc.want, f["error"])
		})
	}
}

// The memo of saved uploads is of one tenant: the same update reaching another tenant (the bot moved) saves its own.
func TestTheUploadMemoIsOfOneTenant(t *testing.T) {
	r := newRig(t)
	u := r.documentUpdate(700, "", "report.pdf", "application/pdf", []byte("x"))
	body, err := json.Marshal(u)
	require.NoError(t, err)
	other := r.conn
	other.TenantID = uuid.New()
	conns := []connectors.Connection{r.conn, r.conn, other}
	uploadIDs := make([]string, 0, len(conns))
	for _, conn := range conns {
		evs, err := r.c.Handle(context.Background(), httptest.NewRecorder(), body, conn, r.creds())
		require.NoError(t, err)
		require.Len(t, evs, 1, "every delivery has its event")
		uploadIDs = append(uploadIDs, payloadOf(t, evs[0])["files"].([]any)[0].(map[string]any)["upload_id"].(string))
	}
	assert.Equal(t, uploadIDs[0], uploadIDs[1], "the replay within a tenant reuses its upload")
	assert.NotEqual(t, uploadIDs[0], uploadIDs[2], "another tenant gets an upload of its own")
	assert.Len(t, r.up.files, 2, "one per tenant; the replay within a tenant reuses")
}

// A platform bot that another service already uses keeps its webhook through the application's start: the
// platform bot then only sends. The start replaces it only when it is told to.
func TestStartPlatformLeavesAForeignWebhook(t *testing.T) {
	r := newRig(t)
	r.platform.SetWebhook("https://other.example.org/hook", "s")
	err := r.c.StartPlatform(context.Background(), r.platformCreds())
	require.Error(t, err)
	assert.Equal(t, "https://other.example.org/hook", r.platform.Webhook().URL, "the other service's webhook is not replaced")

	takeover := telegram.New(telegram.Config{PlatformToken: platformToken, PublicOrigin: origin, APIURL: r.api.URL(), ReplacePlatformWebhook: true})
	require.NoError(t, takeover.StartPlatform(context.Background(), r.platformCreds()))
	assert.Equal(t, origin+"/automations/hooks/platform/telegram_bot", r.platform.Webhook().URL)
	// The own webhook is set again at each start.
	require.NoError(t, r.c.StartPlatform(context.Background(), r.platformCreds()))
}

// keyedLink is a one-use link whose handler keeps its result under the update's key when it spends the link, as
// the documentation asks of a handler, and returns that result when it is asked again.
type keyedLink struct {
	linkHandler
	used       bool
	saved      map[string]string
	failBefore error // returned before the link is spent
	asked      int
	tenants    map[string]uuid.UUID // whose update each kept result was
	lookupErr  error                // TenantOfUpdate fails with it
}

// TenantOfUpdate is telegram.StartTenantByKey: whose update a kept result was.
func (h *keyedLink) TenantOfUpdate(_ context.Context, key string) (uuid.UUID, bool, error) {
	if h.lookupErr != nil {
		return uuid.Nil, false, h.lookupErr
	}
	t, ok := h.tenants[key]
	return t, ok, nil
}

func (h *keyedLink) TenantOf(ctx context.Context, payload string) (uuid.UUID, bool, error) {
	if h.used {
		return uuid.Nil, false, nil // the link is spent
	}
	return h.linkHandler.TenantOf(ctx, payload)
}

func (h *keyedLink) Handle(ctx context.Context, u telegram.StartUpdate) (string, error) {
	h.asked++
	if reply, ok := h.saved[u.UpdateKey]; ok {
		return reply, nil // a rerun of an update that spent the link: the first result
	}
	if h.failBefore != nil {
		err := h.failBefore
		h.failBefore = nil
		return "", err
	}
	reply, err := h.linkHandler.Handle(ctx, u)
	h.used = true
	if h.saved == nil {
		h.saved, h.tenants = map[string]string{}, map[string]uuid.UUID{}
	}
	h.saved[u.UpdateKey], h.tenants[u.UpdateKey] = reply, u.TenantID
	return reply, err
}

// A handler that keeps its result under the update's key gets the first reply to the user even when the
// connector could not keep the result at all: the redelivery (after the abandoned claim) asks the handler again, with
// no tenant for the spent link, and the handler answers with what it kept.
func TestAHandlerThatKeepsItsResultByKeyRecoversAfterTheResultWasLost(t *testing.T) {
	r := newRig(t)
	usage := &usageLog{}
	r.c = telegram.New(telegram.Config{PlatformToken: platformToken, PublicOrigin: origin, Inbound: r.in, APIURL: r.api.URL(), Usage: usage, Connections: r.st})
	h := &keyedLink{linkHandler: linkHandler{prefix: "u_", tenant: r.tenant, known: map[string]bool{"u_abc": true}}}
	require.NoError(t, r.c.RegisterStart(h))
	r.api.Chat(telegramtest.Chat{ID: 800, Type: "private"})
	r.st.failHandled = 3 // all three writes of the result fail
	u := r.platform.Update(800, alice, "/start u_abc")
	code, _ := r.platformUpdate(u)
	assert.Equal(t, http.StatusServiceUnavailable, code)
	assert.Empty(t, r.platform.Sent())
	r.st.abandon()
	code, _ = r.platformUpdate(u)
	assert.Equal(t, http.StatusOK, code)
	require.Len(t, r.platform.Sent(), 1)
	assert.Equal(t, "Linked, Alice", r.platform.Sent()[0].Text, "the reply of the run that spent the link")
	assert.Equal(t, 2, h.asked, "asked again, and answered from what it kept")
	assert.Len(t, h.got, 1, "the link was spent once")
	// The platform bot's reply is counted for the tenant of the first run, though the rerun had no tenant for the link.
	require.Len(t, usage.entries, 1)
	assert.Equal(t, r.tenant, usage.entries[0].TenantID)
	assert.Equal(t, "platform:telegram_bot", usage.entries[0].ConnectionID)
}

// usageLog records what the connector counts.
type usageLog struct{ entries []catalog.UsageEntry }

func (u *usageLog) Record(_ context.Context, e catalog.UsageEntry) error {
	u.entries = append(u.entries, e)
	return nil
}

// A handler that fails before it spends the link is simply run again by the redelivery.
func TestAHandlerThatFailsBeforeSpendingTheLinkIsRunAgain(t *testing.T) {
	r := newRig(t)
	h := &keyedLink{linkHandler: linkHandler{prefix: "u_", tenant: r.tenant, known: map[string]bool{"u_abc": true}}, failBefore: assert.AnError}
	require.NoError(t, r.c.RegisterStart(h))
	r.api.Chat(telegramtest.Chat{ID: 800, Type: "private"})
	u := r.platform.Update(800, alice, "/start u_abc")
	code, _ := r.platformUpdate(u)
	assert.Equal(t, http.StatusServiceUnavailable, code)
	assert.Empty(t, r.platform.Sent())
	code, _ = r.platformUpdate(u)
	assert.Equal(t, http.StatusOK, code, "released at once: the redelivery runs the handler")
	require.Len(t, r.platform.Sent(), 1)
	assert.Equal(t, "Linked, Alice", r.platform.Sent()[0].Text)
	assert.Len(t, h.got, 1, "the link was spent once, by the run that got to it")
}

// When the handler cannot say whose a kept result was, the update is not answered and counted for nobody: the claim
// is given up and the update comes again.
func TestAFailedTenantLookupOfAKeptResultIsRetried(t *testing.T) {
	r := newRig(t)
	usage := &usageLog{}
	r.c = telegram.New(telegram.Config{PlatformToken: platformToken, PublicOrigin: origin, Inbound: r.in, APIURL: r.api.URL(), Usage: usage, Connections: r.st})
	h := &keyedLink{linkHandler: linkHandler{prefix: "u_", tenant: r.tenant, known: map[string]bool{"u_abc": true}}}
	require.NoError(t, r.c.RegisterStart(h))
	r.api.Chat(telegramtest.Chat{ID: 800, Type: "private"})
	// A rerun of a spent link: the handler kept the result, and the lookup of its tenant fails.
	u := r.platform.Update(800, alice, "/start u_abc")
	key := "222222:" + strconv.FormatInt(u["update_id"].(int64), 10)
	h.used, h.saved, h.tenants = true, map[string]string{key: "Linked, Alice"}, map[string]uuid.UUID{key: r.tenant}
	h.lookupErr = assert.AnError
	code, _ := r.platformUpdate(u)
	assert.Equal(t, http.StatusServiceUnavailable, code)
	assert.Empty(t, r.platform.Sent(), "nothing is sent that would be counted for nobody")
	assert.Equal(t, 0, h.asked, "the handler has not run")
	h.lookupErr = nil
	code, _ = r.platformUpdate(u)
	assert.Equal(t, http.StatusOK, code, "the claim was given up: the redelivery goes through")
	require.Len(t, r.platform.Sent(), 1)
	require.Len(t, usage.entries, 1)
	assert.Equal(t, r.tenant, usage.entries[0].TenantID)
}
