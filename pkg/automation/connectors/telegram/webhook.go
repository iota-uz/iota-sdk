package telegram

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/google/uuid"

	"github.com/iota-uz/iota-sdk/pkg/automation/catalog"
	"github.com/iota-uz/iota-sdk/pkg/automation/connectors"
	"github.com/iota-uz/iota-sdk/pkg/automation/engine"
	"github.com/iota-uz/iota-sdk/pkg/automation/graph"
)

// SecretHeader is the header Telegram sends the webhook's secret_token in.
const SecretHeader = "X-Telegram-Bot-Api-Secret-Token"

// AlreadyAnswered is what a person who presses a button of a closed question is told.
const AlreadyAnswered = "Ответ уже принят или время истекло"

var (
	_ engine.WebhookReceiver       = (*Connector)(nil)
	_ engine.ClosedAnswerer        = (*Connector)(nil)
	_ engine.PublishedAnswerer     = (*Connector)(nil)
	_ connectors.Connector         = (*Connector)(nil)
	_ connectors.Provisioner       = (*Connector)(nil)
	_ connectors.Advisor           = (*Connector)(nil)
	_ connectors.PlatformStarter   = (*Connector)(nil)
	_ connectors.ExclusiveIdentity = (*Connector)(nil)
)

// connKey is a connection as the webhook secret and the events name it: its UUID, or "platform:<kind>".
func connKey(c connectors.Connection) string {
	if c.ID != uuid.Nil {
		return c.ID.String()
	}
	return catalog.PlatformConnectionID(Kind)
}

func isPlatform(c connectors.Connection) bool { return c.ID == uuid.Nil }

// Verify compares the secret Telegram sends with the one this connection's webhook was set with, in constant
// time. A tenant connection's secret is the one Provision generated, kept in its inbound state; the platform
// bot has no connection row, so its secret is derived from its token.
func (c *Connector) Verify(r *http.Request, _ []byte, conn connectors.Connection, creds connectors.Credentials) error {
	token := creds["token"]
	got := r.Header.Get(SecretHeader)
	if token == "" || got == "" {
		return errors.New("telegram: no webhook secret")
	}
	var want string
	if isPlatform(conn) {
		want = WebhookSecret(token, connKey(conn))
	} else {
		stored, account, err := c.storedSecret(r.Context(), conn)
		if err != nil {
			return err
		}
		// The secret belongs to the bot it was made for: credentials read for one bot and a secret read for
		// another (a token replaced between the two reads) are never taken together.
		if account != c.ExternalID(creds) {
			return errors.New("telegram: the webhook secret is of another bot")
		}
		want = stored
	}
	if want == "" || !secretEqual(got, want) {
		return errors.New("telegram: the webhook secret does not match")
	}
	return nil
}

// storedSecret reads the connection's webhook secret, and the bot account it is bound to, from its inbound
// state.
func (c *Connector) storedSecret(ctx context.Context, conn connectors.Connection) (string, string, error) {
	reader, ok := c.cfg.Connections.(connectors.InboundReader)
	if !ok {
		return "", "", errors.New("telegram: the connector cannot read the webhook secret")
	}
	state, err := reader.InboundState(ctx, conn.TenantID, conn.ID)
	if err != nil {
		return "", "", err
	}
	return heldSecret(state)
}

// Handle turns one update into events. Only a message and a button press are read, others are answered
// and dropped. The tenant's own bot publishes its messages; the platform bot publishes only a press of a
// button (its tenant is the question's), and hands a `/start <payload>` to the registered StartHandler.
func (c *Connector) Handle(ctx context.Context, w http.ResponseWriter, body []byte, conn connectors.Connection, creds connectors.Credentials) ([]engine.InboundEvent, error) {
	var u gotgbot.Update
	if err := json.Unmarshal(body, &u); err != nil {
		return nil, errors.New("telegram: the update is not JSON")
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
	b := c.bot(creds["token"])
	switch {
	case u.CallbackQuery != nil:
		return c.callback(ctx, b, u, conn)
	case u.Message != nil:
		return c.message(ctx, b, u, conn, creds)
	}
	return nil, nil
}

// updateID is the number of an update, which is the bot's own: Telegram numbers the updates of each bot.
func updateID(u gotgbot.Update) string { return strconv.FormatInt(u.UpdateId, 10) }

// dedupKey is the dedup key of an update of the bot with the token: the bot's account and the update's number,
// so that the bot that replaces another behind the same connection (its numbers start again) is not taken for a
// replay. A new token of the same bot keeps the key.
func dedupKey(token string, u gotgbot.Update) string {
	return strconv.FormatInt(botID(token), 10) + ":" + updateID(u)
}

// ---------------------------------------------------------------------------------------------
// messages

// command reads "/name@bot args" at the start of a message: its name (lower case, without the slash and the
// bot's mention), the bot it is addressed to ("" when it names none) and the rest.
func command(m *gotgbot.Message) (string, string, string, bool) {
	if m.Text == "" {
		return "", "", "", false
	}
	for _, e := range m.Entities {
		if e.Type != "bot_command" || e.Offset != 0 {
			continue
		}
		runes := []rune(m.Text) // entity offsets are UTF-16 units; a command is ASCII
		n := int(e.Length)
		if n < 2 || n > len(runes) {
			return "", "", "", false
		}
		cmd, to := string(runes[1:n]), ""
		if i := strings.IndexByte(cmd, '@'); i >= 0 {
			cmd, to = cmd[:i], cmd[i+1:]
		}
		return strings.ToLower(cmd), strings.ToLower(to), strings.TrimSpace(string(runes[n:])), cmd != ""
	}
	return "", "", "", false
}

// usernameTTL is how long the username of a bot is kept: after it the entry is read again, and a timer drops it
// from the map whether or not anything is cached later.
const usernameTTL = time.Hour

type cachedName struct {
	name string
	at   time.Time
}

// rememberName caches the username of a bot and drops the entries that have outlived usernameTTL, so a token
// that was replaced (its fingerprint is never asked for again) does not stay for the life of the process.
func (c *Connector) rememberName(fp [sha256.Size]byte, name string, now time.Time) {
	c.usernames.Range(func(k, v any) bool {
		if now.Sub(v.(cachedName).at) >= usernameTTL {
			c.usernames.Delete(k)
		}
		return true
	})
	entry := cachedName{name: name, at: now}
	c.usernames.Store(fp, entry)
	// The entry also leaves on its own when its time is up, so a replaced token is not kept for want of a later
	// insertion. A newer entry of the same bot is not touched (CompareAndDelete matches this one only).
	if left := usernameTTL - time.Since(now); left > 0 {
		time.AfterFunc(left, func() { c.usernames.CompareAndDelete(fp, entry) })
	} else {
		c.usernames.CompareAndDelete(fp, entry)
	}
}

// addressedToUs reports whether a command's addressee is this bot (or names none): a command for another bot
// of a group is that bot's, never this connector's.
func (c *Connector) addressedToUs(ctx context.Context, b *bot, to string) (bool, error) {
	if to == "" {
		return true, nil
	}
	fp := sha256.Sum256([]byte(b.api.Token))
	if v, ok := c.usernames.Load(fp); ok {
		if e := v.(cachedName); time.Since(e.at) < usernameTTL {
			return e.name == to, nil
		}
	}
	me, err := b.getMe(ctx)
	if err != nil {
		return false, err // who the bot is cannot be told now: the update is redelivered, not dropped
	}
	name := strings.ToLower(me.Username)
	c.rememberName(fp, name, time.Now())
	return name == to, nil
}

func hasMedia(m *gotgbot.Message) bool {
	return len(m.Photo) > 0 || m.Document != nil || m.Video != nil || m.Audio != nil || m.Voice != nil ||
		m.VideoNote != nil || m.Animation != nil || m.Sticker != nil
}

func (c *Connector) message(ctx context.Context, b *bot, u gotgbot.Update, conn connectors.Connection, creds connectors.Credentials) ([]engine.InboundEvent, error) {
	m := u.Message
	cmd, to, args, isCmd := command(m)
	if isCmd {
		ours, err := c.addressedToUs(ctx, b, to)
		if err != nil {
			return nil, fmt.Errorf("%w: %v", engine.ErrWebhookRetry, err)
		}
		if !ours {
			return nil, nil // for another bot
		}
	}
	if isCmd && cmd == "start" && args != "" {
		if h, ok := c.startHandlerFor(args); ok {
			return nil, c.start(ctx, b, h, m, args, conn, updateID(u))
		}
		if isPlatform(conn) {
			return nil, nil // an unknown prefix on the platform bot: dropped, unanswered
		}
	}
	if isPlatform(conn) {
		return nil, nil // the platform bot's messages belong to no tenant: a press and /start only
	}
	text := m.Text
	if text == "" {
		text = m.Caption
	}
	media := hasMedia(m)
	if strings.TrimSpace(text) == "" && !media {
		return nil, nil // a service message (a member joined, a pin): not a message to a graph
	}
	key := connKey(conn)
	chat := strconv.FormatInt(m.Chat.Id, 10)
	c.rememberChat(ctx, conn.TenantID, conn.ID, ChatInfo{ID: m.Chat.Id, Type: m.Chat.Type, Title: chatTitle(m.Chat), Username: m.Chat.Username, LastSeen: time.Now().UTC()})

	payload := map[string]any{
		"connectionId": key,
		"chatId":       chat,
		"chatType":     chatType(m.Chat.Type),
		"messageId":    strconv.FormatInt(m.MessageId, 10),
		"from":         fromOf(m.From),
		"isCommand":    isCmd,
		"route":        routeOf(isCmd, cmd),
		"hasMedia":     media,
		"date":         time.Unix(m.Date, 0).UTC().Format(time.RFC3339),
	}
	if t := chatTitle(m.Chat); t != "" {
		payload["chatTitle"] = t
	}
	if m.IsTopicMessage && m.MessageThreadId != 0 {
		payload["threadId"] = strconv.FormatInt(m.MessageThreadId, 10)
	}
	if text != "" {
		payload["text"] = text
	}
	if isCmd {
		payload["command"] = cmd
		if args != "" {
			payload["args"] = args
		}
	}
	if refs := refsOf(m); len(refs) > 0 {
		payload["files"] = c.store(ctx, b, conn.TenantID, dedupKey(b.api.Token, u), refs)
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	return []engine.InboundEvent{{
		Event: EventMessage, DedupKey: dedupKey(b.api.Token, u), Payload: raw, OccurredAt: time.Unix(m.Date, 0).UTC(),
		Subject: engine.SubjectRef{Entity: graph.SubjectConversation, Key: key + ":" + chat},
		// A message from the bot itself starts no graph that did not ask for it (loop protection).
		BySelf: m.From != nil && m.From.Id == botID(creds["token"]),
	}}, nil
}

// routeOf is the route of a message: "text", or "command:<name>".
func routeOf(isCmd bool, cmd string) string {
	if isCmd {
		return "command:" + cmd
	}
	return "text"
}

func chatType(t string) string {
	switch t {
	case "private", "group", "supergroup", "channel":
		return t
	}
	return "group"
}

func chatTitle(c gotgbot.Chat) string {
	if c.Title != "" {
		return c.Title
	}
	return strings.TrimSpace(c.FirstName + " " + c.LastName)
}

// fromOf is the sender as the events carry it; a message without one (a channel's) has the zero user.
func fromOf(u *gotgbot.User) map[string]any {
	if u == nil {
		return map[string]any{"userId": "0", "firstName": ""}
	}
	out := map[string]any{"userId": strconv.FormatInt(u.Id, 10), "firstName": u.FirstName}
	if u.Username != "" {
		out["username"] = u.Username
	}
	if u.LastName != "" {
		out["lastName"] = u.LastName
	}
	return out
}

// startBudget bounds each of the two phases of a /start that is claimed durably (the handler's run; the reply's
// delivery): shorter than the store's claim timeout, so a live worker does not outlast its claim.
const startBudget = 60 * time.Second

// start hands `/start <payload>` to its handler and sends its reply. An unknown or used payload (or one
// of another tenant, on a tenant's bot) reaches the handler with the nil tenant, which only replies. A
// failure of the lookup or of the handler is redelivered by Telegram.
//
// With a store that keeps the updates a connector handles itself (connectors.InboundActions) the update goes
// through two atomic claims: one before the handler runs (two deliveries never both run it) and one before
// the reply is sent (two deliveries never both send it, the first request included). The handler's result
// (the reply) is kept between them, so a reply that could not be sent is sent again by the redelivery without
// asking the handler, which has consumed its one-use link. The update is kept per bot, not per route: Telegram's
// update ids are those of one bot, and the route stays when the bot behind it changes.
func (c *Connector) start(ctx context.Context, b *bot, h StartHandler, m *gotgbot.Message, payload string, conn connectors.Connection, updateKey string) error {
	retry := func(err error) error { return fmt.Errorf("%w: %v", engine.ErrWebhookRetry, err) }
	key := connKey(conn)
	acts, durable := c.cfg.Connections.(connectors.InboundActions)
	scope := actionScope{acts: acts, conn: key + "@" + c.ExternalID(connectors.Credentials{"token": b.api.Token}), event: startEvent, key: updateKey}
	var claim connectors.InboundAction
	if durable {
		a, claimed, err := acts.ClaimInbound(ctx, scope.conn, scope.event, scope.key)
		if err != nil {
			return retry(err)
		}
		if !claimed {
			switch a.State {
			case connectors.InboundDone:
				return nil
			case connectors.InboundHandled:
				return c.deliverStartReply(ctx, b, m, conn, scope)
			case connectors.InboundClaimed, connectors.InboundDelivering:
			}
			return retry(errors.New("the update is being handled by another delivery"))
		}
		claim = a
	}
	hctx, cancel := context.WithTimeout(ctx, startBudget)
	defer cancel()
	tenant, ok, err := h.TenantOf(hctx, payload)
	if err != nil {
		scope.release(c, claim.Owner)
		return retry(err)
	}
	if !ok || (!isPlatform(conn) && tenant != conn.TenantID) {
		tenant = uuid.Nil
	}
	// The tenant the link belongs to (the platform bot: the payload's); a tenant's own bot: its tenant.
	account := conn.TenantID
	if isPlatform(conn) {
		account = tenant
	}
	up := StartUpdate{TenantID: tenant, ConnectionID: key, ChatID: m.Chat.Id, ChatType: chatType(m.Chat.Type), Payload: payload,
		UpdateKey: strconv.FormatInt(botID(b.api.Token), 10) + ":" + updateKey}
	if m.From != nil {
		up.UserID, up.Username, up.FirstName = m.From.Id, m.From.Username, m.From.FirstName
	}
	reply, err := h.Handle(hctx, up)
	if err != nil {
		scope.release(c, claim.Owner)
		return retry(err)
	}
	if !durable {
		c.sendStartReply(ctx, b, m, conn, account, reply)
		return nil
	}
	// The handler completed (it may have consumed the link): its result is kept before anything else. A claim
	// that was taken over meanwhile (the handler outlasted it) leaves the update to the new claimant.
	// The write is tried again a few times: the link is spent, and a result that is lost cannot be asked for
	// again. A claim that was taken over is final.
	var kept error
	for attempt := 0; attempt < 3; attempt++ {
		if kept = acts.InboundHandled(ctx, scope.conn, scope.event, scope.key, claim.Owner, account, reply); kept == nil || errors.Is(kept, connectors.ErrInboundLost) {
			break
		}
		select {
		case <-ctx.Done():
		case <-time.After(time.Duration(attempt+1) * 100 * time.Millisecond):
		}
	}
	if kept != nil {
		return retry(kept)
	}
	return c.deliverStartReply(ctx, b, m, conn, scope)
}

// actionScope names the durable record of an update.
type actionScope struct {
	acts        connectors.InboundActions
	conn, event string
	key         string
}

func (a actionScope) release(c *Connector, owner uuid.UUID) {
	if a.acts == nil {
		return
	}
	if err := a.acts.ReleaseInbound(context.WithoutCancel(context.Background()), a.conn, a.event, a.key, owner); err != nil {
		c.log().Warn("telegram: a claim of a /start was not released", "error", err.Error())
	}
}

// deliverStartReply sends the kept reply of a handled /start once and finishes the update. The delivery is
// claimed first, so an overlapping delivery (a redelivery while the first request is still sending) does not
// send it again. A send that definitely did not happen (429, a message refused before it was sent) gives the
// claim up for the redelivery. One whose result is unknown (a 5xx, a cut connection) does too: the reply may
// then be sent twice, which is preferred to a reply that is lost. A refusal that can never pass (the chat
// blocked the bot) finishes the update.
func (c *Connector) deliverStartReply(ctx context.Context, b *bot, m *gotgbot.Message, conn connectors.Connection, scope actionScope) error {
	retry := func(err error) error { return fmt.Errorf("%w: %v", engine.ErrWebhookRetry, err) }
	d, claimed, err := scope.acts.ClaimDelivery(ctx, scope.conn, scope.event, scope.key)
	if err != nil {
		return retry(err)
	}
	if !claimed {
		if d.State == connectors.InboundDone {
			return nil
		}
		return retry(errors.New("the reply is being sent by another delivery"))
	}
	dctx, cancel := context.WithTimeout(ctx, startBudget)
	defer cancel()
	if d.Reply != "" {
		_, sent, err := c.send(dctx, b.api.Token, strconv.FormatInt(m.Chat.Id, 10), outgoing{Text: d.Reply})
		if err != nil {
			var refused *catalog.CallError
			if !errors.As(callFailure(err), &refused) {
				c.log().Warn("telegram: the reply to /start was not sent; it is retried", "error", err.Error())
				scope.release(c, d.Owner)
				return retry(err)
			}
			c.log().Warn("telegram: the reply to /start was not sent", "error", err.Error())
		} else {
			c.countStartReply(dctx, conn, d.Tenant, sent)
		}
	}
	if err := scope.acts.InboundCompleted(ctx, scope.conn, scope.event, scope.key, d.Owner); err != nil {
		// The reply went out; the record stays delivering and is abandoned with its claim's age.
		c.log().Warn("telegram: a handled /start was not finished in the record", "error", err.Error())
	}
	return nil
}

// sendStartReply sends the reply of a /start with a store that keeps no record of it: once, whatever the result.
func (c *Connector) sendStartReply(ctx context.Context, b *bot, m *gotgbot.Message, conn connectors.Connection, account uuid.UUID, reply string) {
	if reply == "" {
		return
	}
	_, sent, err := c.send(ctx, b.api.Token, strconv.FormatInt(m.Chat.Id, 10), outgoing{Text: reply})
	if err != nil {
		c.log().Warn("telegram: the reply to /start was not sent", "error", err.Error())
		return
	}
	c.countStartReply(ctx, conn, account, sent)
}

// countStartReply records the reply as a message of the tenant the link belongs to, counted for the bot that
// sent it.
func (c *Connector) countStartReply(ctx context.Context, conn connectors.Connection, account uuid.UUID, sent int) {
	if c.cfg.Usage == nil || account == uuid.Nil {
		return
	}
	err := c.cfg.Usage.Record(ctx, catalog.UsageEntry{TenantID: account, Kind: Kind, ConnectionID: connKey(conn), Unit: "message", Quantity: int64(sent), At: time.Now()})
	if err != nil {
		c.log().Warn("telegram: a message was not counted", "error", err.Error())
	}
}

// startEvent names the /start updates a connector handled itself, in the engine's dedup record.
const startEvent = "telegram.start"

// ---------------------------------------------------------------------------------------------
// presses of buttons

// press is what a press of a button leaves to its follow-up: the query to answer and the message to edit.
type press struct {
	queryID  string
	chatID   int64
	message  int64
	text     string
	entities []gotgbot.MessageEntity
	label    string
	expires  time.Time
}

type presses struct {
	mu sync.Mutex
	m  map[string]press
}

func (p *presses) put(key string, v press) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.m == nil {
		p.m = map[string]press{}
	}
	now := time.Now()
	for k, e := range p.m {
		if now.After(e.expires) {
			delete(p.m, k)
		}
	}
	v.expires = now.Add(2 * time.Minute)
	p.m[key] = v
}

func (p *presses) take(key string) (press, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	v, ok := p.m[key]
	delete(p.m, key)
	return v, ok
}

func pressKey(connection, dedup string) string { return connection + "/" + dedup }

func (c *Connector) callback(ctx context.Context, b *bot, u gotgbot.Update, conn connectors.Connection) ([]engine.InboundEvent, error) {
	cq := u.CallbackQuery
	token, idx, ok := parseCallback(cq.Data)
	if !ok {
		c.ack(ctx, b, cq.Id, "") // not a button of ours: the spinner stops, nothing else
		return nil, nil
	}
	chat, message := int64(0), int64(0)
	var original *gotgbot.Message
	switch m := cq.Message.(type) {
	case gotgbot.Message:
		original = &m
		chat, message = m.Chat.Id, m.MessageId
	case nil:
	default:
		chat, message = m.GetChat().Id, m.GetMessageId()
	}
	if message == 0 {
		c.ack(ctx, b, cq.Id, AlreadyAnswered)
		return nil, nil
	}
	tenant, open, err := c.cfg.Inbound.Correlation(ctx, token)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", engine.ErrWebhookRetry, err)
	}
	if open && !isPlatform(conn) && tenant != conn.TenantID {
		open = false // a token of another tenant's question is nobody's here
	}
	if !open {
		if err := c.closed(ctx, b, cq.Id, chat, message); err != nil {
			return nil, fmt.Errorf("%w: %v", engine.ErrWebhookRetry, err) // redelivered until the keyboard is off
		}
		return nil, nil
	}
	label := ""
	pr := press{queryID: cq.Id, chatID: chat, message: message}
	if original != nil {
		pr.text, pr.entities = original.Text, original.Entities
		if original.ReplyMarkup != nil {
			for _, row := range original.ReplyMarkup.InlineKeyboard {
				for _, btn := range row {
					if btn.CallbackData == cq.Data {
						label = btn.Text
					}
				}
			}
		}
	}
	pr.label = label
	key := connKey(conn)
	c.presses.put(pressKey(key, dedupKey(b.api.Token, u)), pr)
	chatID := strconv.FormatInt(chat, 10)
	raw, err := json.Marshal(map[string]any{
		"chatId": chatID, "messageId": strconv.FormatInt(message, 10), "choice": strconv.Itoa(idx), "label": label,
		"from": fromOf(&cq.From), "answeredAt": time.Now().UTC().Format(time.RFC3339),
	})
	if err != nil {
		return nil, err
	}
	return []engine.InboundEvent{{
		TenantID: tenant, Event: EventCallback, DedupKey: dedupKey(b.api.Token, u), Correlation: token, Payload: raw,
		Subject: engine.SubjectRef{Entity: graph.SubjectConversation, Key: key + ":" + chatID},
	}}, nil
}

// AnswerClosed answers a press of a button whose question is closed (answered, expired) while the update
// was handled: a second press, a press after the timeout.
func (c *Connector) AnswerClosed(ctx context.Context, ev engine.InboundEvent, conn connectors.Connection, creds connectors.Credentials) error {
	if ev.Event != EventCallback {
		return nil
	}
	pr, ok := c.presses.take(pressKey(connKey(conn), ev.DedupKey))
	if !ok {
		return nil
	}
	// A cleanup that fails is answered 503 like an accepted press's: Telegram redelivers it, and the
	// redelivery (the answer being closed) takes the keyboard off.
	return c.closed(ctx, c.bot(creds["token"]), pr.queryID, pr.chatID, pr.message)
}

// AnswerPublished follows the first accepted press: the press is acknowledged, the keyboard is taken off and
// the text carries the option that was chosen. It runs once the answer is committed, so only the press
// that counts changes the message.
func (c *Connector) AnswerPublished(ctx context.Context, ev engine.InboundEvent, conn connectors.Connection, creds connectors.Credentials) error {
	if ev.Event != EventCallback {
		return nil
	}
	pr, ok := c.presses.take(pressKey(connKey(conn), ev.DedupKey))
	if !ok {
		return nil
	}
	b := c.bot(creds["token"])
	c.ack(ctx, b, pr.queryID, "")
	text := pr.text
	if pr.label != "" && text != "" && len([]rune(text))+len([]rune(pr.label))+4 <= maxText {
		err := retrying(ctx, func() error {
			return b.editText(ctx, pr.chatID, pr.message, text+"\n\n→ "+pr.label, pr.entities)
		})
		if err == nil || notModified(err) {
			return nil
		}
	}
	// The keyboard is taken off here, with a few tries for what is transient. When even that fails the update
	// is answered 503: Telegram redelivers the press, which (its answer being closed by then) takes the
	// keyboard off again, and goes on redelivering until that succeeds. The chosen option is then not added to
	// the text, which only this first delivery has the text for.
	err := retrying(ctx, func() error { return b.removeKeyboard(ctx, pr.chatID, pr.message) })
	if err != nil && !notModified(err) {
		c.log().Warn("telegram: the keyboard was not removed", "error", err.Error())
		return err
	}
	return nil
}

// retrying runs fn up to three times, pausing between tries, while its failure is transient (a 5xx, a 429, a
// lost connection); a refusal of the request itself is returned at once.
func retrying(ctx context.Context, fn func() error) error {
	pause := 300 * time.Millisecond
	var err error
	for attempt := 0; attempt < 3; attempt++ {
		if err = fn(); err == nil {
			return nil
		}
		var api *apiError
		wait := pause
		switch {
		case errors.As(err, &api) && api.Code == 429:
			wait = max(api.RetryAfter, pause)
			if wait > 3*time.Second {
				return err
			}
		case errors.As(err, &api) && api.Code < 500:
			return err
		}
		if ctx.Err() != nil || attempt == 2 {
			return err
		}
		select {
		case <-ctx.Done():
			return err
		case <-time.After(wait):
		}
		pause *= 3
	}
	return err
}

// closed answers a press of a closed question and takes its keyboard off. It does not change the text.
func (c *Connector) closed(ctx context.Context, b *bot, queryID string, chat, message int64) error {
	c.ack(ctx, b, queryID, AlreadyAnswered)
	err := retrying(ctx, func() error { return b.removeKeyboard(ctx, chat, message) })
	if err != nil && !notModified(err) {
		c.log().Warn("telegram: the keyboard was not removed", "error", err.Error())
		return err
	}
	return nil
}

// ack acknowledges a press; Telegram stops showing the spinner. A failure is logged only: the press is
// handled either way.
func (c *Connector) ack(ctx context.Context, b *bot, queryID, text string) {
	// A transient failure is tried again a few times; a refusal (the query is too old) is final.
	if err := retrying(ctx, func() error { return b.answerCallback(ctx, queryID, text) }); err != nil {
		c.log().Warn("telegram: a press was not acknowledged", "error", err.Error())
	}
}

// notModified reports Telegram's "message is not modified" or "message to edit not found": the keyboard is
// gone already.
func notModified(err error) bool {
	var api *apiError
	if !errors.As(err, &api) || api.Code != 400 {
		return false
	}
	d := strings.ToLower(api.Description)
	return strings.Contains(d, "not modified") || strings.Contains(d, "message to edit not found") || strings.Contains(d, "message can't be edited")
}
