package telegram_test

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/pkg/automation/catalog"
	"github.com/iota-uz/iota-sdk/pkg/automation/connectors"
	"github.com/iota-uz/iota-sdk/pkg/automation/connectors/telegram"
	"github.com/iota-uz/iota-sdk/pkg/automation/connectors/telegram/telegramtest"
	"github.com/iota-uz/iota-sdk/pkg/automation/engine"
	"github.com/iota-uz/iota-sdk/pkg/automation/graph"
)

type usage struct {
	mu      sync.Mutex
	entries []catalog.UsageEntry
}

func (u *usage) Record(_ context.Context, e catalog.UsageEntry) error {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.entries = append(u.entries, e)
	return nil
}

func (r *rig) node(typ string) catalog.CallNode {
	for _, n := range r.c.Nodes() {
		if n.Spec().Type == typ {
			return n.(catalog.CallNode)
		}
	}
	r.t.Fatalf("no node %s", typ)
	return nil
}

func (r *rig) input(params map[string]any, u *usage) catalog.CallInput {
	in := catalog.CallInput{
		TenantID: r.tenant, RunID: uuid.New(), NodeID: "n", Attempt: 1, Params: params,
		Connection:  &catalog.ConnectionInfo{ID: r.conn.ID.String(), Kind: telegram.Kind, Name: "bot"},
		Credentials: r.creds(), Deadline: time.Now().Add(time.Minute),
	}
	if u != nil {
		in.Usage = u
	}
	return in
}

func num(n int64) json.Number { return json.Number(jsonInt(n)) }

func jsonInt(n int64) string {
	b, _ := json.Marshal(n)
	return string(b)
}

func TestSendDeliversTheMessageAndCountsIt(t *testing.T) {
	r := newRig(t)
	r.api.Chat(telegramtest.Chat{ID: 700, Type: "private"})
	u := &usage{}
	res, err := r.node(telegram.NodeSend).Execute(context.Background(), r.input(map[string]any{
		"chat_id": num(700), "text": "Order <b>1</b>", "parse_mode": "HTML", "silent": true,
		"url_buttons": []any{map[string]any{"text": "Open", "url": "https://erp.example.com/o/1"}},
	}, u))
	require.NoError(t, err)
	assert.Equal(t, "sent", res.Outcome)
	assert.Equal(t, "700", res.Output["chat_id"])
	assert.NotEmpty(t, res.Output["message_id"])
	sent := r.bot.Sent()
	require.Len(t, sent, 1)
	assert.Equal(t, "Order <b>1</b>", sent[0].Text)
	assert.Equal(t, "HTML", sent[0].Parse)
	assert.True(t, sent[0].Silent)
	assert.Equal(t, [][]telegramtest.Button{{{Text: "Open", URL: "https://erp.example.com/o/1"}}}, sent[0].Markup)
	require.Len(t, u.entries, 1)
	assert.Equal(t, "message", u.entries[0].Unit)
	assert.Equal(t, int64(1), u.entries[0].Quantity)
	assert.Equal(t, r.conn.ID.String(), u.entries[0].ConnectionID, "a tenant's own bot")
	assert.Equal(t, r.tenant, u.entries[0].TenantID)

	// The platform bot is counted apart.
	in := r.input(map[string]any{"chat_id": num(700), "text": "via platform"}, u)
	in.Connection = &catalog.ConnectionInfo{ID: "platform:telegram_bot", Kind: telegram.Kind, Platform: true}
	in.Credentials = r.platformCreds()
	_, err = r.node(telegram.NodeSend).Execute(context.Background(), in)
	require.NoError(t, err)
	require.Len(t, u.entries, 2)
	assert.Equal(t, "platform:telegram_bot", u.entries[1].ConnectionID)
}

func TestSendAcceptsAnIDFromAnExpressionAndAUsername(t *testing.T) {
	r := newRig(t)
	r.api.Chat(telegramtest.Chat{ID: -1002, Type: "supergroup", Username: "salesteam", Title: "Sales"})
	_, err := r.node(telegram.NodeSend).Execute(context.Background(), r.input(map[string]any{"chat_id": "-1002", "text": "a"}, nil))
	require.NoError(t, err)
	_, err = r.node(telegram.NodeSend).Execute(context.Background(), r.input(map[string]any{"chat_id": "@salesteam", "text": "b"}, nil))
	require.NoError(t, err)
	assert.Len(t, r.bot.Sent(), 2)
	_, err = r.node(telegram.NodeSend).Execute(context.Background(), r.input(map[string]any{"chat_id": "nonsense", "text": "c"}, nil))
	var ce *catalog.CallError
	require.True(t, errors.As(err, &ce))
	assert.Equal(t, catalog.CallInvalidRequest, ce.Code)
}

func TestSendToATopicAndToTheGeneralChat(t *testing.T) {
	r := newRig(t)
	r.api.Chat(telegramtest.Chat{ID: -1003, Type: "supergroup", Forum: true, Topics: map[int64]bool{77: true}})
	ctx := context.Background()
	_, err := r.node(telegram.NodeSend).Execute(ctx, r.input(map[string]any{"chat_id": num(-1003), "thread_id": num(77), "text": "in the topic"}, nil))
	require.NoError(t, err)
	_, err = r.node(telegram.NodeSend).Execute(ctx, r.input(map[string]any{"chat_id": num(-1003), "text": "in general"}, nil))
	require.NoError(t, err)
	sent := r.bot.Sent()
	require.Len(t, sent, 2)
	assert.Equal(t, int64(77), sent[0].ThreadID, "delivered into the topic")
	assert.Equal(t, int64(0), sent[1].ThreadID, "without thread_id: the general chat")

	_, err = r.node(telegram.NodeSend).Execute(ctx, r.input(map[string]any{"chat_id": num(-1003), "thread_id": num(99), "text": "x"}, nil))
	var ce *catalog.CallError
	require.True(t, errors.As(err, &ce))
	assert.Equal(t, catalog.CallRejected, ce.Code, "a topic that does not exist")
}

func TestSendWithFiles(t *testing.T) {
	r := newRig(t)
	r.api.Chat(telegramtest.Chat{ID: 700, Type: "private"})
	r.up.files = map[string]telegram.UploadFile{
		"u1": {Name: "report.pdf", Mime: "application/pdf", Data: []byte("%PDF")},
		"u2": {Name: "pic.png", Mime: "image/png", Data: []byte("PNG")},
		"u3": {Name: "b.txt", Mime: "text/plain", Data: []byte("b")},
	}
	ctx := context.Background()
	res, err := r.node(telegram.NodeSend).Execute(ctx, r.input(map[string]any{"chat_id": num(700), "text": "the report", "files": []any{"u1"}}, nil))
	require.NoError(t, err)
	assert.Equal(t, "sent", res.Outcome)
	sent := r.bot.Sent()
	require.Len(t, sent, 1)
	assert.Equal(t, []string{"report.pdf"}, sent[0].Files)
	assert.Equal(t, "the report", sent[0].Caption)
	assert.Len(t, r.bot.CallsOf("sendDocument"), 1)

	_, err = r.node(telegram.NodeSend).Execute(ctx, r.input(map[string]any{"chat_id": num(700), "text": "pic", "files": []any{"u2"}}, nil))
	require.NoError(t, err)
	assert.Len(t, r.bot.CallsOf("sendPhoto"), 1, "an image goes as a photo")

	res, err = r.node(telegram.NodeSend).Execute(ctx, r.input(map[string]any{"chat_id": num(700), "text": "two", "files": []any{"u1", "u3"}}, nil))
	require.NoError(t, err)
	assert.Len(t, r.bot.CallsOf("sendMediaGroup"), 1)
	assert.NotEmpty(t, res.Output["message_id"])

	_, err = r.node(telegram.NodeSend).Execute(ctx, r.input(map[string]any{"chat_id": num(700), "text": "x", "files": []any{"missing"}}, nil))
	var ce *catalog.CallError
	require.True(t, errors.As(err, &ce))
	assert.Equal(t, catalog.CallInvalidRequest, ce.Code)
}

func TestBlockedBotIsRejectedAndTheConnectionStaysConnected(t *testing.T) {
	r := newRig(t)
	r.api.Chat(telegramtest.Chat{ID: 700, Type: "private"})
	r.bot.Block(700)
	_, err := r.node(telegram.NodeSend).Execute(context.Background(), r.input(map[string]any{"chat_id": num(700), "text": "hi"}, nil))
	var ce *catalog.CallError
	require.True(t, errors.As(err, &ce))
	assert.Equal(t, catalog.CallRejected, ce.Code)
	assert.Empty(t, r.st.status, "the connection is not touched")

	_, err = r.node(telegram.NodeSend).Execute(context.Background(), r.input(map[string]any{"chat_id": num(12345), "text": "hi"}, nil))
	require.True(t, errors.As(err, &ce))
	assert.Equal(t, catalog.CallRejected, ce.Code, "chat not found")
}

func TestUnauthorizedMarksTheConnectionNeedsReauth(t *testing.T) {
	r := newRig(t)
	r.api.Chat(telegramtest.Chat{ID: 700, Type: "private"})
	r.bot.Fail(telegramtest.Failure{Method: "sendMessage", Code: 401, Description: "Unauthorized"})
	_, err := r.node(telegram.NodeSend).Execute(context.Background(), r.input(map[string]any{"chat_id": num(700), "text": "hi"}, nil))
	var ce *catalog.CallError
	require.True(t, errors.As(err, &ce))
	assert.Equal(t, catalog.CallUnauthorized, ce.Code)
	assert.Equal(t, connectors.StatusNeedsReauth, r.st.status)
	assert.Equal(t, "reauth", r.st.reason)
}

func TestRateLimitIsARetryAfterNotASecondMessage(t *testing.T) {
	r := newRig(t)
	r.api.Chat(telegramtest.Chat{ID: 700, Type: "private"})
	r.bot.Fail(telegramtest.Failure{Method: "sendMessage", Code: 429, Description: "Too Many Requests: retry after 3", RetryAfter: 3})
	u := &usage{}
	res, err := r.node(telegram.NodeSend).Execute(context.Background(), r.input(map[string]any{"chat_id": num(700), "text": "hi"}, u))
	require.NoError(t, err)
	assert.Equal(t, 3*time.Second, res.RetryAfter)
	assert.Empty(t, res.Outcome)
	assert.Empty(t, r.bot.Sent(), "nothing was sent")
	assert.Empty(t, u.entries, "and nothing counted")
	// The engine repeats after the delay: now it goes through, once.
	res, err = r.node(telegram.NodeSend).Execute(context.Background(), r.input(map[string]any{"chat_id": num(700), "text": "hi"}, u))
	require.NoError(t, err)
	assert.Equal(t, "sent", res.Outcome)
	assert.Len(t, r.bot.Sent(), 1)
}

// A lost answer after the send is an error whose outcome is unknown, never a result: the engine
// records outcome_unknown instead of sending again. Neither the error nor its text carries the token.
func TestALostAnswerIsAnUnknownOutcomeWithoutTheToken(t *testing.T) {
	r := newRig(t)
	r.api.Chat(telegramtest.Chat{ID: 700, Type: "private"})
	r.bot.Fail(telegramtest.Failure{Method: "sendMessage", Lost: true})
	_, err := r.node(telegram.NodeSend).Execute(context.Background(), r.input(map[string]any{"chat_id": num(700), "text": "hi"}, nil))
	require.Error(t, err)
	var ce *catalog.CallError
	assert.False(t, errors.As(err, &ce), "not a refusal")
	assert.Len(t, r.bot.Sent(), 1, "the message was sent")
	assert.NotContains(t, err.Error(), tenantToken)
	assert.NotContains(t, err.Error(), "AAAAAAAA")

	// A refused connection to the API: the URL (with the token) is not in the error either.
	closed := telegram.New(telegram.Config{APIURL: "http://127.0.0.1:1"})
	_, err = closed.Send(context.Background(), catalog.ConnectionInfo{}, catalog.Credentials{"token": tenantToken}, telegram.Message{ChatID: 1, Text: "x"})
	require.Error(t, err)
	assert.NotContains(t, err.Error(), tenantToken)
	assert.NotContains(t, err.Error(), "127.0.0.1")
}

func TestSendRefusesAnOverlongText(t *testing.T) {
	r := newRig(t)
	r.api.Chat(telegramtest.Chat{ID: 700, Type: "private"})
	long := make([]rune, 4097)
	for i := range long {
		long[i] = 'я'
	}
	_, err := r.node(telegram.NodeSend).Execute(context.Background(), r.input(map[string]any{"chat_id": num(700), "text": string(long)}, nil))
	var ce *catalog.CallError
	require.True(t, errors.As(err, &ce))
	assert.Equal(t, catalog.CallInvalidRequest, ce.Code)
	assert.Empty(t, r.bot.CallsOf("sendMessage"), "refused before it is sent")
}

func TestClientSendForOtherCode(t *testing.T) {
	r := newRig(t)
	r.api.Chat(telegramtest.Chat{ID: 700, Type: "private"})
	var c telegram.Client = r.c
	ref, err := c.Send(context.Background(), catalog.ConnectionInfo{ID: r.conn.ID.String(), Kind: telegram.Kind}, catalog.Credentials{"token": tenantToken},
		telegram.Message{ChatID: 700, Text: "hello", Buttons: [][]telegram.Button{{{Text: "Go", URL: "https://x.example.com"}}}})
	require.NoError(t, err)
	assert.Equal(t, int64(700), ref.ChatID)
	assert.NotZero(t, ref.MessageID)
	_, err = c.Send(context.Background(), catalog.ConnectionInfo{}, catalog.Credentials{"token": tenantToken},
		telegram.Message{ChatID: 700, Text: "x", Buttons: [][]telegram.Button{{{Text: "both", URL: "https://x", CallbackData: "d"}}}})
	var ce *catalog.CallError
	require.True(t, errors.As(err, &ce), "a button has exactly one of URL and CallbackData")

	r.bot.Fail(telegramtest.Failure{Method: "sendMessage", Code: 429, RetryAfter: 2})
	_, err = c.Send(context.Background(), catalog.ConnectionInfo{}, catalog.Credentials{"token": tenantToken}, telegram.Message{ChatID: 700, Text: "x"})
	var ra *telegram.RetryAfterError
	require.True(t, errors.As(err, &ra))
	assert.Equal(t, 2*time.Second, ra.After)
}

// ---------------------------------------------------------------------------------------------
// ask

const askToken = "AbCdEfGhIjKl"

func (r *rig) ask(params map[string]any) (catalog.CallResult, error) {
	ctx := engine.WithCorrelation(context.Background(), engine.Correlation{Token: askToken})
	return r.node(telegram.NodeAsk).Execute(ctx, r.input(params, nil))
}

func options(n int) []any {
	out := make([]any, n)
	for i := range out {
		out[i] = map[string]any{"outcome": "opt" + string(rune('a'+i)), "label": "Option " + string(rune('A'+i))}
	}
	return out
}

func TestAskSendsButtonsWithTheCorrelationToken(t *testing.T) {
	r := newRig(t)
	r.api.Chat(telegramtest.Chat{ID: 700, Type: "private"})
	res, err := r.ask(map[string]any{"chat_id": num(700), "text": "Approve?", "options": options(3), "columns": num(2)})
	require.NoError(t, err)
	assert.NotEmpty(t, res.Output["message_id"])
	m := r.bot.Sent()[0]
	require.Len(t, m.Markup, 2, "three buttons in two columns: two rows")
	assert.Len(t, m.Markup[0], 2)
	assert.Len(t, m.Markup[1], 1)
	assert.Equal(t, telegramtest.Button{Text: "Option A", CallbackData: "a:" + askToken + ":0"}, m.Markup[0][0])
	assert.Equal(t, "a:"+askToken+":2", m.Markup[1][0].CallbackData)
	for _, row := range m.Markup {
		for _, b := range row {
			assert.LessOrEqual(t, len(b.CallbackData), 64)
		}
	}
	assert.Equal(t, telegram.CallbackData(askToken, 1), "a:"+askToken+":1")

	_, err = r.node(telegram.NodeAsk).Execute(context.Background(), r.input(map[string]any{"chat_id": num(700), "text": "x", "options": options(2)}, nil))
	require.Error(t, err, "no correlation token: not an ask")
}

func TestAskAwaitsAPressAndMapsItToTheOutcome(t *testing.T) {
	r := newRig(t)
	ask := r.node(telegram.NodeAsk).(engine.Asker)
	in := r.input(map[string]any{"timeout": "2h"}, nil)
	aw, err := ask.Await(in, catalog.CallResult{})
	require.NoError(t, err)
	assert.Equal(t, telegram.EventCallback, aw.Event)
	assert.Equal(t, 2*time.Hour, aw.Timeout)
	assert.True(t, aw.Single)
	aw, err = ask.Await(r.input(map[string]any{}, nil), catalog.CallResult{})
	require.NoError(t, err)
	assert.Equal(t, 24*time.Hour, aw.Timeout, "a day by default")
	ev, single := ask.(engine.AnswerEventer).AnswerEvent(in)
	assert.Equal(t, telegram.EventCallback, ev)
	assert.True(t, single)

	params := map[string]any{"options": []any{
		map[string]any{"outcome": "yes", "label": "Yes"}, map[string]any{"outcome": "no", "label": "No"}}}
	payload := json.RawMessage(`{"chatId":"700","messageId":"9","choice":"1","label":"No","from":{"userId":"501","username":"alice","firstName":"Alice"},"answeredAt":"2026-01-02T03:04:05Z"}`)
	outcome, out, err := ask.(engine.ParamAnswerer).AnswerFor(params, payload)
	require.NoError(t, err)
	assert.Equal(t, "no", outcome)
	var got map[string]any
	require.NoError(t, json.Unmarshal(out, &got))
	assert.Equal(t, map[string]any{
		"choice": "no", "label": "No", "message_id": "9", "answered_at": "2026-01-02T03:04:05Z",
		"from": map[string]any{"user_id": "501", "username": "alice", "first_name": "Alice"},
	}, got)

	_, _, err = ask.(engine.ParamAnswerer).AnswerFor(params, json.RawMessage(`{"choice":"7","from":{}}`))
	assert.Error(t, err, "a button the question does not have")
}

// ---------------------------------------------------------------------------------------------
// presses

// asked sends a question with two buttons and returns the message as it was sent.
func (r *rig) asked() telegramtest.Message {
	r.t.Helper()
	r.api.Chat(telegramtest.Chat{ID: 700, Type: "private"})
	_, err := r.ask(map[string]any{"chat_id": num(700), "text": "Approve the order?", "options": options(2)})
	require.NoError(r.t, err)
	return r.bot.Sent()[0]
}

func TestPressOfAnOpenQuestionIsPublishedWithItsCorrelation(t *testing.T) {
	r := newRig(t)
	m := r.asked()
	r.in.open[askToken] = r.tenant
	u := r.pressUpdate(m, alice, 1)
	code, evs := r.tenantUpdate(u)
	require.Equal(t, 200, code)
	require.Len(t, evs, 1)
	ev := evs[0]
	assert.Equal(t, telegram.EventCallback, ev.Event)
	assert.Equal(t, askToken, ev.Correlation)
	assert.Equal(t, r.tenant, ev.TenantID)
	assert.Equal(t, r.conn.ID.String()+":700", ev.Subject.Key)
	p := payloadOf(t, ev)
	assert.Equal(t, "1", p["choice"])
	assert.Equal(t, "Option B", p["label"], "the label is read from the keyboard of the message")
	assert.Equal(t, "700", p["chatId"])
	assert.Equal(t, map[string]any{"userId": "501", "username": "alice", "firstName": "Alice"}, p["from"])
	assert.NotEmpty(t, p["answeredAt"])
	// Nothing is acknowledged or edited before the answer is committed.
	assert.Empty(t, r.bot.CallsOf("answerCallbackQuery"))
	assert.Empty(t, r.bot.CallsOf("editMessageText"))
}

func (r *rig) pressUpdate(m telegramtest.Message, from telegramtest.User, i int) map[string]any {
	r.t.Helper()
	var flat []telegramtest.Button
	for _, row := range m.Markup {
		flat = append(flat, row...)
	}
	return r.bot.PressUpdate(m, from, flat[i].CallbackData)
}

func TestFirstAcceptedPressAcknowledgesRemovesTheKeyboardAndAddsTheChoice(t *testing.T) {
	r := newRig(t)
	m := r.asked()
	r.in.open[askToken] = r.tenant
	u := r.pressUpdate(m, alice, 1)
	_, evs := r.tenantUpdate(u)
	require.Len(t, evs, 1)
	// The engine tells the connector the answer was committed.
	r.c.AnswerPublished(context.Background(), evs[0], r.conn, r.creds())
	queryID := u["callback_query"].(map[string]any)["id"].(string)
	text, answered := r.bot.Answered(queryID)
	assert.True(t, answered, "every press is acknowledged")
	assert.Empty(t, text)
	got, _ := r.bot.Message(700, m.ID)
	assert.Empty(t, got.Markup, "the keyboard is gone")
	assert.Equal(t, "Approve the order?\n\n→ Option B", got.Text, "the same text with the chosen option")
}

func TestAPressOfAClosedQuestionIsAnsweredWithoutAnEvent(t *testing.T) {
	r := newRig(t)
	m := r.asked()
	// The token is not open (answered already, or expired): no event, the press is answered.
	u := r.pressUpdate(m, alice, 0)
	code, evs := r.tenantUpdate(u)
	require.Equal(t, 200, code)
	assert.Empty(t, evs)
	queryID := u["callback_query"].(map[string]any)["id"].(string)
	text, answered := r.bot.Answered(queryID)
	assert.True(t, answered)
	assert.Equal(t, telegram.AlreadyAnswered, text)
	got, _ := r.bot.Message(700, m.ID)
	assert.Empty(t, got.Markup, "the keyboard is taken off at the late press")
	assert.Equal(t, "Approve the order?", got.Text, "and the text is not changed")
}

// A press that loses the race for the single answer is answered by the engine's AnswerClosed.
func TestAPressThatLostTheRaceIsAnsweredAsClosed(t *testing.T) {
	r := newRig(t)
	m := r.asked()
	r.in.open[askToken] = r.tenant
	u := r.pressUpdate(m, alice, 0)
	_, evs := r.tenantUpdate(u)
	require.Len(t, evs, 1)
	r.c.AnswerClosed(context.Background(), evs[0], r.conn, r.creds())
	queryID := u["callback_query"].(map[string]any)["id"].(string)
	text, _ := r.bot.Answered(queryID)
	assert.Equal(t, telegram.AlreadyAnswered, text)
	got, _ := r.bot.Message(700, m.ID)
	assert.Equal(t, "Approve the order?", got.Text, "the text stays")
}

func TestPressOfAnotherTenantsQuestionIsClosedForThisBot(t *testing.T) {
	r := newRig(t)
	m := r.asked()
	r.in.open[askToken] = uuid.New()
	_, evs := r.tenantUpdate(r.pressUpdate(m, alice, 0))
	assert.Empty(t, evs)
}

// On the platform bot the tenant of a press is the question's.
func TestPlatformPressIsOfTheQuestionsTenant(t *testing.T) {
	r := newRig(t)
	r.api.Chat(telegramtest.Chat{ID: 800, Type: "private"})
	ctx := engine.WithCorrelation(context.Background(), engine.Correlation{Token: askToken})
	in := r.input(map[string]any{"chat_id": num(800), "text": "Q?", "options": options(2)}, nil)
	in.Connection = &catalog.ConnectionInfo{ID: "platform:telegram_bot", Kind: telegram.Kind, Platform: true}
	in.Credentials = r.platformCreds()
	_, err := r.node(telegram.NodeAsk).Execute(ctx, in)
	require.NoError(t, err)
	m := r.platform.Sent()[0]
	r.in.open[askToken] = r.tenant
	_, evs := r.platformUpdate(r.platform.PressUpdate(m, alice, m.Markup[0][0].CallbackData))
	require.Len(t, evs, 1)
	assert.Equal(t, r.tenant, evs[0].TenantID)
	assert.Equal(t, "platform:telegram_bot:800", evs[0].Subject.Key)

	// An unknown token: answered, nothing published.
	delete(r.in.open, askToken)
	u := r.platform.PressUpdate(m, alice, m.Markup[1][0].CallbackData)
	_, evs = r.platformUpdate(u)
	assert.Empty(t, evs)
	text, _ := r.platform.Answered(u["callback_query"].(map[string]any)["id"].(string))
	assert.Equal(t, telegram.AlreadyAnswered, text)

	// A press with data that is not an ask's is only acknowledged.
	u = r.platform.PressUpdate(m, alice, "other:data")
	_, evs = r.platformUpdate(u)
	assert.Empty(t, evs)
}

func TestAnAlbumTakesNoButtons(t *testing.T) {
	r := newRig(t)
	r.api.Chat(telegramtest.Chat{ID: 700, Type: "private"})
	r.up.files = map[string]telegram.UploadFile{"a": {Name: "a.txt", Data: []byte("a")}, "b": {Name: "b.txt", Data: []byte("b")}}
	params := map[string]any{"chat_id": num(700), "text": "x", "files": []any{"a", "b"},
		"url_buttons": []any{map[string]any{"text": "Open", "url": "https://x.example.com"}}}
	// Refused when the graph is saved…
	_, issues := r.node(telegram.NodeSend).Spec().Params.Parse(params)
	require.Len(t, issues, 1)
	assert.Equal(t, []any{"url_buttons"}, issues[0].Path)
	// …and before anything is sent.
	_, err := r.node(telegram.NodeSend).Execute(context.Background(), r.input(params, nil))
	var ce *catalog.CallError
	require.True(t, errors.As(err, &ce))
	assert.Equal(t, catalog.CallInvalidRequest, ce.Code)
	assert.Empty(t, r.bot.Sent())
	// One file keeps its buttons.
	params["files"] = []any{"a"}
	_, err = r.node(telegram.NodeSend).Execute(context.Background(), r.input(params, nil))
	require.NoError(t, err)
	assert.Len(t, r.bot.Sent()[0].Markup, 1)
}

// Every message Telegram delivered is counted: an album is several.
func TestUsageCountsEachDeliveredMessage(t *testing.T) {
	r := newRig(t)
	r.api.Chat(telegramtest.Chat{ID: 700, Type: "private"})
	r.up.files = map[string]telegram.UploadFile{"a": {Name: "a.txt", Data: []byte("a")}, "b": {Name: "b.txt", Data: []byte("b")}, "c": {Name: "c.txt", Data: []byte("c")}}
	u := &usage{}
	_, err := r.node(telegram.NodeSend).Execute(context.Background(), r.input(map[string]any{"chat_id": num(700), "text": "x", "files": []any{"a", "b", "c"}}, u))
	require.NoError(t, err)
	require.Len(t, u.entries, 1)
	assert.Equal(t, int64(3), u.entries[0].Quantity)
	assert.Len(t, r.bot.Sent(), 3)
}

// The reply to /start is a message too: counted for the tenant the link belongs to and the bot that sent it.
func TestStartReplyIsCounted(t *testing.T) {
	r := newRig(t)
	u := &usage{}
	r.c = telegram.New(telegram.Config{PlatformToken: platformToken, PublicOrigin: origin, Inbound: r.in, APIURL: r.api.URL(), Usage: u, Connections: r.st})
	h := &linkHandler{prefix: "u_", tenant: r.tenant, known: map[string]bool{"u_abc": true}}
	require.NoError(t, r.c.RegisterStart(h))
	r.api.Chat(telegramtest.Chat{ID: 800, Type: "private"})
	r.platformUpdate(r.platform.Update(800, alice, "/start u_abc"))
	require.Len(t, u.entries, 1)
	assert.Equal(t, r.tenant, u.entries[0].TenantID)
	assert.Equal(t, "platform:telegram_bot", u.entries[0].ConnectionID)
	assert.Equal(t, int64(1), u.entries[0].Quantity)
	// An unknown link has no tenant to count for.
	r.platformUpdate(r.platform.Update(800, alice, "/start u_unknown"))
	assert.Len(t, u.entries, 1)
	// A tenant's own bot counts for its connection.
	r.api.Chat(telegramtest.Chat{ID: 700, Type: "private"})
	r.tenantUpdate(r.bot.Update(700, alice, "/start u_abc"))
	require.Len(t, u.entries, 2)
	assert.Equal(t, r.conn.ID.String(), u.entries[1].ConnectionID)
}

// A read answers with text only: a photo without a caption is an event for triggers, not the reply.
func TestReadFilterNeedsText(t *testing.T) {
	r := newRig(t)
	parsed, issues := r.read().Spec().Params.Parse(map[string]any{"chat_id": num(700), "from_user_id": num(501)})
	require.Empty(t, issues)
	assert.Equal(t, `event.text != "" && event.chatId == "700" && event.from.userId == "501"`, parsed["filter"])
	parsed, _ = r.read().Spec().Params.Parse(map[string]any{"chat_id": "event.chatId"})
	assert.Equal(t, `event.text != ""`, parsed["filter"], "the chat of an expression is the conversation's")
}

func (r *rig) read() interface{ Spec() graph.NodeSpec } {
	for _, n := range r.c.Nodes() {
		if n.Spec().Type == telegram.NodeRead {
			return n
		}
	}
	r.t.Fatal("no read node")
	return nil
}
