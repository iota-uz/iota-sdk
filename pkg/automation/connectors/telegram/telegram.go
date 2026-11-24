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
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

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
	WarnForeignWebhook = "foreign_webhook"
	WarnPollingInUse   = "polling_in_use"
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
	Logger      *slog.Logger
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

// Advise warns before a bot is connected that another service uses: a webhook that is not this
// application's, or a long poll (getUpdates) running. Connecting the bot replaces the webhook, so that
// service would stop receiving messages. It does not change anything at Telegram.
func (c *Connector) Advise(ctx context.Context, creds connectors.Credentials) ([]connectors.Warning, error) {
	b := c.bot(creds["token"])
	info, err := b.webhookInfo(ctx)
	if err != nil {
		return nil, err
	}
	if info.Url != "" {
		if c.cfg.PublicOrigin != "" && strings.HasPrefix(info.Url, c.hookPrefix()) {
			return nil, nil // this application's: an update of the same bot
		}
		return []connectors.Warning{{Code: WarnForeignWebhook}}, nil
	}
	// No webhook: another service may be polling. A request for the updates of a bot another one is
	// polling at the moment is refused with 409; it takes none (no offset confirms anything).
	var updates []json.RawMessage
	err = b.call(ctx, "getUpdates", map[string]string{"limit": "1", "timeout": "0", "allowed_updates": "[]"}, nil, &updates)
	var api *apiError
	if errors.As(err, &api) && api.Code == 409 {
		return []connectors.Warning{{Code: WarnPollingInUse}}, nil
	}
	return nil, nil
}

// Provision sets the connection's webhook, with a secret only this connection's Verify accepts.
func (c *Connector) Provision(ctx context.Context, conn connectors.Connection, creds connectors.Credentials) error {
	if c.cfg.PublicOrigin == "" {
		return errors.New("telegram: the application's public origin is not configured (status 400)")
	}
	token := creds["token"]
	return c.bot(token).setWebhook(ctx, c.hookURL(conn), WebhookSecret(token, connKey(conn)))
}

// Deprovision removes the webhook of the bot, when it is this application's: one another service has
// set since is left as it is.
func (c *Connector) Deprovision(ctx context.Context, conn connectors.Connection, creds connectors.Credentials) error {
	b := c.bot(creds["token"])
	info, err := b.webhookInfo(ctx)
	if err != nil {
		return err
	}
	if c.cfg.PublicOrigin == "" || !strings.HasPrefix(info.Url, c.hookPrefix()) {
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

// StartPlatform sets the platform bot's webhook at /automations/hooks/platform/telegram_bot.
func (c *Connector) StartPlatform(ctx context.Context) error {
	if c.cfg.PlatformToken == "" || c.cfg.PublicOrigin == "" {
		return nil
	}
	return c.Provision(ctx, connectors.Connection{Kind: Kind, Name: "platform"}, connectors.Credentials{"token": c.cfg.PlatformToken})
}
