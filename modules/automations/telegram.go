package automations

import (
	"context"
	"errors"
	"log/slog"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/iota-uz/iota-sdk/modules/automations/infrastructure/uploads"
	coreservices "github.com/iota-uz/iota-sdk/modules/core/services"
	"github.com/iota-uz/iota-sdk/pkg/automation/catalog"
	"github.com/iota-uz/iota-sdk/pkg/automation/connectors"
	"github.com/iota-uz/iota-sdk/pkg/automation/connectors/telegram"
	"github.com/iota-uz/iota-sdk/pkg/automation/engine"
	"github.com/iota-uz/iota-sdk/pkg/composition"
)

// WithoutTelegram leaves out the Telegram bot connector (telegram.New) the component registers when the
// application registers no connector of kind telegram_bot itself.
func WithoutTelegram() Option {
	return func(c *component) { c.noTelegram = true }
}

// newTelegram is the SDK's Telegram bot connector over the composition: the platform bot is APP_TELEGRAMBOTTOKEN
// (a platform.credentials entry of telegram_bot wins), the webhooks point to the configured origin, updates go to
// the engine's inbound, and the files of chats are kept privately in the uploads (served by
// /automations/attachments/{id}). What the connector needs is resolved when it is first used: the directory is
// built before the engine and the store.
func (c *component) newTelegram(container *composition.Container, cfg settingsCfg) *telegram.Connector {
	tc := telegram.Config{
		PlatformToken: cfg.telegramToken,
		PublicOrigin:  cfg.origin,
		Inbound:       lazyInbound{c: c, container: container},
		Connections:   lazyConnectionState{container: container},
		Usage:         lazyUsage{container: container},
		Logger:        slog.Default(),
	}
	if composition.Has[*coreservices.UploadService](container) {
		tc.Uploads = lazyTelegramUploads{container: container, privateRoot: cfg.uploads.PrivatePath()}
	}
	return telegram.New(tc)
}

// errNoEngine is the inbound of a composition that runs no automation engine.
var errNoEngine = errors.New("automations: the composition runs no automation engine")

// lazyInbound is the engine's Inbound, resolved when an update first arrives.
type lazyInbound struct {
	c         *component
	container *composition.Container
}

func (l lazyInbound) get() (engine.Inbound, error) {
	rt, err := l.c.runtime(l.container)
	if err != nil {
		return nil, err
	}
	if rt == nil {
		return nil, errNoEngine
	}
	return rt.Inbound(), nil
}

func (l lazyInbound) Publish(ctx context.Context, tx pgx.Tx, ev engine.InboundEvent) (bool, error) {
	in, err := l.get()
	if err != nil {
		return false, err
	}
	return in.Publish(ctx, tx, ev)
}

func (l lazyInbound) Correlation(ctx context.Context, token string) (uuid.UUID, bool, error) {
	in, err := l.get()
	if err != nil {
		return uuid.Nil, false, err
	}
	return in.Correlation(ctx, token)
}

// lazyUsage records the messages the connector sends outside a node in automation.connector_usage.
type lazyUsage struct{ container *composition.Container }

func (l lazyUsage) Record(ctx context.Context, e catalog.UsageEntry) error {
	db, err := composition.Resolve[*pgxpool.Pool](l.container)
	if err != nil {
		return err
	}
	return connectors.NewUsageRecorder(db).Record(ctx, e)
}

// lazyTelegramUploads is uploads.NewTelegramUploads over the core's upload service.
type lazyTelegramUploads struct {
	container   *composition.Container
	privateRoot string
}

func (l lazyTelegramUploads) get() (telegram.Uploads, error) {
	svc, err := composition.Resolve[*coreservices.UploadService](l.container)
	if err != nil {
		return nil, err
	}
	db, err := composition.Resolve[*pgxpool.Pool](l.container)
	if err != nil {
		return nil, err
	}
	return uploads.NewTelegramUploads(svc, db, l.privateRoot), nil
}

func (l lazyTelegramUploads) Save(ctx context.Context, tenantID uuid.UUID, f telegram.UploadFile) (telegram.Stored, error) {
	u, err := l.get()
	if err != nil {
		return telegram.Stored{}, err
	}
	return u.Save(ctx, tenantID, f)
}

func (l lazyTelegramUploads) Open(ctx context.Context, tenantID uuid.UUID, id string) (telegram.UploadFile, error) {
	u, err := l.get()
	if err != nil {
		return telegram.UploadFile{}, err
	}
	return u.Open(ctx, tenantID, id)
}

// connectionOptions are the choices the connector of a tenant's connection offers for a param field
// (connectors.OptionSource): the chats a Telegram bot knows, for the editor's picker.
type connectionOptions struct {
	store connectors.ConnectionStore
	dir   connectors.Directory
}

func (o connectionOptions) Options(ctx context.Context, tenantID, connectionID uuid.UUID, source, query string) ([]connectors.Option, error) {
	conn, _, err := o.store.Get(ctx, tenantID, connectionID)
	if err != nil {
		return nil, err
	}
	c, ok := o.dir.Get(conn.Kind)
	if !ok {
		return []connectors.Option{}, nil
	}
	src, ok := c.(connectors.OptionSource)
	if !ok {
		return []connectors.Option{}, nil
	}
	return src.Options(ctx, tenantID, connectionID, source, query)
}
