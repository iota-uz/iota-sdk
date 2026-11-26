// Package telegramchannel is the Telegram notification channel: the channel
// adapter that sends a notice to the private chat a user linked with the
// tenant's bot, and the telegram.StartHandler that completes that link when
// the user presses Start on the deep link https://t.me/<bot>?start=u_<token>.
//
// The adapter is one per application. On every send it picks the connection
// of the message's tenant (the one the tenant chose for the channel, else the
// platform bot enabled for it) and sends through the Telegram connector's
// client. The automations module contributes it when the application
// registers the Telegram connector.
package telegramchannel

import (
	"context"
	"errors"
	"fmt"
	"html"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/iota-uz/iota-sdk/modules/core/notifications"
	"github.com/iota-uz/iota-sdk/pkg/automation/catalog"
	"github.com/iota-uz/iota-sdk/pkg/automation/connectors"
	"github.com/iota-uz/iota-sdk/pkg/automation/connectors/telegram"
)

// Prefix is the /start payload prefix of a link token.
const Prefix = "u_"

// maxText is the Bot API's limit of a message text.
const maxText = 4096

// BotIdentity asks a bot who it is (the connector's Test: getMe).
type BotIdentity interface {
	Test(ctx context.Context, creds connectors.Credentials) (connectors.Identity, error)
}

// Config configures the channel.
type Config struct {
	// Client sends through a connection (the Telegram connector).
	Client telegram.Client
	// Bots names the bot of a connection for the deep link (the Telegram connector).
	Bots BotIdentity
	// Credentials opens the connection a message is sent through.
	Credentials connectors.CredentialSource
	// Connections picks the tenant's connection of the channel.
	Connections notifications.ChannelConnections
	// PublicOrigin is the https origin that makes a notice's link absolute for
	// its button; without it a relative link has no button.
	PublicOrigin string
}

// Channel is the Telegram channel adapter.
type Channel struct {
	cfg   Config
	mu    sync.Mutex
	names map[string]botName
}

type botName struct {
	username string
	at       time.Time
}

// botNameTTL is how long the username of a connection's bot is kept.
const botNameTTL = 10 * time.Minute

var _ notifications.ChannelAdapter = (*Channel)(nil)

// New is the channel adapter.
func New(cfg Config) *Channel {
	cfg.PublicOrigin = strings.TrimRight(strings.TrimSpace(cfg.PublicOrigin), "/")
	return &Channel{cfg: cfg, names: map[string]botName{}}
}

// Spec is the registry entry of the channel.
func (c *Channel) Spec() notifications.ChannelSpec {
	return notifications.ChannelSpec{
		Channel:       notifications.ChannelTelegram,
		Label:         map[string]string{"en": "Telegram", "ru": "Telegram", "uz": "Telegram", "uz-Cyrl": "Telegram", "pt-BR": "Telegram", "zh": "Telegram"},
		Order:         40,
		Address:       notifications.AddressLinked,
		ConnectorKind: string(telegram.Kind),
		Summary:       func(content notifications.ChannelContent) (string, string) { return content.Title, content.Body },
		LinkURL:       c.LinkURL,
	}
}

// Channel implements notifications.ChannelAdapter.
func (c *Channel) Channel() notifications.Channel { return notifications.ChannelTelegram }

// open opens the connection the tenant's channel sends through now.
func (c *Channel) open(ctx context.Context, tenantID uuid.UUID) (catalog.ConnectionInfo, connectors.Credentials, error) {
	if c.cfg.Connections == nil || c.cfg.Credentials == nil {
		return catalog.ConnectionInfo{}, nil, notifications.ErrChannelUnavailable
	}
	ref, ok, err := c.cfg.Connections.Connection(ctx, tenantID, c.Spec())
	if err != nil {
		return catalog.ConnectionInfo{}, nil, err
	}
	if !ok {
		return catalog.ConnectionInfo{}, nil, notifications.ErrChannelUnavailable
	}
	return c.openRef(ctx, tenantID, ref)
}

func (c *Channel) openRef(ctx context.Context, tenantID uuid.UUID, ref string) (catalog.ConnectionInfo, connectors.Credentials, error) {
	info, creds, err := c.cfg.Credentials.Open(ctx, tenantID, ref)
	switch {
	case errors.Is(err, connectors.ErrNotFound), errors.Is(err, connectors.ErrNotConnected), errors.Is(err, connectors.ErrNeedsReauth):
		return catalog.ConnectionInfo{}, nil, fmt.Errorf("%w: %w", notifications.ErrChannelUnavailable, err)
	case err != nil:
		return catalog.ConnectionInfo{}, nil, err
	}
	return info, creds, nil
}

// LinkURL is the deep link that confirms a link token through a connection's bot.
func (c *Channel) LinkURL(ctx context.Context, tenantID uuid.UUID, connectionID, token string) (string, error) {
	if c.cfg.Bots == nil || c.cfg.Credentials == nil {
		return "", notifications.ErrChannelUnavailable
	}
	_, creds, err := c.openRef(ctx, tenantID, connectionID)
	if err != nil {
		return "", err
	}
	// Keyed by the connection and its bot: a replaced token is another bot.
	key := connectionID + "|" + botKey(creds["token"])
	c.mu.Lock()
	name, ok := c.names[key]
	c.mu.Unlock()
	if !ok || time.Since(name.at) > botNameTTL {
		identity, err := c.cfg.Bots.Test(ctx, creds)
		if err != nil {
			return "", err
		}
		username := strings.TrimPrefix(strings.TrimSpace(identity.Display), "@")
		if username == "" {
			return "", errors.New("telegram: the bot has no username")
		}
		name = botName{username: username, at: time.Now()}
		c.mu.Lock()
		c.names[key] = name
		c.mu.Unlock()
	}
	return "https://t.me/" + name.username + "?start=" + Prefix + token, nil
}

// botKey is the bot id part of a token ("<id>:<secret>"): enough to tell bots apart without keeping a secret.
func botKey(token string) string {
	id, _, _ := strings.Cut(token, ":")
	return id
}

// Send implements notifications.ChannelAdapter.
func (c *Channel) Send(ctx context.Context, m notifications.Message) (notifications.Receipt, error) {
	chat, err := strconv.ParseInt(strings.TrimSpace(m.Address), 10, 64)
	if err != nil || chat == 0 {
		return notifications.Receipt{}, fmt.Errorf("%w: telegram: the address is not a chat id", notifications.ErrPermanent)
	}
	if c.cfg.Client == nil {
		return notifications.Receipt{}, notifications.ErrChannelUnavailable
	}
	info, creds, err := c.open(ctx, m.TenantID)
	if err != nil {
		return notifications.Receipt{}, err
	}
	msg := telegram.Message{ChatID: chat, Text: Render(m.Title, m.Body), ParseMode: "HTML"}
	if link := c.absolute(m.ActionURL); link != "" {
		msg.Buttons = [][]telegram.Button{{{Text: phrase(m.Language, phraseOpen), URL: link}}}
	}
	ref, err := c.cfg.Client.Send(ctx, info, creds, msg)
	if err != nil {
		return notifications.Receipt{}, deliveryError(err)
	}
	return notifications.Receipt{ProviderMessageID: strconv.FormatInt(ref.ChatID, 10) + ":" + strconv.FormatInt(ref.MessageID, 10)}, nil
}

// absolute is the notice's link as an https URL a button can carry, "" when there is none.
func (c *Channel) absolute(link string) string {
	link = strings.TrimSpace(link)
	switch {
	case link == "":
		return ""
	case strings.HasPrefix(link, "https://") || strings.HasPrefix(link, "http://"):
		return link
	case strings.HasPrefix(link, "/") && !strings.HasPrefix(link, "//") && c.cfg.PublicOrigin != "":
		return c.cfg.PublicOrigin + link
	}
	return ""
}

// deliveryError maps an error of the connector's Send to the delivery layer:
// 429 repeats after retry_after; a server error or a connection never made
// repeats; a blocked bot or a missing chat is a permanent loss of the target;
// another refusal is permanent; anything else may have been sent (ambiguous:
// sendMessage is not idempotent).
func deliveryError(err error) error {
	var later *telegram.RetryAfterError
	if errors.As(err, &later) {
		return &notifications.RetryAfterError{After: later.After}
	}
	var call *catalog.CallError
	if errors.As(err, &call) {
		if call.Code == catalog.CallRejected && targetGone(call.Message) {
			return fmt.Errorf("%w: %w: telegram: %s", notifications.ErrPermanent, notifications.ErrTargetGone, call.Code)
		}
		return fmt.Errorf("%w: telegram: %s", notifications.ErrPermanent, call.Code)
	}
	if errors.Is(err, notifications.ErrChannelUnavailable) {
		return err
	}
	if telegram.NotSent(err) {
		return fmt.Errorf("telegram: the message was not sent: %w", err)
	}
	return fmt.Errorf("%w: telegram: the outcome of the send is unknown", notifications.ErrAmbiguous)
}

// targetGone reports a refusal that says the chat cannot be written to any more.
func targetGone(message string) bool {
	m := strings.ToLower(message)
	for _, s := range []string{"blocked by the user", "chat not found", "user is deactivated", "bot was kicked", "user not found", "forbidden", "peer_id_invalid"} {
		if strings.Contains(m, s) {
			return true
		}
	}
	return false
}

// Render is the HTML text of a notice: the title in bold, then the body, both
// escaped, cut with an ellipsis to the Bot API's 4096 characters.
func Render(title, body string) string {
	title, body = strings.TrimSpace(title), strings.TrimSpace(body)
	budget := maxText
	var b strings.Builder
	if title != "" {
		const open, closeTag = "<b>", "</b>"
		b.WriteString(open)
		budget -= utf8.RuneCountInString(open + closeTag)
		budget = writeEscaped(&b, title, budget)
		b.WriteString(closeTag)
		if body != "" && budget > 3 {
			b.WriteString("\n\n")
			budget -= 2
		}
	}
	if body != "" && budget > 1 {
		writeEscaped(&b, body, budget)
	}
	return b.String()
}

// writeEscaped writes s escaped within budget runes, ending with "…" when it
// is cut; it returns the budget left.
func writeEscaped(b *strings.Builder, s string, budget int) int {
	escaped := html.EscapeString(s)
	if n := utf8.RuneCountInString(escaped); n <= budget {
		b.WriteString(escaped)
		return budget - n
	}
	budget-- // the ellipsis
	for _, r := range s {
		part := html.EscapeString(string(r))
		n := utf8.RuneCountInString(part)
		if n > budget {
			break
		}
		b.WriteString(part)
		budget -= n
	}
	b.WriteString("…")
	return 0
}
