package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/PaulSonOfLars/gotgbot/v2"

	"github.com/iota-uz/iota-sdk/pkg/automation/catalog"
)

// Client sends through a connection or the platform bot: for SDK code that is not a graph node (the
// notification channel of #1349). The Connector implements it.
type Client interface {
	Send(ctx context.Context, conn catalog.ConnectionInfo, creds catalog.Credentials, msg Message) (MessageRef, error)
}

// Message is a message to a chat.
type Message struct {
	ChatID    int64
	ThreadID  int64      // message_thread_id of a forum topic; 0 — no topic
	Text      string     // ≤ 4096 chars after rendering
	ParseMode string     // "" | "HTML" (MarkdownV2 is not offered)
	Buttons   [][]Button // inline keyboard
	Silent    bool
}

// Button is an inline button: exactly one of URL and CallbackData.
type Button struct{ Text, URL, CallbackData string }

// MessageRef names a message that was sent.
type MessageRef struct{ ChatID, MessageID int64 }

// Limits of the Bot API a message is checked against before it is sent.
const (
	maxText         = 4096
	maxCaption      = 1024
	maxCallbackData = 64
	maxButtons      = 100
)

// Errors a message is refused with before anything is sent.
var (
	ErrInvalidMessage = errors.New("telegram: invalid message")
)

// RetryAfterError is Telegram asking to repeat a request later (429).
type RetryAfterError struct{ After time.Duration }

func (e *RetryAfterError) Error() string {
	return fmt.Sprintf("telegram: too many requests, repeat in %s", e.After)
}

// Send sends msg with the bot of creds. A refusal of the chat or of the message is a
// *catalog.CallError (rejected, invalid_request, unauthorized); 429 is a *RetryAfterError; anything else
// (a cut connection, a 5xx) is an error whose outcome is unknown: the message may have been sent.
func (c *Connector) Send(ctx context.Context, _ catalog.ConnectionInfo, creds catalog.Credentials, msg Message) (MessageRef, error) {
	token := creds["token"]
	if token == "" {
		return MessageRef{}, &catalog.CallError{Code: catalog.CallUnauthorized, Message: "the connection has no bot token"}
	}
	if msg.ChatID == 0 {
		return MessageRef{}, fmt.Errorf("%w: no chat", ErrInvalidMessage)
	}
	ref, _, err := c.send(ctx, token, strconv.FormatInt(msg.ChatID, 10), outgoing{
		Text: msg.Text, ParseMode: msg.ParseMode, Thread: msg.ThreadID, Buttons: msg.Buttons, Silent: msg.Silent,
	})
	return ref, callFailure(err)
}

var _ Client = (*Connector)(nil)

// NotSent reports whether an error of Send says the message was not sent, so sending it again cannot
// duplicate it: the Bot API answered with a server error (5xx), or the connection to it was never made. Any
// other error that is neither a *catalog.CallError nor a *RetryAfterError leaves the outcome unknown (a
// timeout or a cut connection after the request went out).
func NotSent(err error) bool {
	var api *apiError
	if errors.As(err, &api) {
		return api.Code >= 500
	}
	var tr *transportError
	return errors.As(err, &tr) && tr.dial
}

// outgoing is a message the connector sends: text or the caption of files, a keyboard, a topic.
type outgoing struct {
	Text      string
	ParseMode string
	Thread    int64
	Buttons   [][]Button
	Silent    bool
	Files     []file
}

// file is an attachment to send.
type file struct {
	Name string
	Mime string
	Data []byte
}

// send checks and sends one outgoing message (one request) to chat, a numeric id or @username. It returns
// the first message and how many messages Telegram delivered (an album of files is several).
func (c *Connector) send(ctx context.Context, token, chat string, m outgoing) (MessageRef, int, error) {
	if err := m.check(); err != nil {
		return MessageRef{}, 0, err
	}
	p := map[string]string{"chat_id": chat}
	if m.Thread != 0 {
		p["message_thread_id"] = strconv.FormatInt(m.Thread, 10)
	}
	if m.Silent {
		p["disable_notification"] = "true"
	}
	if m.ParseMode != "" {
		p["parse_mode"] = m.ParseMode
	}
	if markup, err := keyboard(m.Buttons); err != nil {
		return MessageRef{}, 0, err
	} else if markup != "" {
		p["reply_markup"] = markup
	}
	b := c.bot(token)
	var sent gotgbot.Message
	var err error
	delivered := 1
	switch len(m.Files) {
	case 0:
		p["text"] = m.Text
		err = b.call(ctx, "sendMessage", p, nil, &sent)
	case 1:
		f := m.Files[0]
		method, field := "sendDocument", "document"
		if isPhoto(f) {
			method, field = "sendPhoto", "photo"
		}
		if m.Text != "" {
			p["caption"] = m.Text
		}
		err = b.call(ctx, method, p, map[string]gotgbot.FileReader{field: fileReader(f)}, &sent)
	default:
		var group []gotgbot.Message
		media := make([]map[string]string, len(m.Files))
		data := make(map[string]gotgbot.FileReader, len(m.Files))
		for i, f := range m.Files {
			key := "file" + strconv.Itoa(i)
			media[i] = map[string]string{"type": "document", "media": "attach://" + key}
			if i == 0 && m.Text != "" { // the caption of a group is its first item's
				media[i]["caption"] = m.Text
				if m.ParseMode != "" {
					media[i]["parse_mode"] = m.ParseMode
				}
			}
			data[key] = fileReader(f)
		}
		raw, merr := json.Marshal(media)
		if merr != nil {
			return MessageRef{}, 0, merr
		}
		delete(p, "parse_mode")
		delete(p, "reply_markup") // a media group takes no keyboard
		p["media"] = string(raw)
		if err = b.call(ctx, "sendMediaGroup", p, data, &group); err == nil {
			if len(group) == 0 {
				return MessageRef{}, 0, &transportError{Method: "sendMediaGroup", kind: "the answer has no message"}
			}
			sent, delivered = group[0], len(group)
		}
	}
	if err != nil {
		return MessageRef{}, 0, err
	}
	if sent.Chat.Id == 0 || sent.MessageId == 0 {
		return MessageRef{}, 0, &transportError{Method: "send", kind: "the answer names no message"}
	}
	return MessageRef{ChatID: sent.Chat.Id, MessageID: sent.MessageId}, delivered, nil
}

func fileReader(f file) gotgbot.FileReader {
	return gotgbot.FileReader{Name: f.Name, Data: bytes.NewReader(f.Data)}
}

// isPhoto reports whether a file goes as a photo: an image Telegram shows inline (≤ 10 MB, JPEG, PNG,
// WebP or GIF-less).
func isPhoto(f file) bool {
	if len(f.Data) > 10<<20 {
		return false
	}
	switch strings.ToLower(f.Mime) {
	case "image/jpeg", "image/png":
		return true
	}
	return false
}

func (m outgoing) check() error {
	limit := maxText
	if len(m.Files) > 0 {
		limit = maxCaption
	}
	n := utf8.RuneCountInString(m.Text)
	switch {
	case len(m.Files) == 0 && strings.TrimSpace(m.Text) == "":
		return fmt.Errorf("%w: the text is empty", ErrInvalidMessage)
	case n > limit:
		return fmt.Errorf("%w: the text has %d characters, at most %d", ErrInvalidMessage, n, limit)
	case m.ParseMode != "" && m.ParseMode != "HTML":
		return fmt.Errorf("%w: parse mode is empty or HTML", ErrInvalidMessage)
	case m.Thread < 0:
		return fmt.Errorf("%w: a topic id is positive", ErrInvalidMessage)
	case len(m.Files) > 10:
		return fmt.Errorf("%w: at most 10 files", ErrInvalidMessage)
	case len(m.Files) > 1 && len(m.Buttons) > 0:
		return fmt.Errorf("%w: an album of files takes no buttons: send one file with them, or the buttons with a text", ErrInvalidMessage)
	}
	return nil
}

// keyboard is the inline keyboard as the Bot API takes it, "" for none.
func keyboard(rows [][]Button) (string, error) {
	if len(rows) == 0 {
		return "", nil
	}
	total := 0
	out := make([][]map[string]string, 0, len(rows))
	for _, row := range rows {
		if len(row) == 0 {
			continue
		}
		r := make([]map[string]string, 0, len(row))
		for _, b := range row {
			total++
			switch {
			case strings.TrimSpace(b.Text) == "":
				return "", fmt.Errorf("%w: a button has no text", ErrInvalidMessage)
			case (b.URL == "") == (b.CallbackData == ""):
				return "", fmt.Errorf("%w: a button has exactly one of a URL and callback data", ErrInvalidMessage)
			case len(b.CallbackData) > maxCallbackData:
				return "", fmt.Errorf("%w: callback data is at most %d bytes", ErrInvalidMessage, maxCallbackData)
			case total > maxButtons:
				return "", fmt.Errorf("%w: at most %d buttons", ErrInvalidMessage, maxButtons)
			}
			if b.URL != "" {
				r = append(r, map[string]string{"text": b.Text, "url": b.URL})
			} else {
				r = append(r, map[string]string{"text": b.Text, "callback_data": b.CallbackData})
			}
		}
		out = append(out, r)
	}
	if len(out) == 0 {
		return "", nil
	}
	raw, err := json.Marshal(map[string]any{"inline_keyboard": out})
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

// callFailure maps an error of a send to what callers of Send read: a CallError for a refusal, a
// RetryAfterError for 429, the error itself when the outcome is unknown. A message that was refused
// before anything was sent is an invalid_request.
func callFailure(err error) error {
	if err == nil {
		return nil
	}
	var api *apiError
	switch {
	case errors.Is(err, ErrInvalidMessage):
		return &catalog.CallError{Code: catalog.CallInvalidRequest, Message: err.Error()}
	case !errors.As(err, &api):
		return err
	case api.Code == 429:
		after := api.RetryAfter
		if after <= 0 {
			after = time.Second
		}
		return &RetryAfterError{After: after}
	case api.Code == 401:
		return &catalog.CallError{Code: catalog.CallUnauthorized, Message: "Telegram did not accept the bot token"}
	case api.Code >= 500:
		return err // the message may have been sent
	case api.Code == 403 || rejectedChat(api.Description):
		return &catalog.CallError{Code: catalog.CallRejected, Message: "Telegram refused the chat: " + api.Description}
	}
	return &catalog.CallError{Code: catalog.CallInvalidRequest, Message: "Telegram refused the message: " + api.Description}
}

// rejectedChat reports a 400 that is about the chat (it is gone, the bot may not write there) rather than
// about the message.
func rejectedChat(description string) bool {
	d := strings.ToLower(description)
	for _, s := range []string{"chat not found", "user not found", "not enough rights", "have no rights", "bot was kicked", "bot is not a member",
		"user is deactivated", "peer_id_invalid", "message thread not found", "topic_closed", "group chat was upgraded", "chat_write_forbidden"} {
		if strings.Contains(d, s) {
			return true
		}
	}
	return false
}
