// Package email is the email connector of automation graphs: it sends a message over SMTP, waits for the
// reply to it, starts graphs on a message that arrives in a mailbox, and marks or moves messages over IMAP.
//
// It has two connector kinds in one package: smtp (sending) and imap (receiving, waiting for a reply and
// marking). A tenant connects either with a login and a password (an app password) for any server, or
// with an OAuth2 sign-in for Gmail and Microsoft 365: the same SMTP and IMAP protocols, authenticated with
// SASL XOAUTH2. One sign-in creates both connections; they share a login id and when the provider refuses
// the tokens both go to needs_reauth. Microsoft 365 is OAuth only: Exchange Online is switching off basic
// SMTP AUTH.
//
// The application's own SMTP settings (SMTP_*) are offered to every tenant as the platform credential of
// the smtp kind (policy auto); they send, they never receive.
//
// Incoming mail is polled over IMAP (engine.Poller, 60 s, never IDLE). The cursor is
// {mailbox, uidvalidity, last_uid}: the first poll of a connection, and the first after UIDVALIDITY
// changed, sets it to the mailbox's last UID and publishes nothing, so old mail never starts a graph. A
// reply is found by its headers: the event email.received.v1 has the subject conversation
// "<imap connection id>:<thread root Message-ID>", the thread root being the first of References, else
// In-Reply-To; email.wait_reply waits on the key of the message it sent. No marker is put in a subject or
// an address.
//
// The connector never logs a login, a password or a token: the dialogue with a server is not written
// anywhere unless Config.Debug is set, and then every AUTH exchange and every credential value is masked.
// The HTML of an incoming message is not passed on, only its text; links and attachments are not opened.
package email

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/iota-uz/iota-sdk/pkg/automation/catalog"
	"github.com/iota-uz/iota-sdk/pkg/automation/connectors"
	"github.com/iota-uz/iota-sdk/pkg/automation/engine"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/smtpconfig"
)

// Kinds of the two connectors.
const (
	KindSMTP catalog.ConnectorKind = "smtp"
	KindIMAP catalog.ConnectorKind = "imap"
)

// Names of the platform credentials that hold the OAuth2 client of a provider.
const (
	GoogleClient    = "google_oauth"
	MicrosoftClient = "microsoft_oauth"
)

// Config configures the connectors.
type Config struct {
	// Platform is the application's own SMTP (SMTP_*): the platform credential of kind smtp, offered to
	// every tenant (policy auto). Nil, or without a host: none.
	Platform *smtpconfig.Config
	// Inbound is the engine's inbound; the connector publishes through engine.Poller, so it only keeps it.
	Inbound engine.Inbound
	// OAuth keeps the tokens of a sign-in fresh (OAuth2.Token); nil: sign-in presets do not work.
	OAuth connectors.OAuth2
	// Dialer opens every connection (a test points it at its fake server); nil: a dialer with a 20 s timeout.
	Dialer *net.Dialer

	// Providers overrides what the sign-in presets (gmail, m365) point at, by preset name: a sovereign
	// cloud of Microsoft, a test's servers. A field left empty keeps the default.
	Providers map[string]Provider

	// Uploads stores the attachments of incoming messages; without it an attachment is listed with
	// error "no_storage".
	Uploads Uploads
	// Connections is the store the connector marks the other connection of a sign-in with when the
	// provider refuses its tokens (needs_reauth), and seeds the cursor of the mailbox a reply to a message
	// it sends will come to; without it only the refusing connection is marked and the first poll of a
	// mailbox no graph has used yet may jump over an early reply.
	Connections ConnectionState
	// Usage records a message sent outside a node; nodes use the one their call carries.
	Usage catalog.UsageRecorder
	// TLSConfig is the base TLS configuration (tests trust their own certificate); nil: the system roots.
	TLSConfig *tls.Config
	// Debug, when set, gets the dialogue with the servers with every AUTH exchange and credential masked.
	Debug io.Writer
	// Clock is the connector's time (tests); default time.Now.
	Clock  func() time.Time
	Logger *slog.Logger
}

// Provider is what a sign-in preset points at.
type Provider struct {
	// AuthURL, TokenURL and Scopes are the OAuth2 provider's.
	AuthURL, TokenURL string
	Scopes            []string
	// SMTPHost, SMTPPort, IMAPHost and IMAPPort are the servers; Security is tls, starttls or auto (by port).
	SMTPHost string
	SMTPPort int
	IMAPHost string
	IMAPPort int
	Security string
}

// ConnectionState is the part of connectors.ConnectionStore the connector uses to link the two connections
// of one sign-in.
type ConnectionState interface {
	List(ctx context.Context, tenantID uuid.UUID, kind catalog.ConnectorKind) ([]connectors.Connection, error)
	Get(ctx context.Context, tenantID, id uuid.UUID) (connectors.Connection, connectors.Credentials, error)
	SetStatus(ctx context.Context, tenantID, id uuid.UUID, s connectors.ConnectionStatus, reason string) error
	UpdateInbound(ctx context.Context, tenantID, id uuid.UUID, fn func(state json.RawMessage) (json.RawMessage, error)) error
}

// New is the smtp connector and the imap connector (which is also an engine.Poller).
func New(cfg Config) []connectors.Connector {
	m := &hub{cfg: cfg}
	return []connectors.Connector{&smtpConnector{m}, &imapConnector{m}}
}

// hub is what the two connectors share.
type hub struct{ cfg Config }

func (h *hub) now() time.Time {
	if h.cfg.Clock != nil {
		return h.cfg.Clock()
	}
	return time.Now()
}

func (h *hub) log() *slog.Logger {
	if h.cfg.Logger != nil {
		return h.cfg.Logger
	}
	return slog.Default()
}

func (h *hub) dialer() *net.Dialer {
	if h.cfg.Dialer != nil {
		return h.cfg.Dialer
	}
	return &net.Dialer{Timeout: 20 * time.Second}
}

func (h *hub) tlsConfig(host string) *tls.Config {
	var c *tls.Config
	if h.cfg.TLSConfig != nil {
		c = h.cfg.TLSConfig.Clone()
	} else {
		c = &tls.Config{}
	}
	if c.ServerName == "" {
		c.ServerName = host
	}
	if c.MinVersion < tls.VersionTLS12 {
		c.MinVersion = tls.VersionTLS12
	}
	return c
}

// ---------------------------------------------------------------------------------------------
// credentials schemas

// Presets of a connection: how the mailbox is reached.
const (
	PresetCustom = "custom"
	PresetGmail  = "gmail"
	PresetM365   = "m365"
)

// oauthFields are the credentials a sign-in keeps beside the account: the token set, the mailbox the
// tokens are for and the id that links the two connections of one sign-in.
const oauthFields = `
 "access_token":{"type":"string","x-secret":true,"maxLength":8192},
 "refresh_token":{"type":"string","x-secret":true,"maxLength":8192},
 "token_type":{"type":"string","maxLength":40},
 "expires_at":{"type":"string","maxLength":40},
 "login_id":{"type":"string","maxLength":64},
 "token_from":{"type":"string","maxLength":64}`

const presetField = `"preset":{"type":"string","enum":["custom","gmail","m365"],"default":"custom",
  "x-label":{"en":"Account","ru":"Аккаунт","uz":"Akkaunt"},
  "x-options":{"custom":{"en":"Any server (login and password)","ru":"Любой сервер (логин и пароль)","uz":"Istalgan server (login va parol)"},
               "gmail":{"en":"Gmail (sign in with Google)","ru":"Gmail (вход через Google)","uz":"Gmail (Google orqali kirish)"},
               "m365":{"en":"Microsoft 365 (sign in with Microsoft)","ru":"Microsoft 365 (вход через Microsoft)","uz":"Microsoft 365 (Microsoft orqali kirish)"}},
  "x-oauth-client":{"gmail":"` + GoogleClient + `","m365":"` + MicrosoftClient + `"},
  "description":"gmail and m365 are signed in with OAuth2 and offered only when the application has the OAuth2 client of the provider; Microsoft 365 has no password option."}`

const passwordField = `"password":{"type":"string","x-secret":true,"maxLength":1000,
  "x-label":{"en":"Password (an app password)","ru":"Пароль (пароль приложения)","uz":"Parol (ilova paroli)"}}`

// customWhen is the condition under which the fields of a login and a password are required.
const customWhen = `"if":{"anyOf":[{"not":{"required":["preset"]}},{"properties":{"preset":{"const":"custom"}}}]}`

const smtpSchema = `{"type":"object","additionalProperties":false,"properties":{` + presetField + `,
 "host":{"type":"string","maxLength":253,"x-label":{"en":"SMTP server","ru":"SMTP-сервер","uz":"SMTP server"}},
 "port":{"type":"integer","minimum":1,"maximum":65535,"default":587,
   "x-label":{"en":"Port (465 TLS, 587 STARTTLS)","ru":"Порт (465 TLS, 587 STARTTLS)","uz":"Port (465 TLS, 587 STARTTLS)"}},
 "security":{"type":"string","enum":["auto","tls","starttls"],"default":"auto","x-label":{"en":"Connection security","ru":"Защита соединения","uz":"Ulanish xavfsizligi"},
   "description":"auto: TLS on port 465 (SMTP) or 993 (IMAP), STARTTLS elsewhere. The connection is always encrypted."},
 "username":{"type":"string","maxLength":254,"x-label":{"en":"Login","ru":"Логин","uz":"Login"}},
 ` + passwordField + `,
 "from_address":{"type":"string","maxLength":254,"x-label":{"en":"Sender address","ru":"Адрес отправителя","uz":"Yuboruvchi manzili"}},
 "from_name":{"type":"string","maxLength":200,"x-label":{"en":"Sender name","ru":"Имя отправителя","uz":"Yuboruvchi nomi"}},` + oauthFields + `},
 "allOf":[{` + customWhen + `,"then":{"required":["host","port","from_address"]}}]}`

const imapSchema = `{"type":"object","additionalProperties":false,"properties":{` + presetField + `,
 "host":{"type":"string","maxLength":253,"x-label":{"en":"IMAP server","ru":"IMAP-сервер","uz":"IMAP server"}},
 "port":{"type":"integer","minimum":1,"maximum":65535,"default":993,
   "x-label":{"en":"Port (993 TLS)","ru":"Порт (993 TLS)","uz":"Port (993 TLS)"}},
 "security":{"type":"string","enum":["auto","tls","starttls"],"default":"auto","x-label":{"en":"Connection security","ru":"Защита соединения","uz":"Ulanish xavfsizligi"},
   "description":"auto: TLS on port 993, STARTTLS elsewhere. The connection is always encrypted."},
 "username":{"type":"string","maxLength":254,"x-label":{"en":"Login","ru":"Логин","uz":"Login"}},
 ` + passwordField + `,
 "mailbox":{"type":"string","maxLength":200,"default":"INBOX","x-label":{"en":"Mailbox","ru":"Папка","uz":"Papka"}},` + oauthFields + `},
 "allOf":[{` + customWhen + `,"then":{"required":["host","port","username","password"]}}]}`

// oauthPresets are the sign-in providers of both kinds. The scopes are the ones the issue names plus the
// OpenID ones that tell which mailbox the tokens are for.
func (h *hub) oauthPresets() map[string]connectors.OAuth2Config {
	out := defaultPresets()
	for name, p := range h.cfg.Providers {
		d, ok := out[name]
		if !ok {
			continue
		}
		if p.AuthURL != "" {
			d.AuthURL = p.AuthURL
		}
		if p.TokenURL != "" {
			d.TokenURL = p.TokenURL
		}
		if len(p.Scopes) > 0 {
			d.Scopes = p.Scopes
		}
		out[name] = d
	}
	return out
}

func defaultPresets() map[string]connectors.OAuth2Config {
	return map[string]connectors.OAuth2Config{
		PresetGmail: {
			AuthURL: "https://accounts.google.com/o/oauth2/v2/auth", TokenURL: "https://oauth2.googleapis.com/token",
			Scopes:           []string{"https://mail.google.com/", "openid", "email"},
			ClientCredential: GoogleClient,
			// A refresh token is issued on the first consent only; without the prompt a second sign-in
			// of an account would come back without one.
			AuthParams: map[string]string{"prompt": "consent"},
		},
		PresetM365: {
			AuthURL: "https://login.microsoftonline.com/common/oauth2/v2.0/authorize", TokenURL: "https://login.microsoftonline.com/common/oauth2/v2.0/token",
			Scopes:           []string{"https://outlook.office.com/SMTP.Send", "https://outlook.office.com/IMAP.AccessAsUser.All", "offline_access", "openid", "email", "profile"},
			ClientCredential: MicrosoftClient,
		},
	}
}

// ---------------------------------------------------------------------------------------------
// the smtp connector

type smtpConnector struct{ *hub }

var (
	_ connectors.Connector      = (*smtpConnector)(nil)
	_ connectors.OAuth2Finisher = (*smtpConnector)(nil)
)

func (c *smtpConnector) Kind() catalog.ConnectorKind { return KindSMTP }

func (c *smtpConnector) Descriptor() connectors.Descriptor {
	return connectors.Descriptor{
		Kind:  KindSMTP,
		Title: map[string]string{"en": "Email (SMTP)", "ru": "Почта (SMTP)", "uz": "Pochta (SMTP)"},
		Description: map[string]string{
			"en": "Send email through any SMTP server, Gmail or Microsoft 365.",
			"ru": "Отправка писем через любой SMTP-сервер, Gmail или Microsoft 365.",
		},
		Category:      connectors.CategoryEmail,
		Icon:          "mail",
		Auth:          connectors.AuthCustom,
		Credentials:   json.RawMessage(smtpSchema),
		OAuth2Presets: c.oauthPresets(),
		Platform:      connectors.PlatformAuto,
	}
}

func (c *smtpConnector) Nodes() []catalog.NodeType {
	return []catalog.NodeType{&sendNode{c.hub}}
}

func (c *smtpConnector) Triggers() []catalog.TriggerSource { return nil }

// PlatformCredential is the application's SMTP as the platform credential of kind smtp.
func (c *smtpConnector) PlatformCredential() (connectors.PlatformCredential, bool) {
	return PlatformCredential(c.cfg.Platform)
}

// PlatformCredential is the platform credential of kind smtp for the application's SMTP settings.
func PlatformCredential(cfg *smtpconfig.Config) (connectors.PlatformCredential, bool) {
	if cfg == nil || !cfg.IsConfigured() {
		return connectors.PlatformCredential{}, false
	}
	port := cfg.Port
	if port == 0 {
		port = 587
	}
	creds := connectors.Credentials{"preset": PresetCustom, "host": cfg.Host, "port": itoa(port), "from_address": cfg.From}
	if cfg.Username != "" {
		creds["username"] = cfg.Username
	}
	if cfg.Password != "" {
		creds["password"] = cfg.Password
	}
	id := connectors.Identity{}
	if addr := mailboxAddress(cfg.From); addr != "" {
		id = connectors.Identity{Display: addr, External: strings.ToLower(addr), Kind: connectors.IdentityEmail}
	}
	return connectors.PlatformCredential{Credentials: creds, Identity: id}, true
}

// FinishGrant keeps the mailbox the tokens are for in the credentials and brings the grant of the imap
// connection of the same sign-in.
func (c *smtpConnector) FinishGrant(g connectors.OAuth2Grant) (connectors.OAuth2Grant, error) {
	return finishGrant(g, KindSMTP, KindIMAP)
}

// Test connects, secures the connection and authenticates; it sends no message.
func (c *smtpConnector) Test(ctx context.Context, creds connectors.Credentials) (connectors.Identity, error) {
	p, err := c.parseSMTP(creds)
	if err != nil {
		return connectors.Identity{}, err
	}
	token := ""
	if p.oauth() {
		if token, err = tokenOf(creds); err != nil {
			return connectors.Identity{}, err
		}
	}
	cl, err := c.dialSMTP(ctx, p, token)
	if err != nil {
		return connectors.Identity{}, err
	}
	_ = cl.Quit()
	_ = cl.Close()
	return identityOf(p.fromAddress, p.username), nil
}

// ---------------------------------------------------------------------------------------------
// the imap connector

type imapConnector struct{ *hub }

var (
	_ connectors.Connector      = (*imapConnector)(nil)
	_ connectors.OAuth2Finisher = (*imapConnector)(nil)
	_ engine.Poller             = (*imapConnector)(nil)
)

func (c *imapConnector) Kind() catalog.ConnectorKind { return KindIMAP }

func (c *imapConnector) Descriptor() connectors.Descriptor {
	return connectors.Descriptor{
		Kind:  KindIMAP,
		Title: map[string]string{"en": "Email (IMAP)", "ru": "Почта (IMAP)", "uz": "Pochta (IMAP)"},
		Description: map[string]string{
			"en": "Receive email, wait for a reply to a message and mark or move messages in a mailbox.",
			"ru": "Входящие письма, ожидание ответа на письмо, пометки и перенос писем в ящике.",
		},
		Category:      connectors.CategoryEmail,
		Icon:          "mail",
		Auth:          connectors.AuthCustom,
		Credentials:   json.RawMessage(imapSchema),
		OAuth2Presets: c.oauthPresets(),
		Platform:      connectors.PlatformNone,
		Inbound:       true,
	}
}

func (c *imapConnector) Nodes() []catalog.NodeType {
	return []catalog.NodeType{newWaitReplyNode(c.hub), &labelNode{c.hub}}
}

func (c *imapConnector) Triggers() []catalog.TriggerSource {
	return []catalog.TriggerSource{triggers{}}
}

func (c *imapConnector) FinishGrant(g connectors.OAuth2Grant) (connectors.OAuth2Grant, error) {
	return finishGrant(g, KindIMAP, KindSMTP)
}

// Test authenticates and selects the mailbox (read-only).
func (c *imapConnector) Test(ctx context.Context, creds connectors.Credentials) (connectors.Identity, error) {
	p, err := c.parseIMAP(creds)
	if err != nil {
		return connectors.Identity{}, err
	}
	token := ""
	if p.oauth() {
		if token, err = tokenOf(creds); err != nil {
			return connectors.Identity{}, err
		}
	}
	s, err := c.openIMAP(ctx, p, token)
	if err != nil {
		return connectors.Identity{}, err
	}
	defer s.close()
	if _, err := s.selectMailbox(p.mailbox, true); err != nil {
		return connectors.Identity{}, err
	}
	return identityOf(p.username), nil
}

// ---------------------------------------------------------------------------------------------
// the sign-in of both kinds

// finishGrant puts the mailbox the token is for into the credentials of the grant and makes the grant of
// the other kind with the same tokens.
func finishGrant(g connectors.OAuth2Grant, own, other catalog.ConnectorKind) (connectors.OAuth2Grant, error) {
	if g.Credentials[connectors.OAuth2Preset] == "" {
		return g, nil // a sign-in without a preset has nothing to add
	}
	addr, err := addressOfIDToken(g.Credentials[connectors.OAuth2IDToken])
	if err != nil {
		return g, err
	}
	login := uuid.NewString()
	g.Credentials = shared(g.Credentials, addr, login)
	if own == KindSMTP {
		g.Credentials["from_address"] = addr
	}
	g.Identity = connectors.Identity{Display: addr, External: strings.ToLower(addr), Kind: connectors.IdentityEmail}
	companion := connectors.OAuth2Grant{Kind: other, Identity: g.Identity, Credentials: shared(g.Credentials, addr, login)}
	delete(companion.Credentials, "from_address")
	if other == KindSMTP {
		companion.Credentials["from_address"] = addr
	}
	g.Companions = append(g.Companions, companion)
	return g, nil
}

// shared is a copy of the credentials with the mailbox and the login id set.
func shared(src connectors.Credentials, addr, login string) connectors.Credentials {
	out := make(connectors.Credentials, len(src)+2)
	for k, v := range src {
		out[k] = v
	}
	out["username"], out["login_id"] = addr, login
	return out
}

var errNoMailbox = errors.New("email: the sign-in does not tell which mailbox it is for")
