// Package telegramtest is an in-memory Telegram Bot API for tests of the connector and of graphs that use
// it: sendMessage, sendPhoto, sendDocument, sendMediaGroup, editMessageText, editMessageReplyMarkup,
// answerCallbackQuery, getFile, getMe, setWebhook, getWebhookInfo, deleteWebhook and getUpdates. It also
// plays Telegram's side of a webhook: a message, a photo or a press of a button posted to the URL the
// bot set, with the secret header.
//
// The contract test of the connector runs against it (CONTRACT_TARGET=fake, the default) and, by hand
// with a real bot token, against Telegram (CONTRACT_TARGET=live; never in CI).
package telegramtest

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Server is the fake Bot API.
type Server struct {
	mu      sync.Mutex
	srv     *httptest.Server
	bots    map[string]*Bot
	chats   map[int64]*Chat
	files   map[string]storedFile // file id → file
	fileSeq int
	// Client posts the webhook deliveries.
	Client *http.Client
}

// Chat is a chat the fake knows.
type Chat struct {
	ID       int64
	Type     string // private | group | supergroup | channel
	Title    string
	Username string
	// Forum: topics are the topic ids that exist.
	Forum  bool
	Topics map[int64]bool
}

type storedFile struct {
	owner string // the token of the bot the file belongs to: only that bot can get and download it
	path  string
	data  []byte
}

// New starts the fake; Close stops it.
func New() *Server {
	s := &Server{bots: map[string]*Bot{}, chats: map[int64]*Chat{}, files: map[string]storedFile{}, Client: &http.Client{Timeout: 30 * time.Second}}
	s.srv = httptest.NewServer(http.HandlerFunc(s.serve))
	return s
}

// URL is the Bot API origin: Config.APIURL.
func (s *Server) URL() string { return s.srv.URL }

func (s *Server) Close() { s.srv.Close() }

// Chat registers a chat the bots may write to.
func (s *Server) Chat(c Chat) *Chat {
	s.mu.Lock()
	defer s.mu.Unlock()
	cp := c
	if cp.Type == "" {
		cp.Type = "private"
	}
	cp.Topics = map[int64]bool{}
	for k, v := range c.Topics {
		cp.Topics[k] = v
	}
	s.chats[cp.ID] = &cp
	return &cp
}

func (s *Server) chatByRef(ref string) *Chat {
	if strings.HasPrefix(ref, "@") {
		for _, c := range s.chats {
			if "@"+c.Username == ref {
				return c
			}
		}
		return nil
	}
	id, err := strconv.ParseInt(ref, 10, 64)
	if err != nil {
		return nil
	}
	return s.chats[id]
}

// Bot registers a bot with its token ("<id>:<secret>") and username.
func (s *Server) Bot(token, username string) *Bot {
	s.mu.Lock()
	defer s.mu.Unlock()
	id, _, _ := strings.Cut(token, ":")
	n, _ := strconv.ParseInt(id, 10, 64)
	b := &Bot{s: s, Token: token, ID: n, Username: username, blocked: map[int64]bool{}, answered: map[string]string{}}
	s.bots[token] = b
	return b
}

// Failure is a refusal the next matching request gets.
type Failure struct {
	Method      string // "" matches any
	Code        int
	Description string
	RetryAfter  int
	Times       int // 0: once
	// Lost: the request is processed (the message is stored) but the answer is a 500: the client cannot know.
	Lost bool
}

// Bot is a bot of the fake: its webhook, its messages and the requests it got.
type Bot struct {
	s        *Server
	Token    string
	ID       int64
	Username string

	mu       sync.Mutex
	webhook  Webhook
	messages []*Message
	nextMsg  int64
	nextUpd  int64
	calls    []Call
	fails    []*Failure
	after    func(method string)
	blocked  map[int64]bool
	answered map[string]string // callback query id → the text it was answered with
	polling  bool
	pending  int
}

// Webhook is what setWebhook stored.
type Webhook struct {
	URL    string
	Secret string
}

// Call is a request the bot made.
type Call struct {
	Method string
	Params map[string]string
	Files  []string
}

// Message is a message in a chat: sent by the bot, or received by it.
type Message struct {
	ID       int64
	ChatID   int64
	ThreadID int64
	Text     string
	Caption  string
	Parse    string
	Silent   bool
	Markup   [][]Button // the inline keyboard now
	Files    []string
	FromBot  bool
	Edited   bool
}

// Button is a button of an inline keyboard.
type Button struct{ Text, URL, CallbackData string }

// Calls are the requests the bot made, in order.
func (b *Bot) Calls() []Call {
	b.mu.Lock()
	defer b.mu.Unlock()
	return append([]Call(nil), b.calls...)
}

// CallsOf are the requests of one method.
func (b *Bot) CallsOf(method string) []Call {
	var out []Call
	for _, c := range b.Calls() {
		if c.Method == method {
			out = append(out, c)
		}
	}
	return out
}

// Sent are the messages the bot sent, with their state now (an edit applies).
func (b *Bot) Sent() []Message {
	b.mu.Lock()
	defer b.mu.Unlock()
	var out []Message
	for _, m := range b.messages {
		if m.FromBot {
			out = append(out, *m)
		}
	}
	return out
}

// Message is a message of a chat by id.
func (b *Bot) Message(chat, id int64) (Message, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for _, m := range b.messages {
		if m.ChatID == chat && m.ID == id {
			return *m, true
		}
	}
	return Message{}, false
}

// Webhook is the webhook the bot has set.
func (b *Bot) Webhook() Webhook {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.webhook
}

// SetWebhook sets a webhook as another service would (not through the connector).
func (b *Bot) SetWebhook(url, secret string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.webhook = Webhook{URL: url, Secret: secret}
}

// SetPolling simulates another service long-polling getUpdates for the bot.
func (b *Bot) SetPolling(on bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.polling = on
}

// Block makes the bot's messages to the chat refused, as Telegram does after the user blocked the bot.
func (b *Bot) Block(chat int64) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.blocked[chat] = true
}

// After sets a function that runs once the bot has processed a request (a message is stored, say) and
// before the answer is given: a test replays an update there to overlap two deliveries.
func (b *Bot) After(fn func(method string)) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.after = fn
}

// Fail makes the next request of the failure's method fail.
func (b *Bot) Fail(f Failure) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if f.Times <= 0 {
		f.Times = 1
	}
	b.fails = append(b.fails, &f)
}

// Answered is the text a callback query was answered with, and whether it was answered at all.
func (b *Bot) Answered(queryID string) (string, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	t, ok := b.answered[queryID]
	return t, ok
}

// ---------------------------------------------------------------------------------------------
// the HTTP side

type apiResp struct {
	OK          bool            `json:"ok"`
	Result      json.RawMessage `json:"result,omitempty"`
	ErrorCode   int             `json:"error_code,omitempty"`
	Description string          `json:"description,omitempty"`
	Parameters  map[string]int  `json:"parameters,omitempty"`
}

type apiError struct {
	code       int
	desc       string
	retryAfter int
}

func (e *apiError) Error() string { return e.desc }

func fail(code int, desc string) *apiError { return &apiError{code: code, desc: desc} }

func (s *Server) serve(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/")
	if rest, ok := strings.CutPrefix(strings.TrimPrefix(r.URL.EscapedPath(), "/"), "file/bot"); ok {
		s.serveFile(w, rest)
		return
	}
	rest, ok := strings.CutPrefix(path, "bot")
	if !ok {
		http.NotFound(w, r)
		return
	}
	token, method, ok := strings.Cut(rest, "/")
	if !ok {
		http.NotFound(w, r)
		return
	}
	s.mu.Lock()
	b := s.bots[token]
	s.mu.Unlock()
	if b == nil {
		reply(w, nil, fail(401, "Unauthorized"))
		return
	}
	params, files, err := readParams(r)
	if err != nil {
		reply(w, nil, fail(400, "Bad Request: "+err.Error()))
		return
	}
	b.mu.Lock()
	b.calls = append(b.calls, Call{Method: method, Params: params, Files: fileNames(files)})
	var lost bool
	for i, f := range b.fails {
		if f.Method == "" || f.Method == method {
			lost = f.Lost
			ff := *f
			f.Times--
			if f.Times <= 0 {
				b.fails = append(b.fails[:i], b.fails[i+1:]...)
			}
			if !ff.Lost {
				b.mu.Unlock()
				reply(w, nil, &apiError{code: ff.Code, desc: ff.Description, retryAfter: ff.RetryAfter})
				return
			}
			break
		}
	}
	b.mu.Unlock()
	res, aerr := b.handle(method, params, files)
	b.mu.Lock()
	after := b.after
	b.mu.Unlock()
	if after != nil {
		after(method)
	}
	if lost {
		http.Error(w, "lost", http.StatusInternalServerError)
		return
	}
	reply(w, res, aerr)
}

func reply(w http.ResponseWriter, result any, e *apiError) {
	w.Header().Set("Content-Type", "application/json")
	if e != nil {
		out := apiResp{ErrorCode: e.code, Description: e.desc}
		if e.retryAfter > 0 {
			out.Parameters = map[string]int{"retry_after": e.retryAfter}
		}
		w.WriteHeader(e.code)
		writeJSON(w, out)
		return
	}
	raw, err := json.Marshal(result)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, apiResp{OK: true, Result: raw})
}

// writeJSON writes a Bot API answer; a failure to write is the client's having gone.
func writeJSON(w http.ResponseWriter, v apiResp) {
	if err := json.NewEncoder(w).Encode(v); err != nil {
		return
	}
}

type upload struct {
	name string
	data []byte
}

func fileNames(files map[string]upload) []string {
	out := make([]string, 0, len(files))
	for _, f := range files {
		out = append(out, f.name)
	}
	return out
}

func readParams(r *http.Request) (map[string]string, map[string]upload, error) {
	params := map[string]string{}
	files := map[string]upload{}
	ct := r.Header.Get("Content-Type")
	switch {
	case strings.HasPrefix(ct, "multipart/form-data"):
		if err := r.ParseMultipartForm(64 << 20); err != nil {
			return nil, nil, err
		}
		for k, v := range r.MultipartForm.Value {
			params[k] = v[0]
		}
		for k, fh := range r.MultipartForm.File {
			f, err := fh[0].Open()
			if err != nil {
				return nil, nil, err
			}
			data, err := io.ReadAll(f)
			_ = f.Close()
			if err != nil {
				return nil, nil, err
			}
			files[k] = upload{name: fh[0].Filename, data: data}
		}
	default:
		body, err := io.ReadAll(r.Body)
		if err != nil {
			return nil, nil, err
		}
		if len(bytes.TrimSpace(body)) > 0 {
			var raw map[string]any
			if err := json.Unmarshal(body, &raw); err != nil {
				return nil, nil, err
			}
			for k, v := range raw {
				if s, ok := v.(string); ok {
					params[k] = s
					continue
				}
				b, err := json.Marshal(v)
				if err != nil {
					return nil, nil, err
				}
				params[k] = string(b)
			}
		}
	}
	return params, files, nil
}

func (s *Server) serveFile(w http.ResponseWriter, rest string) {
	token, path, ok := strings.Cut(rest, "/")
	if !ok {
		http.NotFound(w, nil)
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, f := range s.files {
		if f.path == path && f.owner == token {
			_, _ = w.Write(f.data)
			return
		}
	}
	http.Error(w, "not found", http.StatusNotFound)
}

// ---------------------------------------------------------------------------------------------
// methods

type userJSON struct {
	ID        int64  `json:"id"`
	IsBot     bool   `json:"is_bot"`
	FirstName string `json:"first_name"`
	Username  string `json:"username,omitempty"`
}

func (b *Bot) user() userJSON {
	return userJSON{ID: b.ID, IsBot: true, FirstName: b.Username, Username: b.Username}
}

func (b *Bot) handle(method string, p map[string]string, files map[string]upload) (any, *apiError) {
	switch method {
	case "getMe":
		return b.user(), nil
	case "setWebhook":
		if p["url"] == "" {
			return nil, fail(400, "Bad Request: bad webhook: URL is required")
		}
		b.mu.Lock()
		b.webhook = Webhook{URL: p["url"], Secret: p["secret_token"]}
		b.mu.Unlock()
		return true, nil
	case "deleteWebhook":
		b.mu.Lock()
		b.webhook = Webhook{}
		b.mu.Unlock()
		return true, nil
	case "getWebhookInfo":
		b.mu.Lock()
		defer b.mu.Unlock()
		return map[string]any{"url": b.webhook.URL, "has_custom_certificate": false, "pending_update_count": b.pending}, nil
	case "getUpdates":
		b.mu.Lock()
		defer b.mu.Unlock()
		if b.webhook.URL != "" {
			return nil, fail(409, "Conflict: can't use getUpdates method while webhook is active")
		}
		if b.polling {
			return nil, fail(409, "Conflict: terminated by other getUpdates request; make sure that only one bot instance is running")
		}
		return []any{}, nil
	case "sendMessage", "sendPhoto", "sendDocument":
		return b.send(method, p, files)
	case "sendMediaGroup":
		return b.sendGroup(p, files)
	case "editMessageText":
		return b.edit(p, true)
	case "editMessageReplyMarkup":
		return b.edit(p, false)
	case "answerCallbackQuery":
		b.mu.Lock()
		b.answered[p["callback_query_id"]] = p["text"]
		b.mu.Unlock()
		return true, nil
	case "getFile":
		b.s.mu.Lock()
		defer b.s.mu.Unlock()
		f, ok := b.s.files[p["file_id"]]
		if !ok || f.owner != b.Token {
			return nil, fail(400, "Bad Request: invalid file_id")
		}
		if len(f.data) > 20<<20 {
			return nil, fail(400, "Bad Request: file is too big")
		}
		return map[string]any{"file_id": p["file_id"], "file_unique_id": p["file_id"], "file_size": len(f.data), "file_path": f.path}, nil
	}
	return nil, fail(404, "Not Found")
}

func (b *Bot) target(p map[string]string) (*Chat, *apiError) {
	b.s.mu.Lock()
	c := b.s.chatByRef(p["chat_id"])
	b.s.mu.Unlock()
	if c == nil {
		return nil, fail(400, "Bad Request: chat not found")
	}
	b.mu.Lock()
	blocked := b.blocked[c.ID]
	b.mu.Unlock()
	if blocked {
		return nil, fail(403, "Forbidden: bot was blocked by the user")
	}
	if t := p["message_thread_id"]; t != "" {
		id, _ := strconv.ParseInt(t, 10, 64)
		if c.Forum && !c.Topics[id] {
			return nil, fail(400, "Bad Request: message thread not found")
		}
	}
	return c, nil
}

func parseMarkup(raw string) ([][]Button, *apiError) {
	if raw == "" {
		return nil, nil
	}
	var doc struct {
		Rows [][]struct {
			Text string `json:"text"`
			URL  string `json:"url"`
			Data string `json:"callback_data"`
		} `json:"inline_keyboard"`
	}
	if err := json.Unmarshal([]byte(raw), &doc); err != nil {
		return nil, fail(400, "Bad Request: can't parse reply keyboard markup JSON object")
	}
	out := make([][]Button, 0, len(doc.Rows))
	for _, row := range doc.Rows {
		var r []Button
		for _, b := range row {
			if len(b.Data) > 64 {
				return nil, fail(400, "Bad Request: BUTTON_DATA_INVALID")
			}
			r = append(r, Button{Text: b.Text, URL: b.URL, CallbackData: b.Data})
		}
		out = append(out, r)
	}
	return out, nil
}

func (b *Bot) record(c *Chat, p map[string]string, m *Message) *Message {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.nextMsg++
	m.ID, m.ChatID, m.FromBot = b.nextMsg, c.ID, true
	m.ThreadID, _ = strconv.ParseInt(p["message_thread_id"], 10, 64)
	m.Silent = p["disable_notification"] == "true"
	m.Parse = p["parse_mode"]
	b.messages = append(b.messages, m)
	snap := *m // the response is built from a copy: an edit changes the stored message under the lock
	return &snap
}

func (b *Bot) messageJSON(m *Message, c *Chat) map[string]any {
	out := map[string]any{"message_id": m.ID, "date": time.Now().Unix(), "chat": map[string]any{"id": c.ID, "type": c.Type, "title": c.Title}, "from": b.user()}
	if m.Text != "" {
		out["text"] = m.Text
	}
	if m.ThreadID != 0 {
		out["message_thread_id"] = m.ThreadID
	}
	return out
}

func (b *Bot) send(method string, p map[string]string, files map[string]upload) (any, *apiError) {
	c, e := b.target(p)
	if e != nil {
		return nil, e
	}
	markup, e := parseMarkup(p["reply_markup"])
	if e != nil {
		return nil, e
	}
	m := &Message{Markup: markup}
	switch method {
	case "sendMessage":
		if strings.TrimSpace(p["text"]) == "" {
			return nil, fail(400, "Bad Request: message text is empty")
		}
		if len([]rune(p["text"])) > 4096 {
			return nil, fail(400, "Bad Request: message is too long")
		}
		m.Text = p["text"]
	default:
		field := "document"
		if method == "sendPhoto" {
			field = "photo"
		}
		f, ok := files[field]
		if !ok {
			return nil, fail(400, "Bad Request: there is no "+field+" in the request")
		}
		m.Caption, m.Files = p["caption"], []string{f.name}
	}
	return b.messageJSON(b.record(c, p, m), c), nil
}

func (b *Bot) sendGroup(p map[string]string, files map[string]upload) (any, *apiError) {
	c, e := b.target(p)
	if e != nil {
		return nil, e
	}
	var media []struct {
		Media   string `json:"media"`
		Caption string `json:"caption"`
	}
	if err := json.Unmarshal([]byte(p["media"]), &media); err != nil || len(media) < 2 || len(media) > 10 {
		return nil, fail(400, "Bad Request: media group takes 2-10 items")
	}
	// Every attachment is resolved before anything is recorded: a refused group leaves nothing sent.
	items := make([]upload, len(media))
	for i, it := range media {
		f, ok := files[strings.TrimPrefix(it.Media, "attach://")]
		if !ok {
			return nil, fail(400, "Bad Request: file not attached")
		}
		items[i] = f
	}
	out := make([]any, 0, len(media))
	for i, it := range media {
		m := b.record(c, p, &Message{Caption: it.Caption, Files: []string{items[i].name}})
		out = append(out, b.messageJSON(m, c))
	}
	return out, nil
}

func (b *Bot) edit(p map[string]string, text bool) (any, *apiError) {
	b.s.mu.Lock()
	c := b.s.chatByRef(p["chat_id"])
	b.s.mu.Unlock()
	if c == nil {
		return nil, fail(400, "Bad Request: chat not found")
	}
	id, _ := strconv.ParseInt(p["message_id"], 10, 64)
	b.mu.Lock()
	defer b.mu.Unlock()
	var m *Message
	for _, x := range b.messages {
		if x.ChatID == c.ID && x.ID == id && x.FromBot {
			m = x
		}
	}
	if m == nil {
		return nil, fail(400, "Bad Request: message to edit not found")
	}
	markup, e := parseMarkup(p["reply_markup"])
	if e != nil {
		return nil, e
	}
	switch {
	case text && p["text"] == m.Text && len(markup) == 0 && len(m.Markup) == 0:
		return nil, fail(400, "Bad Request: message is not modified: specified new message content and reply markup are exactly the same as a current content and reply markup of the message")
	case text:
		m.Text = p["text"]
	case len(markup) == 0 && len(m.Markup) == 0:
		return nil, fail(400, "Bad Request: message is not modified: specified new message content and reply markup are exactly the same as a current content and reply markup of the message")
	}
	m.Markup, m.Edited = markup, true
	return b.messageJSON(m, c), nil
}

// ---------------------------------------------------------------------------------------------
// Telegram's side of the webhook

// User is a person writing to a bot.
type User struct {
	ID        int64
	Username  string
	FirstName string
	LastName  string
}

// Delivery is the application's answer to a webhook post.
type Delivery struct {
	UpdateID int64
	Status   int
}

// Redeliver posts an update again, with the same update_id (Telegram does it after a 5xx).
func (b *Bot) Redeliver(update map[string]any) (Delivery, error) {
	b.mu.Lock()
	hook := b.webhook
	b.mu.Unlock()
	id, _ := update["update_id"].(int64)
	return b.post(hook, update, id)
}

func (b *Bot) post(hook Webhook, update map[string]any, id int64) (Delivery, error) {
	if hook.URL == "" {
		return Delivery{UpdateID: id}, fmt.Errorf("telegramtest: the bot has no webhook")
	}
	body, err := json.Marshal(update)
	if err != nil {
		return Delivery{}, err
	}
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, hook.URL, bytes.NewReader(body))
	if err != nil {
		return Delivery{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	if hook.Secret != "" {
		req.Header.Set("X-Telegram-Bot-Api-Secret-Token", hook.Secret)
	}
	resp, err := b.s.Client.Do(req)
	if err != nil {
		return Delivery{UpdateID: id}, err
	}
	defer func() { _ = resp.Body.Close() }()
	_, _ = io.Copy(io.Discard, resp.Body)
	return Delivery{UpdateID: id, Status: resp.StatusCode}, nil
}

// Post posts a raw update to the bot's webhook, with or without the right secret.
func (b *Bot) Post(update map[string]any, secret string) (Delivery, error) {
	b.mu.Lock()
	hook := Webhook{URL: b.webhook.URL, Secret: secret}
	b.mu.Unlock()
	return b.post(hook, update, 0)
}

func (b *Bot) incoming(c *Chat, from User, extra map[string]any) map[string]any {
	b.mu.Lock()
	b.nextMsg++
	id := b.nextMsg
	b.mu.Unlock()
	msg := map[string]any{
		"message_id": id, "date": time.Now().Unix(),
		"chat": map[string]any{"id": c.ID, "type": c.Type, "title": c.Title, "username": c.Username, "is_forum": c.Forum},
		"from": map[string]any{"id": from.ID, "is_bot": false, "first_name": from.FirstName, "username": from.Username},
	}
	if c.Type == "private" {
		chat := msg["chat"].(map[string]any)
		chat["first_name"], chat["last_name"] = from.FirstName, from.LastName
		delete(chat, "title")
	}
	for k, v := range extra {
		msg[k] = v
	}
	b.mu.Lock()
	b.messages = append(b.messages, &Message{ID: id, ChatID: c.ID, Text: fmt.Sprint(extra["text"])})
	b.mu.Unlock()
	return msg
}

// Update builds the update of a text message to the bot, without posting it.
func (b *Bot) Update(chat int64, from User, text string) map[string]any {
	b.s.mu.Lock()
	c := b.s.chats[chat]
	b.s.mu.Unlock()
	if c == nil {
		c = &Chat{ID: chat, Type: "private"}
	}
	extra := map[string]any{"text": text}
	if strings.HasPrefix(text, "/") {
		n := len(text)
		if i := strings.IndexByte(text, ' '); i >= 0 {
			n = i
		}
		extra["entities"] = []map[string]any{{"type": "bot_command", "offset": 0, "length": n}}
	}
	b.mu.Lock()
	b.nextUpd++
	id := b.nextUpd
	b.mu.Unlock()
	return map[string]any{"update_id": id, "message": b.incoming(c, from, extra)}
}

// SendText posts a text message of a user to the bot's webhook.
func (b *Bot) SendText(chat int64, from User, text string) (Delivery, error) {
	u := b.Update(chat, from, text)
	return b.Redeliver(u)
}

// SendInTopic posts a message in a topic of a forum.
func (b *Bot) SendInTopic(chat int64, topic int64, from User, text string) (Delivery, error) {
	u := b.Update(chat, from, text)
	msg := u["message"].(map[string]any)
	msg["message_thread_id"], msg["is_topic_message"] = topic, true
	return b.Redeliver(u)
}

// PhotoUpdate builds the update of a photo (with an optional caption) of a user, without posting it. The
// file is held for getFile.
func (b *Bot) PhotoUpdate(chat int64, from User, caption string, data []byte) map[string]any {
	return b.fileUpdate(chat, from, caption, "photo", "", "", data)
}

// DocumentUpdate builds the update of a document of a user, without posting it.
func (b *Bot) DocumentUpdate(chat int64, from User, caption, name, mime string, data []byte) map[string]any {
	return b.fileUpdate(chat, from, caption, "document", name, mime, data)
}

// SendPhoto posts a photo (with an optional caption) of a user.
func (b *Bot) SendPhoto(chat int64, from User, caption string, data []byte) (Delivery, error) {
	return b.Redeliver(b.PhotoUpdate(chat, from, caption, data))
}

// SendDocument posts a document of a user.
func (b *Bot) SendDocument(chat int64, from User, caption, name, mime string, data []byte) (Delivery, error) {
	return b.Redeliver(b.DocumentUpdate(chat, from, caption, name, mime, data))
}

func (b *Bot) fileUpdate(chat int64, from User, caption, kind, name, mime string, data []byte) map[string]any {
	b.s.mu.Lock()
	b.s.fileSeq++
	seq := b.s.fileSeq
	id := fmt.Sprintf("file-%d", seq)
	// The sequence is part of the download path, so two files of one name are two files; the name itself is
	// kept apart from any formatting (it may hold a "%").
	path := fmt.Sprintf("photos/file_%d.jpg", seq)
	if kind != "photo" {
		path = fmt.Sprintf("documents/%d/", seq) + url.PathEscape(name) // a file path is URL-safe, as Telegram's are
	}
	b.s.files[id] = storedFile{owner: b.Token, path: path, data: data}
	b.s.mu.Unlock()
	u := b.Update(chat, from, "")
	msg := u["message"].(map[string]any)
	delete(msg, "text")
	if caption != "" {
		msg["caption"] = caption
	}
	if kind == "photo" {
		msg["photo"] = []map[string]any{{"file_id": id + "-s", "file_unique_id": id, "width": 90, "height": 90, "file_size": 10},
			{"file_id": id, "file_unique_id": id, "width": 800, "height": 800, "file_size": len(data)}}
		b.s.mu.Lock()
		b.s.files[id+"-s"] = storedFile{owner: b.Token, path: path + ".thumb", data: []byte("x")}
		b.s.mu.Unlock()
	} else {
		msg["document"] = map[string]any{"file_id": id, "file_unique_id": id, "file_name": name, "mime_type": mime, "file_size": len(data)}
	}
	return u
}

// Press posts the press of the i-th button (in reading order) of a message the bot sent.
func (b *Bot) Press(chat, message int64, from User, i int) (Delivery, string, error) {
	m, ok := b.Message(chat, message)
	if !ok {
		return Delivery{}, "", fmt.Errorf("telegramtest: no message %d in chat %d", message, chat)
	}
	var flat []Button
	for _, row := range m.Markup {
		flat = append(flat, row...)
	}
	if i < 0 || i >= len(flat) {
		// A press of a button of a message whose keyboard is gone: Telegram sends the data the client had.
		return Delivery{}, "", fmt.Errorf("telegramtest: the message has %d buttons", len(flat))
	}
	return b.PressData(chat, message, from, flat[i].CallbackData, m)
}

// PressUpdate builds the update of a press of the button with the given data on a message as it was
// sent (its keyboard as it showed), without posting it.
func (b *Bot) PressUpdate(as Message, from User, data string) map[string]any {
	b.s.mu.Lock()
	c := b.s.chats[as.ChatID]
	b.s.mu.Unlock()
	if c == nil {
		c = &Chat{ID: as.ChatID, Type: "private"}
	}
	rows := make([][]map[string]string, 0, len(as.Markup))
	for _, row := range as.Markup {
		var r []map[string]string
		for _, bt := range row {
			e := map[string]string{"text": bt.Text}
			if bt.CallbackData != "" {
				e["callback_data"] = bt.CallbackData
			}
			r = append(r, e)
		}
		rows = append(rows, r)
	}
	b.mu.Lock()
	b.nextUpd++
	uid := b.nextUpd
	b.mu.Unlock()
	return map[string]any{"update_id": uid, "callback_query": map[string]any{
		"id": fmt.Sprintf("q-%d-%d", b.ID, uid), "chat_instance": "ci", "data": data,
		"from": map[string]any{"id": from.ID, "is_bot": false, "first_name": from.FirstName, "username": from.Username},
		"message": map[string]any{
			"message_id": as.ID, "date": time.Now().Unix(), "text": as.Text,
			"chat":         map[string]any{"id": c.ID, "type": c.Type, "title": c.Title},
			"from":         b.user(),
			"reply_markup": map[string]any{"inline_keyboard": rows},
		},
	}}
}

// PressData posts a press with the given callback data (the message is shown with its keyboard as it was).
func (b *Bot) PressData(chat, message int64, from User, data string, as Message) (Delivery, string, error) {
	as.ChatID, as.ID = chat, message
	u := b.PressUpdate(as, from, data)
	d, err := b.Redeliver(u)
	return d, u["callback_query"].(map[string]any)["id"].(string), err
}
