package automations

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/iota-uz/iota-sdk/modules/core/notifications"
	"github.com/iota-uz/iota-sdk/modules/core/notifications/telegramchannel"
	"github.com/iota-uz/iota-sdk/pkg/automation/catalog"
	"github.com/iota-uz/iota-sdk/pkg/automation/connectors"
	"github.com/iota-uz/iota-sdk/pkg/automation/connectors/telegram"
	"github.com/iota-uz/iota-sdk/pkg/composition"
)

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
