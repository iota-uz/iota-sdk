package automations

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/iota-uz/iota-sdk/modules/core/notifications"
	"github.com/iota-uz/iota-sdk/modules/core/notifications/telegramchannel"
	"github.com/iota-uz/iota-sdk/pkg/automation/catalog"
	"github.com/iota-uz/iota-sdk/pkg/automation/connectors"
	"github.com/iota-uz/iota-sdk/pkg/automation/connectors/telegram"
	"github.com/iota-uz/iota-sdk/pkg/automation/engine"
	"github.com/iota-uz/iota-sdk/pkg/composition"
)

// WithTelegram registers the Telegram bot connector built from cfg. What cfg leaves empty the module fills
// in: the engine's inbound (Inbound), the module's connection store (Connections), the usage recorder
// (Usage) and the configured origin (PublicOrigin). With the core notification stack composed, the
// connector also becomes the notification channel "telegram" and completes the users' links (/start u_…).
func WithTelegram(cfg telegram.Config) Option {
	return func(c *component) { c.telegram = &cfg }
}

// newTelegram is the connector of WithTelegram over the container.
func (c *component) newTelegram(container *composition.Container, origin string) *telegram.Connector {
	cfg := *c.telegram
	if cfg.Inbound == nil {
		cfg.Inbound = lazyInbound{c: c, container: container}
	}
	if cfg.Connections == nil {
		cfg.Connections = lazyTelegramStore{container: container}
	}
	if cfg.Usage == nil {
		cfg.Usage = lazyUsage{container: container}
	}
	if strings.TrimSpace(cfg.PublicOrigin) == "" {
		cfg.PublicOrigin = origin
	}
	return telegram.New(cfg)
}

// telegramConnector is the Telegram connector of the directory, when one is registered.
func telegramConnector(container *composition.Container) (*telegram.Connector, bool) {
	dir, err := composition.Resolve[connectors.Directory](container)
	if err != nil {
		return nil, false
	}
	conn, ok := dir.Get(telegram.Kind)
	if !ok {
		return nil, false
	}
	tg, ok := conn.(*telegram.Connector)
	return tg, ok
}

// contributeTelegramChannel makes a registered Telegram connector the notification channel "telegram" of
// the core module and registers the /start handler that links a user's chat (#1349).
func (c *component) contributeTelegramChannel(b *composition.Builder, origin string) {
	composition.Provide[notifications.ConnectorConnections](b, func(container *composition.Container) (notifications.ConnectorConnections, error) {
		return connectorConnections{container: container}, nil
	})
	notifications.ContributeChannels(b, func(container *composition.Container) ([]notifications.ChannelContribution, error) {
		tg, ok := telegramConnector(container)
		if !ok || !composition.Has[notifications.ChannelConnections](container) {
			return nil, nil
		}
		creds, err := composition.Resolve[connectors.CredentialSource](container)
		if err != nil {
			return nil, err
		}
		conns, err := composition.Resolve[notifications.ChannelConnections](container)
		if err != nil {
			return nil, err
		}
		channel := telegramchannel.New(telegramchannel.Config{Client: tg, Bots: tg, Credentials: creds, Connections: conns, PublicOrigin: origin})
		return []notifications.ChannelContribution{{Spec: channel.Spec(), Adapter: channel}}, nil
	})
	composition.ContributeHooks(b, func(container *composition.Container) ([]composition.Hook, error) {
		return []composition.Hook{{Name: "automations-telegram-links", Start: func(context.Context) (composition.StopFn, error) {
			tg, ok := telegramConnector(container)
			if !ok || !composition.Has[notifications.ChannelLinkService](container) {
				return nil, nil //nolint:nilnil // a hook with nothing to stop
			}
			links, err := composition.Resolve[notifications.ChannelLinkService](container)
			if err != nil {
				return nil, err
			}
			return nil, tg.RegisterStart(telegramchannel.NewTelegramStartHandler(links))
		}}}, nil
	})
}

// connectorConnections lists a tenant's working connections of a kind for the notification channels: its
// own connections (by their names), then the platform credential when the tenant may use it.
type connectorConnections struct{ container *composition.Container }

func (l connectorConnections) Connections(ctx context.Context, tenantID uuid.UUID, kind string) ([]notifications.ConnectorConnection, error) {
	store, err := composition.Resolve[connectors.ConnectionStore](l.container)
	if err != nil {
		return nil, err
	}
	connected, err := store.Connected(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	refs := connected[catalog.ConnectorKind(kind)]
	if len(refs) == 0 {
		return nil, nil
	}
	names := map[string]string{}
	if list, err := store.List(ctx, tenantID, catalog.ConnectorKind(kind)); err == nil {
		for _, conn := range list {
			names[conn.ID.String()] = connectionName(conn)
		}
	}
	var platformName string
	if pc, err := composition.Resolve[connectors.PlatformCredentials](l.container); err == nil {
		if creds, identity, ok := pc.Platform(catalog.ConnectorKind(kind)); ok {
			platformName = identity.Display
			if platformName == "" {
				platformName = l.platformAccount(ctx, catalog.ConnectorKind(kind), creds)
			}
		}
	}
	out := make([]notifications.ConnectorConnection, 0, len(refs))
	for _, ref := range refs {
		if strings.HasPrefix(ref.ID, catalog.PlatformPrefix) {
			out = append(out, notifications.ConnectorConnection{ID: ref.ID, Name: platformName, Platform: true})
			continue
		}
		name := names[ref.ID]
		if name == "" {
			name = ref.Name
		}
		out = append(out, notifications.ConnectorConnection{ID: ref.ID, Name: name})
	}
	return out, nil
}

// platformNames keeps the account name of a platform credential by kind and credential fingerprint.
var platformNames sync.Map

// platformAccount asks the connector who the platform credential is (a bot's @username), once per credential.
func (l connectorConnections) platformAccount(ctx context.Context, kind catalog.ConnectorKind, creds connectors.Credentials) string {
	dir, err := composition.Resolve[connectors.Directory](l.container)
	if err != nil {
		return ""
	}
	conn, ok := dir.Get(kind)
	if !ok {
		return ""
	}
	key := string(kind)
	if ext, ok := conn.(interface {
		ExternalID(connectors.Credentials) string
	}); ok {
		key += "|" + ext.ExternalID(creds)
	}
	if v, ok := platformNames.Load(key); ok {
		return v.(string)
	}
	tctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	identity, err := conn.Test(tctx, creds)
	if err != nil {
		return ""
	}
	platformNames.Store(key, identity.Display)
	return identity.Display
}

// connectionName names a connection for people: its display name, the account, its slug.
func connectionName(c connectors.Connection) string {
	for _, v := range []string{c.DisplayName, c.Identity.Display, c.Name} {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return c.ID.String()
}

// errNoEngine is the inbound's answer while the composition runs no engine.
var errNoEngine = errors.New("automations: the composition runs no automation engine")

// lazyInbound is the engine's inbound, resolved on the first event (the engine is built after the directory).
type lazyInbound struct {
	c         *component
	container *composition.Container
}

func (l lazyInbound) inbound() (engine.Inbound, error) {
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
	in, err := l.inbound()
	if err != nil {
		return false, err
	}
	return in.Publish(ctx, tx, ev)
}

func (l lazyInbound) Correlation(ctx context.Context, token string) (uuid.UUID, bool, error) {
	in, err := l.inbound()
	if err != nil {
		return uuid.Nil, false, err
	}
	return in.Correlation(ctx, token)
}

// lazyTelegramStore is the module's connection store for the Telegram connector, resolved per call (the store
// is built after the directory). It forwards the optional parts of the built-in store the connector uses.
type lazyTelegramStore struct{ container *composition.Container }

func (l lazyTelegramStore) store() (connectors.ConnectionStore, error) {
	return composition.Resolve[connectors.ConnectionStore](l.container)
}

func (l lazyTelegramStore) Get(ctx context.Context, tenantID, id uuid.UUID) (connectors.Connection, connectors.Credentials, error) {
	s, err := l.store()
	if err != nil {
		return connectors.Connection{}, nil, err
	}
	return s.Get(ctx, tenantID, id)
}

func (l lazyTelegramStore) SetStatusIf(ctx context.Context, tenantID, id uuid.UUID, st connectors.ConnectionStatus, reason string, cond func(state json.RawMessage, version int) bool) (bool, error) {
	s, err := l.store()
	if err != nil {
		return false, err
	}
	return s.SetStatusIf(ctx, tenantID, id, st, reason, cond)
}

func (l lazyTelegramStore) UpdateInbound(ctx context.Context, tenantID, id uuid.UUID, fn func(state json.RawMessage) (json.RawMessage, error)) error {
	s, err := l.store()
	if err != nil {
		return err
	}
	return s.UpdateInbound(ctx, tenantID, id, fn)
}

var errStoreLacks = errors.New("automations: the connection store does not keep this state")

func (l lazyTelegramStore) InboundState(ctx context.Context, tenantID, id uuid.UUID) (json.RawMessage, error) {
	s, err := l.store()
	if err != nil {
		return nil, err
	}
	r, ok := s.(connectors.InboundReader)
	if !ok {
		return nil, errStoreLacks
	}
	return r.InboundState(ctx, tenantID, id)
}

func (l lazyTelegramStore) ConnectionExists(ctx context.Context, id uuid.UUID) (bool, error) {
	s, err := l.store()
	if err != nil {
		return true, err
	}
	r, ok := s.(connectors.ExistenceChecker)
	if !ok {
		return true, nil
	}
	return r.ConnectionExists(ctx, id)
}

func (l lazyTelegramStore) actions() (connectors.InboundActions, error) {
	s, err := l.store()
	if err != nil {
		return nil, err
	}
	a, ok := s.(connectors.InboundActions)
	if !ok {
		return nil, errStoreLacks
	}
	return a, nil
}

func (l lazyTelegramStore) ClaimInbound(ctx context.Context, connection, event, key string) (connectors.InboundAction, bool, error) {
	a, err := l.actions()
	if err != nil {
		return connectors.InboundAction{}, false, err
	}
	return a.ClaimInbound(ctx, connection, event, key)
}

func (l lazyTelegramStore) InboundHandled(ctx context.Context, connection, event, key string, owner uuid.UUID, tenantID uuid.UUID, reply string) error {
	a, err := l.actions()
	if err != nil {
		return err
	}
	return a.InboundHandled(ctx, connection, event, key, owner, tenantID, reply)
}

func (l lazyTelegramStore) ClaimDelivery(ctx context.Context, connection, event, key string) (connectors.InboundAction, bool, error) {
	a, err := l.actions()
	if err != nil {
		return connectors.InboundAction{}, false, err
	}
	return a.ClaimDelivery(ctx, connection, event, key)
}

func (l lazyTelegramStore) InboundCompleted(ctx context.Context, connection, event, key string, owner uuid.UUID) error {
	a, err := l.actions()
	if err != nil {
		return err
	}
	return a.InboundCompleted(ctx, connection, event, key, owner)
}

func (l lazyTelegramStore) ReleaseInbound(ctx context.Context, connection, event, key string, owner uuid.UUID) error {
	a, err := l.actions()
	if err != nil {
		return err
	}
	return a.ReleaseInbound(ctx, connection, event, key, owner)
}

// lazyUsage records the connector's own sends (the reply to /start) in automation.connector_usage.
type lazyUsage struct{ container *composition.Container }

func (l lazyUsage) Record(ctx context.Context, e catalog.UsageEntry) error {
	db, err := composition.Resolve[*pgxpool.Pool](l.container)
	if err != nil {
		return err
	}
	return connectors.NewUsageRecorder(db).Record(ctx, e)
}
