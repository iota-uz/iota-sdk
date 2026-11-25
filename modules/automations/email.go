package automations

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/iota-uz/iota-sdk/modules/automations/services"
	coreservices "github.com/iota-uz/iota-sdk/modules/core/services"
	"github.com/iota-uz/iota-sdk/pkg/automation/catalog"
	"github.com/iota-uz/iota-sdk/pkg/automation/connectors"
	"github.com/iota-uz/iota-sdk/pkg/automation/connectors/email"
	"github.com/iota-uz/iota-sdk/pkg/composition"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/smtpconfig"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/uploadsconfig"
)

// WithoutEmail leaves out the email connectors (smtp and imap, email.New) the component registers when the
// application registers none of those kinds.
func WithoutEmail() Option {
	return func(c *component) { c.noEmail = true }
}

// emailConnectors are the SDK's email connectors wired to the composition: the application's SMTP_* as the
// platform credential of kind smtp, the OAuth2 helper for the Google and Microsoft 365 sign-ins, the connection
// store (the two connections of a sign-in, cursors seeded before a send) and the SDK's uploads for
// attachments. The store and the OAuth2 helper are built after the directory that holds these connectors, so
// they are resolved on first use.
func emailConnectors(container *composition.Container, smtpCfg smtpconfig.Config, uploadsCfg uploadsconfig.Config) []connectors.Connector {
	cfg := email.Config{
		OAuth:       lazyOAuth2{container: container},
		Connections: lazyConnectionState{container: container},
		Logger:      slog.Default(),
	}
	if smtpCfg.IsConfigured() {
		cfg.Platform = &smtpCfg
	}
	if composition.Has[*coreservices.UploadService](container) && composition.Has[*pgxpool.Pool](container) {
		cfg.Uploads = lazyEmailUploads{container: container, privateRoot: uploadsCfg.PrivatePath()}
	}
	return email.New(cfg)
}

// lazyOAuth2 is the composition's connectors.OAuth2, resolved when a token is first asked for.
type lazyOAuth2 struct{ container *composition.Container }

func (l lazyOAuth2) resolve() (connectors.OAuth2, error) {
	return composition.Resolve[connectors.OAuth2](l.container)
}

func (l lazyOAuth2) Begin(ctx context.Context, tenantID uuid.UUID, kind catalog.ConnectorKind, redirectURL string) (string, string, error) {
	o, err := l.resolve()
	if err != nil {
		return "", "", err
	}
	return o.Begin(ctx, tenantID, kind, redirectURL)
}

func (l lazyOAuth2) Complete(ctx context.Context, tenantID uuid.UUID, state, code string) (connectors.Connection, error) {
	o, err := l.resolve()
	if err != nil {
		return connectors.Connection{}, err
	}
	return o.Complete(ctx, tenantID, state, code)
}

func (l lazyOAuth2) Exchange(ctx context.Context, tenantID uuid.UUID, state, code string) (connectors.OAuth2Grant, error) {
	o, err := l.resolve()
	if err != nil {
		return connectors.OAuth2Grant{}, err
	}
	return o.Exchange(ctx, tenantID, state, code)
}

func (l lazyOAuth2) Token(ctx context.Context, tenantID, connectionID uuid.UUID) (string, error) {
	o, err := l.resolve()
	if err != nil {
		return "", err
	}
	return o.Token(ctx, tenantID, connectionID)
}

// lazyConnectionState is the composition's connection store as email.ConnectionState.
type lazyConnectionState struct{ container *composition.Container }

func (l lazyConnectionState) resolve() (connectors.ConnectionStore, error) {
	return composition.Resolve[connectors.ConnectionStore](l.container)
}

func (l lazyConnectionState) List(ctx context.Context, tenantID uuid.UUID, kind catalog.ConnectorKind) ([]connectors.Connection, error) {
	st, err := l.resolve()
	if err != nil {
		return nil, err
	}
	return st.List(ctx, tenantID, kind)
}

func (l lazyConnectionState) Get(ctx context.Context, tenantID, id uuid.UUID) (connectors.Connection, connectors.Credentials, error) {
	st, err := l.resolve()
	if err != nil {
		return connectors.Connection{}, nil, err
	}
	return st.Get(ctx, tenantID, id)
}

func (l lazyConnectionState) SetStatusIf(ctx context.Context, tenantID, id uuid.UUID, s connectors.ConnectionStatus, reason string, cond func(state json.RawMessage, version int) bool) (bool, error) {
	st, err := l.resolve()
	if err != nil {
		return false, err
	}
	return st.SetStatusIf(ctx, tenantID, id, s, reason, cond)
}

func (l lazyConnectionState) UpdateInbound(ctx context.Context, tenantID, id uuid.UUID, fn func(state json.RawMessage) (json.RawMessage, error)) error {
	st, err := l.resolve()
	if err != nil {
		return err
	}
	return st.UpdateInbound(ctx, tenantID, id, fn)
}

// lazyEmailUploads stores the attachments of incoming email through the core module's upload service
// (services.NewEmailUploads), resolved on the first attachment.
type lazyEmailUploads struct {
	container   *composition.Container
	privateRoot string
}

func (l lazyEmailUploads) Save(ctx context.Context, tenantID uuid.UUID, f email.UploadFile) (email.Stored, error) {
	svc, err := composition.Resolve[*coreservices.UploadService](l.container)
	if err != nil {
		return email.Stored{}, err
	}
	pool, err := composition.Resolve[*pgxpool.Pool](l.container)
	if err != nil {
		return email.Stored{}, err
	}
	return services.NewEmailUploads(svc, pool, l.privateRoot).Save(ctx, tenantID, f)
}

var (
	_ connectors.OAuth2     = lazyOAuth2{}
	_ email.ConnectionState = lazyConnectionState{}
	_ email.Uploads         = lazyEmailUploads{}
)
