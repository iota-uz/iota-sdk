package telegram

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
)

// defaultAPIURL is the Bot API origin.
const defaultAPIURL = "https://api.telegram.org"

// Timeouts of Bot API requests: a call node has its own deadline above them.
const (
	requestTimeout = 10 * time.Second
	uploadTimeout  = 60 * time.Second
)

// apiError is a refusal of the Bot API: the HTTP-like code, the description Telegram gave and, for 429,
// when to repeat. It carries no URL, token or request parameter.
type apiError struct {
	Method      string
	Code        int
	Description string
	RetryAfter  time.Duration
	// Migrated is the new id of a group that became a supergroup.
	Migrated int64
}

func (e *apiError) Error() string {
	// "status <code>" is where the integrations service reads a status from.
	return fmt.Sprintf("telegram %s: status %d: %s", e.Method, e.Code, e.Description)
}

// transportError is a request that failed before an answer (a dial, a timeout, a cut connection). The
// request may have reached Telegram. Its text names the method and the kind of failure only: the
// underlying error holds the URL, and the URL holds the bot token.
type transportError struct {
	Method string
	kind   string
	ctx    error // context.Canceled or context.DeadlineExceeded, when that was the cause
}

func (e *transportError) Error() string { return "telegram " + e.Method + ": " + e.kind }
func (e *transportError) Unwrap() error { return e.ctx }

// client is the gotgbot transport with the bot token kept out of every error.
type client struct {
	inner *gotgbot.BaseBotClient
}

var _ gotgbot.BotClient = (*client)(nil)

func newClient(h *http.Client, apiURL string) *client {
	if h == nil {
		h = &http.Client{}
	}
	return &client{inner: &gotgbot.BaseBotClient{
		Client:             *h,
		DefaultRequestOpts: &gotgbot.RequestOpts{APIURL: apiURL, Timeout: requestTimeout},
	}}
}

func (c *client) RequestWithContext(ctx context.Context, token, method string, params map[string]string, data map[string]gotgbot.FileReader, opts *gotgbot.RequestOpts) (json.RawMessage, error) {
	raw, err := c.inner.RequestWithContext(ctx, token, method, params, data, opts)
	if err == nil {
		return raw, nil
	}
	var tg *gotgbot.TelegramError
	if errors.As(err, &tg) {
		out := &apiError{Method: method, Code: tg.Code, Description: tg.Description}
		if p := tg.ResponseParams; p != nil {
			out.RetryAfter = time.Duration(p.RetryAfter) * time.Second
			out.Migrated = p.MigrateToChatId
		}
		return nil, out
	}
	out := &transportError{Method: method, kind: "the request failed"}
	switch {
	case errors.Is(err, context.Canceled):
		out.kind, out.ctx = "the request was cancelled", context.Canceled
	case errors.Is(err, context.DeadlineExceeded):
		out.kind, out.ctx = "the request timed out", context.DeadlineExceeded
	case strings.Contains(err.Error(), "failed to decode"):
		out.kind = "the answer is not a Bot API answer"
	}
	return nil, out
}

func (c *client) GetAPIURL(opts *gotgbot.RequestOpts) string { return c.inner.GetAPIURL(opts) }

func (c *client) FileURL(token, path string, opts *gotgbot.RequestOpts) string {
	return c.inner.FileURL(token, path, opts)
}

// bot is one bot's Bot API: its token and the transport.
type bot struct {
	api *gotgbot.Bot
	c   *client
	h   *http.Client
}

func (c *Connector) bot(token string) *bot {
	cl := newClient(c.cfg.HTTPClient, c.apiURL)
	return &bot{api: &gotgbot.Bot{Token: token, BotClient: cl}, c: cl, h: c.cfg.HTTPClient}
}

// call runs a Bot API method and decodes its result into out (nil: the result is ignored).
func (b *bot) call(ctx context.Context, method string, params map[string]string, files map[string]gotgbot.FileReader, out any) error {
	timeout := requestTimeout
	if len(files) > 0 {
		timeout = uploadTimeout
	}
	raw, err := b.api.RequestWithContext(ctx, method, params, files, &gotgbot.RequestOpts{Timeout: timeout})
	if err != nil {
		return err
	}
	if out == nil || len(raw) == 0 {
		return nil
	}
	if err := json.Unmarshal(raw, out); err != nil {
		return &transportError{Method: method, kind: "the answer is not readable"}
	}
	return nil
}

func (b *bot) getMe(ctx context.Context) (*gotgbot.User, error) {
	var u gotgbot.User
	if err := b.call(ctx, "getMe", nil, nil, &u); err != nil {
		return nil, err
	}
	return &u, nil
}

func (b *bot) webhookInfo(ctx context.Context) (*gotgbot.WebhookInfo, error) {
	var w gotgbot.WebhookInfo
	if err := b.call(ctx, "getWebhookInfo", nil, nil, &w); err != nil {
		return nil, err
	}
	return &w, nil
}

func (b *bot) setWebhook(ctx context.Context, url, secret string) error {
	allowed, err := json.Marshal([]string{"message", "callback_query"})
	if err != nil {
		return err
	}
	return b.call(ctx, "setWebhook", map[string]string{"url": url, "secret_token": secret, "allowed_updates": string(allowed)}, nil, nil)
}

func (b *bot) deleteWebhook(ctx context.Context) error {
	return b.call(ctx, "deleteWebhook", nil, nil, nil)
}

func (b *bot) answerCallback(ctx context.Context, id, text string) error {
	p := map[string]string{"callback_query_id": id}
	if text != "" {
		p["text"] = text
	}
	return b.call(ctx, "answerCallbackQuery", p, nil, nil)
}

// removeKeyboard takes the inline keyboard off a message, whatever its text.
func (b *bot) removeKeyboard(ctx context.Context, chat, message int64) error {
	return b.call(ctx, "editMessageReplyMarkup", map[string]string{
		"chat_id": strconv.FormatInt(chat, 10), "message_id": strconv.FormatInt(message, 10), "reply_markup": `{"inline_keyboard":[]}`,
	}, nil, nil)
}

// editText replaces a message's text (keeping its entities) and takes its keyboard off.
func (b *bot) editText(ctx context.Context, chat, message int64, text string, entities []gotgbot.MessageEntity) error {
	p := map[string]string{
		"chat_id": strconv.FormatInt(chat, 10), "message_id": strconv.FormatInt(message, 10), "text": text,
		"reply_markup": `{"inline_keyboard":[]}`,
	}
	if len(entities) > 0 {
		raw, err := json.Marshal(entities)
		if err != nil {
			return err
		}
		p["entities"] = string(raw)
	}
	return b.call(ctx, "editMessageText", p, nil, nil)
}

// getFile resolves a file id into its path at the Bot API.
func (b *bot) getFile(ctx context.Context, id string) (*gotgbot.File, error) {
	var f gotgbot.File
	if err := b.call(ctx, "getFile", map[string]string{"file_id": id}, nil, &f); err != nil {
		return nil, err
	}
	return &f, nil
}

// download reads a file of the Bot API up to limit bytes; a longer file is errTooLarge.
func (b *bot) download(ctx context.Context, f *gotgbot.File, limit int64) ([]byte, error) {
	if f.FilePath == "" {
		return nil, errors.New("telegram getFile: no file path")
	}
	url := b.c.FileURL(b.api.Token, f.FilePath, &gotgbot.RequestOpts{})
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, &transportError{Method: "file", kind: "the request failed"}
	}
	h := b.h
	if h == nil {
		h = http.DefaultClient
	}
	ctx, cancel := context.WithTimeout(ctx, uploadTimeout)
	defer cancel()
	resp, err := h.Do(req.WithContext(ctx))
	if err != nil {
		return nil, &transportError{Method: "file", kind: "the download failed"}
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, &apiError{Method: "file", Code: resp.StatusCode, Description: "the file was not served"}
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, limit+1))
	if err != nil {
		return nil, &transportError{Method: "file", kind: "the download was cut"}
	}
	if int64(len(data)) > limit {
		return nil, errTooLarge
	}
	return data, nil
}

var errTooLarge = errors.New("the file is larger than the Bot API allows")
