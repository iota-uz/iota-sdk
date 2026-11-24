// Package telegram is the Telegram bot connector of automation graphs: it sends a message, asks a
// question with buttons and takes the pressed one as the outcome of the node, waits for a text message in a
// chat, and starts graphs on a message to a tenant's own bot.
//
// A tenant connects a bot by its token. When the application sets APP_TELEGRAMBOTTOKEN, the platform's bot
// is offered to every tenant as the platform credential (policy auto): a tenant sends and asks through it,
// but a message to it belongs to no tenant, so it starts no graph. The only updates of the platform bot
// that reach a tenant are the press of a button of that tenant's question (by the question's correlation
// token) and `/start <payload>` with a prefix a StartHandler registered.
//
// Updates come through a webhook with a secret: saving a connection calls setWebhook with
// /automations/hooks/{connection_id} (the platform bot: /automations/hooks/platform/telegram_bot),
// deleting or disabling one calls deleteWebhook. The route, the correlations, the dedup and the wait
// for an answer are the engine's (pkg/automation/engine).
//
// Beyond the platform's own requests the connector never keeps or logs a token: the Bot API URL holds it,
// so no error of a request carries a URL.
package telegram

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"sync/atomic"

	"github.com/google/uuid"

	"github.com/iota-uz/iota-sdk/pkg/automation/catalog"
	"github.com/iota-uz/iota-sdk/pkg/automation/connectors"
	"github.com/iota-uz/iota-sdk/pkg/automation/engine"
)

// Kind is the kind of the connector.
const Kind catalog.ConnectorKind = "telegram_bot"

// Warnings of Advise: codes the integrations screen shows a phrase for. Both read «this bot is already
// used by another service; after connecting it that service stops receiving its messages».
const (
	WarnBotInUse       = "bot_in_use"
	WarnForeignWebhook = "foreign_webhook"
	WarnPollingUnknown = "polling_unknown"
)

// Config configures the connector.
type Config struct {
	// PlatformToken is APP_TELEGRAMBOTTOKEN; empty: no platform credential.
	PlatformToken string
	// PublicOrigin is the https origin of the application the webhooks point to.
	PublicOrigin string
	Inbound      engine.Inbound
	HTTPClient   *http.Client

	// APIURL is the origin of the Bot API (default https://api.telegram.org); a test points it at
	// telegramtest.
	APIURL string
	// Uploads stores the files of incoming messages and reads the files of a send; without it a file is
	// not stored (files[].error says so) and a send with files is refused.
	Uploads Uploads
	// Connections records a connection that Telegram refused (needs_reauth) and the chats a bot
	// receives messages from; without it neither is recorded.
	Connections ConnectionState
	// Usage counts the messages the connector sends outside a node (the reply to /start).
	Usage  catalog.UsageRecorder
	Logger *slog.Logger
}

// ConnectionState is the part of connectors.ConnectionStore the connector writes to.
type ConnectionState interface {
	SetStatus(ctx context.Context, tenantID, id uuid.UUID, s connectors.ConnectionStatus, reason string) error
	UpdateInbound(ctx context.Context, tenantID, id uuid.UUID, fn func(state json.RawMessage) (json.RawMessage, error)) error
}

// Connector is the Telegram bot connector: connectors.Connector, engine.WebhookReceiver and Client.
type Connector struct {
	cfg     Config
	apiURL  string
	starts  starts
	chats   chatMemory
	presses presses
	// platformToken is the platform credential StartPlatform was given (the effective one).
	platformToken atomic.Pointer[string]
}

// New is the connector.
func New(cfg Config) *Connector {
	api := strings.TrimRight(cfg.APIURL, "/")
	if api == "" {
		api = defaultAPIURL
	}
	cfg.PublicOrigin = strings.TrimRight(strings.TrimSpace(cfg.PublicOrigin), "/")
	return &Connector{cfg: cfg, apiURL: api}
}

func (c *Connector) log() *slog.Logger {
	if c.cfg.Logger != nil {
		return c.cfg.Logger
	}
	return slog.Default()
}

func (c *Connector) Kind() catalog.ConnectorKind { return Kind }

const credentialsSchema = `{"type":"object","additionalProperties":false,"required":["token"],"properties":{
 "token":{"type":"string","x-secret":true,"minLength":10,"maxLength":200,"pattern":"^[0-9]{3,20}:[A-Za-z0-9_-]{20,100}$",
  "x-label":{"en":"Bot token","ru":"Токен бота","uz":"Bot tokeni"},
  "x-description":{"en":"The token @BotFather gave the bot.","ru":"Токен, который выдал @BotFather.","uz":"@BotFather bergan token."}}}}`

func (c *Connector) Descriptor() connectors.Descriptor {
	return connectors.Descriptor{
		Kind:        Kind,
		Title:       map[string]string{"en": "Telegram bot", "ru": "Telegram-бот", "uz": "Telegram bot"},
		Description: map[string]string{"en": "Send messages, ask with buttons and receive messages through a bot.", "ru": "Отправка сообщений, вопросы с кнопками и приём сообщений через бота."},
		Category:    connectors.CategoryMessaging,
		Icon:        "telegram",
		Auth:        connectors.AuthCustom,
		Credentials: json.RawMessage(credentialsSchema),
		Platform:    connectors.PlatformAuto,
		Inbound:     true,
	}
}

// Test asks the bot who it is (getMe).
func (c *Connector) Test(ctx context.Context, creds connectors.Credentials) (connectors.Identity, error) {
	token := creds["token"]
	if token == "" {
		return connectors.Identity{}, errors.New("telegram: no bot token (status 401)")
	}
	me, err := c.bot(token).getMe(ctx)
	if err != nil {
		return connectors.Identity{}, err
	}
	if me.Username == "" {
		return connectors.Identity{}, errors.New("telegram: getMe has no username")
	}
	return connectors.Identity{Display: "@" + me.Username, External: fmt.Sprint(me.Id), Kind: connectors.IdentityHandle}, nil
}

// hookURL is where Telegram posts the updates of a connection.
func (c *Connector) hookURL(conn connectors.Connection) string {
	if isPlatform(conn) {
		return c.cfg.PublicOrigin + "/automations/hooks/platform/" + string(Kind)
	}
	return c.cfg.PublicOrigin + "/automations/hooks/" + conn.ID.String()
}

func (c *Connector) hookPrefix() string { return c.cfg.PublicOrigin + "/automations/hooks/" }

// Who a bot's current webhook belongs to.
type hookOwner int

const (
	hookFree    hookOwner = iota // none, or one of a connection that is gone
	hookOwn                      // the URL this connection would set
	hookTaken                    // another live connection of this application, or its platform bot
	hookForeign                  // not this application's
)

// ownerOf classifies the webhook URL of a bot against own, the URL the connection being saved would set
// (empty for a new one). A URL of this application that names a connection is another's while that
// connection exists; one whose connection is gone is nobody's and may be taken over without a word.
func (c *Connector) ownerOf(ctx context.Context, url, own string) (hookOwner, error) {
	switch {
	case url == "":
		return hookFree, nil
	case own != "" && url == own:
		return hookOwn, nil
	case c.cfg.PublicOrigin == "" || !strings.HasPrefix(url, c.hookPrefix()):
		return hookForeign, nil
	}
	rest := strings.TrimPrefix(url, c.hookPrefix())
	if rest == "platform/"+string(Kind) {
		if c.platformLive() {
			return hookTaken, nil // the platform bot's route is live
		}
		return hookFree, nil
	}
	id, err := uuid.Parse(rest)
	if err != nil {
		return hookForeign, nil
	}
	checker, ok := c.cfg.Connections.(connectors.ExistenceChecker)
	if !ok {
		return hookTaken, nil // cannot tell: a bot is never taken from a connection that may be alive
	}
	found, err := checker.ConnectionExists(ctx, id)
	if err != nil {
		return hookTaken, err
	}
	if found {
		return hookTaken, nil
	}
	return hookFree, nil
}

// assess looks at the bot's current webhook and says what connecting the credentials would change
// elsewhere. A bot another live connection owns (or the platform's) is blocked: Telegram has one webhook per
// bot, so the second connection would silently take the first one's messages. A webhook another service set
// is a warning the person confirms. With no webhook at all, whether another service reads the bot by polling
// (getUpdates) cannot be told without disturbing it (a request for updates takes over the long poll of the
// service that has it), so a new connection needs the person's word that nobody does.
func (c *Connector) assess(ctx context.Context, creds connectors.Credentials, existing *connectors.Connection) ([]connectors.Warning, error) {
	b := c.bot(creds["token"])
	info, err := b.webhookInfo(ctx)
	if err != nil {
		return nil, err
	}
	own := ""
	if existing != nil {
		own = c.hookURL(*existing)
	}
	owner, err := c.ownerOf(ctx, info.Url, own)
	if err != nil {
		return nil, err
	}
	switch {
	case owner == hookTaken:
		return []connectors.Warning{{Code: WarnBotInUse, Blocking: true, Field: "token"}}, nil
	case owner == hookForeign:
		return []connectors.Warning{{Code: WarnForeignWebhook}}, nil
	case owner == hookOwn || info.Url != "" || existing != nil:
		return nil, nil // this connection's own, a stale one of a connection that is gone, or a bot it had
	}
	return []connectors.Warning{{Code: WarnPollingUnknown}}, nil
}

// Advise warns before a bot is connected (see assess). It changes nothing at Telegram.
func (c *Connector) Advise(ctx context.Context, existing *connectors.Connection, creds connectors.Credentials) ([]connectors.Warning, error) {
	return c.assess(ctx, creds, existing)
}

// secretKey is the key of the connection's inbound state the webhook secret is kept under.
const secretKey = "telegram_webhook"

// newSecret is a random webhook secret: 32 characters of [A-Za-z0-9_-].
func newSecret() (string, error) {
	raw := make([]byte, 24)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(raw)[:secretLen], nil
}

// Provision sets the connection's webhook with a generated secret, which it hands back for the service to
// keep in the connection's inbound state. It looks at the webhook as it is now: a bot another live connection
// owns is refused, and a foreign webhook (or a bot that may be polled) is replaced only when the person
// confirmed it, whatever advice an earlier look gave.
func (c *Connector) Provision(ctx context.Context, conn connectors.Connection, creds connectors.Credentials, consent []string) (map[string]json.RawMessage, error) {
	if c.cfg.PublicOrigin == "" {
		return nil, errors.New("telegram: the application's public origin is not configured (status 400)")
	}
	token := creds["token"]
	warnings, err := c.assess(ctx, creds, &conn)
	if err != nil {
		return nil, err
	}
	for _, w := range warnings {
		if w.Blocking {
			return nil, &connectors.BlockedError{Field: w.Field}
		}
	}
	// Consent is for the warning of a new connection too (the polling one), which the save was asked for.
	if open := connectors.Unconfirmed(warnings, consent); len(open) > 0 {
		return nil, &connectors.ConfirmationRequiredError{Warnings: open}
	}
	// A connection keeps its secret across saves and re-enables: a save that does not commit after the
	// provider was changed, or a competing one, then never leaves Telegram with a secret the SDK does not
	// hold. Only a connection with none (a new one) gets a generated one.
	secret, err := c.storedSecret(ctx, conn)
	if err != nil || secret == "" {
		if secret, err = newSecret(); err != nil {
			return nil, err
		}
	}
	if err := c.bot(token).setWebhook(ctx, c.hookURL(conn), secret); err != nil {
		return nil, err
	}
	raw, err := json.Marshal(map[string]string{"secret": secret})
	if err != nil {
		return nil, err
	}
	return map[string]json.RawMessage{secretKey: raw}, nil
}

// Deprovision removes the webhook of the bot only when it is exactly this connection's: one another
// connection or service has set since is left as it is.
func (c *Connector) Deprovision(ctx context.Context, conn connectors.Connection, creds connectors.Credentials) error {
	b := c.bot(creds["token"])
	info, err := b.webhookInfo(ctx)
	if err != nil {
		return err
	}
	if c.cfg.PublicOrigin == "" || info.Url != c.hookURL(conn) {
		return nil
	}
	return b.deleteWebhook(ctx)
}

// PlatformCredential is the platform bot of the application's configuration.
func (c *Connector) PlatformCredential() (connectors.PlatformCredential, bool) {
	if c.cfg.PlatformToken == "" {
		return connectors.PlatformCredential{}, false
	}
	return connectors.PlatformCredential{Credentials: connectors.Credentials{"token": c.cfg.PlatformToken}}, true
}

// ExternalID is the id of the bot a token belongs to, as Test reports it.
func (c *Connector) ExternalID(creds connectors.Credentials) string {
	if id := botID(creds["token"]); id != 0 {
		return strconv.FormatInt(id, 10)
	}
	return ""
}

// StartPlatform sets the platform bot's webhook at /automations/hooks/platform/telegram_bot, for the
// credential that is in effect (the configuration's when it names the kind, else APP_TELEGRAMBOTTOKEN).
func (c *Connector) StartPlatform(ctx context.Context, creds connectors.Credentials) error {
	token := creds["token"]
	if token == "" || c.cfg.PublicOrigin == "" {
		return nil
	}
	c.platformToken.Store(&token)
	// The platform bot has no connection row to keep a secret in: its secret is derived (WebhookSecret).
	conn := connectors.Connection{Kind: Kind, Name: "platform"}
	return c.bot(token).setWebhook(ctx, c.hookURL(conn), WebhookSecret(token, connKey(conn)))
}

// platformLive reports whether the platform bot's route is in use.
func (c *Connector) platformLive() bool {
	if t := c.platformToken.Load(); t != nil && *t != "" {
		return true
	}
	return c.cfg.PlatformToken != ""
}
