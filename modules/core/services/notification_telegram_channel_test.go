package services_test

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/internet"
	"github.com/iota-uz/iota-sdk/modules/core/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/modules/core/notifications"
	"github.com/iota-uz/iota-sdk/modules/core/services"
	"github.com/iota-uz/iota-sdk/pkg/composables"
)

const platformBot = "platform:telegram_bot"

// fakeConnectors is the tenant's working telegram_bot connections.
type fakeConnectors struct {
	mu   sync.Mutex
	list []notifications.ConnectorConnection
}

func (f *fakeConnectors) set(ids ...string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.list = nil
	for _, id := range ids {
		f.list = append(f.list, notifications.ConnectorConnection{ID: id, Name: id, Platform: strings.HasPrefix(id, "platform:")})
	}
}

func (f *fakeConnectors) Connections(context.Context, uuid.UUID, string) ([]notifications.ConnectorConnection, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]notifications.ConnectorConnection(nil), f.list...), nil
}

type telegramFixture struct {
	*channelFixture
	telegram   *fakeProvider
	registry   *notifications.Registry
	connectors *fakeConnectors
	conns      *services.NotificationChannelConnectionService
	links      *services.NotificationChannelLinkService
	linkRepo   notifications.ChannelLinkRepository
	user       user.User
}

func setupTelegram(t *testing.T) *telegramFixture {
	t.Helper()
	c := setupChannels(t, fakeAddresses{notifications.ChannelEmail: "a@example.com"})
	tf := &telegramFixture{channelFixture: c, telegram: &fakeProvider{channel: notifications.ChannelTelegram}, connectors: &fakeConnectors{}}
	tf.connectors.set(platformBot)
	reg, err := notifications.NewChannelRegistry(c.email, c.sms, c.push)
	require.NoError(t, err)
	require.NoError(t, reg.Register(notifications.ChannelSpec{
		Channel: notifications.ChannelTelegram, Order: 40, Address: notifications.AddressLinked, ConnectorKind: "telegram_bot",
		Label: map[string]string{"en": "Telegram"},
		LinkURL: func(_ context.Context, _ uuid.UUID, connectionID, token string) (string, error) {
			return "https://t.me/acme_bot?start=u_" + token + "&via=" + url.QueryEscape(connectionID), nil
		},
	}, tf.telegram))
	tf.registry = reg
	tf.linkRepo = persistence.NewNotificationChannelLinkRepository()
	registry := func() notifications.ChannelRegistry { return reg }
	tf.conns = services.NewNotificationChannelConnectionService(persistence.NewNotificationChannelConnectionRepository(), tf.linkRepo, registry,
		func() notifications.ConnectorConnections { return tf.connectors }).WithPool(c.f.Pool)
	reg.WithConnections(tf.conns)
	tf.links = services.NewNotificationChannelLinkService(registry, tf.conns, tf.linkRepo, persistence.NewNotificationLinkTokenRepository()).WithPool(c.f.Pool)
	c.channels.WithRegistry(reg).WithLinks(tf.linkRepo).WithConnections(tf.conns)
	users := persistence.NewUserRepository(persistence.NewUploadRepository())
	tf.user, err = users.GetByID(c.f.Ctx, c.userID)
	require.NoError(t, err)
	return tf
}

// as is the committed context of a user of the tenant.
func (tf *telegramFixture) as(u user.User) context.Context {
	return composables.WithUser(tf.committed, u)
}

// webhook is the context of a provider's webhook: the database, no tenant, no user.
func (tf *telegramFixture) webhook() context.Context { return context.Background() }

func (tf *telegramFixture) start(t *testing.T, u user.User) (string, string) {
	t.Helper()
	start, err := tf.links.StartLink(tf.as(u), notifications.ChannelTelegram)
	require.NoError(t, err)
	parsed, err := url.Parse(start.URL)
	require.NoError(t, err)
	require.WithinDuration(t, time.Now().Add(notifications.LinkTokenTTL), start.ExpiresAt, time.Minute)
	return strings.TrimPrefix(parsed.Query().Get("start"), "u_"), parsed.Query().Get("via")
}

func (tf *telegramFixture) newUser(t *testing.T) user.User {
	t.Helper()
	users := persistence.NewUserRepository(persistence.NewUploadRepository())
	u, err := users.Create(tf.committed, user.New("Second", "Recipient", internet.MustParseEmail(fmt.Sprintf("tg%s@example.com", uuid.NewString()[:8])), user.UILanguageRU, user.WithTenantID(tf.f.TenantID())))
	require.NoError(t, err)
	return u
}

func TestTelegramLink_DeepLinkIsOneUseAndIsolated(t *testing.T) {
	tf := setupTelegram(t)
	tf.commit(t)
	token, connection := tf.start(t, tf.user)
	require.Equal(t, platformBot, connection)

	status, err := tf.links.StatusOf(tf.committed, tf.user.ID(), notifications.ChannelTelegram)
	require.NoError(t, err)
	require.True(t, status.Pending)

	tenant, ok, err := tf.links.TenantOf(tf.webhook(), token)
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, tf.f.TenantID(), tenant)

	// Through another connection (a tenant bot) the token links nothing.
	_, err = tf.links.CompleteLinkUpdate(tf.webhook(), uuid.NewString(), token, "777", "@ada", "1:1")
	require.ErrorIs(t, err, notifications.ErrLinkTokenInvalid)

	done, err := tf.links.CompleteLinkUpdate(tf.webhook(), platformBot, token, "777", "@ada", "1:2")
	require.NoError(t, err)
	require.Equal(t, tf.user.ID(), done.UserID)
	require.False(t, done.Replayed)

	// The redelivery of the same update is answered the same; another /start with the token is refused.
	done, err = tf.links.CompleteLinkUpdate(tf.webhook(), platformBot, token, "777", "@ada", "1:2")
	require.NoError(t, err)
	require.True(t, done.Replayed)
	_, err = tf.links.CompleteLinkUpdate(tf.webhook(), platformBot, token, "777", "@ada", "1:3")
	require.ErrorIs(t, err, notifications.ErrLinkTokenInvalid)
	_, ok, err = tf.links.TenantOf(tf.webhook(), token)
	require.NoError(t, err)
	require.False(t, ok, "a used token has no tenant")
	got, ok, err := tf.links.TenantOfUpdate(tf.webhook(), "1:2")
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, tf.f.TenantID(), got)

	status, err = tf.links.StatusOf(tf.committed, tf.user.ID(), notifications.ChannelTelegram)
	require.NoError(t, err)
	require.True(t, status.Linked)
	require.Equal(t, "@ada", status.Display)
	var rows int
	require.NoError(t, tf.f.Pool.QueryRow(tf.committed, `SELECT COUNT(*) FROM core.notification_channel_links WHERE tenant_id=$1`, tf.f.TenantID()).Scan(&rows))
	require.Equal(t, 1, rows, "one link, however often the link is followed")
}

func TestTelegramLink_ExpiredTokenAndChatMovesToAnotherUser(t *testing.T) {
	tf := setupTelegram(t)
	tf.commit(t)
	token, _ := tf.start(t, tf.user)
	future := services.NewNotificationChannelLinkService(func() notifications.ChannelRegistry { return tf.registry }, tf.conns, tf.linkRepo, persistence.NewNotificationLinkTokenRepository()).
		WithPool(tf.f.Pool).WithClock(func() time.Time { return time.Now().Add(16 * time.Minute) })
	_, err := future.CompleteLinkUpdate(tf.webhook(), platformBot, token, "777", "@ada", "2:1")
	require.ErrorIs(t, err, notifications.ErrLinkTokenInvalid, "a token lives 15 minutes")
	status, err := future.StatusOf(tf.committed, tf.user.ID(), notifications.ChannelTelegram)
	require.NoError(t, err)
	require.True(t, status.Expired)

	_, err = tf.links.CompleteLinkUpdate(tf.webhook(), platformBot, token, "777", "@ada", "2:2")
	require.NoError(t, err)
	other := tf.newUser(t)
	otherToken, _ := tf.start(t, other)
	_, err = tf.links.CompleteLinkUpdate(tf.webhook(), platformBot, otherToken, "777", "@ada", "2:3")
	require.NoError(t, err)
	first, err := tf.links.StatusOf(tf.committed, tf.user.ID(), notifications.ChannelTelegram)
	require.NoError(t, err)
	require.False(t, first.Linked, "a chat belongs to one person of the tenant: it moved")
	second, err := tf.links.StatusOf(tf.committed, other.ID(), notifications.ChannelTelegram)
	require.NoError(t, err)
	require.True(t, second.Linked)
}

func TestTelegramChannel_DeliveryBlockedBotAndAvailability(t *testing.T) {
	tf := setupTelegram(t)
	require.NoError(t, tf.router.SaveRule(tf.f.Ctx, notifications.Rule{EventKey: notifications.TestEventKey, Enabled: true, UserIDs: []uint{tf.userID},
		Channels: []notifications.Channel{notifications.ChannelInApp, notifications.ChannelTelegram}}))
	require.Error(t, tf.channels.Validate(tf.f.Ctx, []notifications.Channel{"Telegram"}), "a malformed channel is refused before writing")
	require.Error(t, tf.channels.Validate(tf.f.Ctx, []notifications.Channel{"whatsapp"}), "an unregistered channel is refused before writing")
	tf.commit(t)
	token, _ := tf.start(t, tf.user)
	_, err := tf.links.CompleteLinkUpdate(tf.webhook(), platformBot, token, "777", "@ada", "3:1")
	require.NoError(t, err)

	send := func() map[notifications.Channel]notifications.Delivery {
		eventID := uuid.NewString()
		_, err := tf.dispatch.Enqueue(tf.committed, notifications.Event{Key: notifications.TestEventKey, ID: eventID, TenantID: tf.f.TenantID()})
		require.NoError(t, err)
		tf.run(t)
		_, err = tf.channels.Process(tf.committed)
		require.NoError(t, err)
		return tf.statuses(t, eventID)
	}
	got := send()
	require.Equal(t, notifications.DeliverySent, got[notifications.ChannelTelegram].Status)
	require.Equal(t, "777", tf.telegram.sent[0].Address)

	// The user blocked the bot: the delivery fails for good, the link is removed and shown as such.
	tf.telegram.setErr(fmt.Errorf("%w: %w: blocked", notifications.ErrPermanent, notifications.ErrTargetGone))
	got = send()
	require.Equal(t, notifications.DeliveryFailed, got[notifications.ChannelTelegram].Status)
	require.Equal(t, notifications.ErrorCodeProviderRejected, got[notifications.ChannelTelegram].ErrorCode)
	require.Equal(t, 1, got[notifications.ChannelTelegram].Attempts, "no automatic retry")
	status, err := tf.links.StatusOf(tf.committed, tf.userID, notifications.ChannelTelegram)
	require.NoError(t, err)
	require.False(t, status.Linked)
	require.Equal(t, notifications.LinkRevokedTargetGone, status.Revoked)
	got = send()
	require.Equal(t, notifications.DeliveryNoAddress, got[notifications.ChannelTelegram].Status)

	// A tenant without a connection and without the platform bot does not see the channel; a delivery
	// queued before is skipped.
	tf.telegram.setErr(nil)
	eventID := uuid.NewString()
	_, err = tf.dispatch.Enqueue(tf.committed, notifications.Event{Key: notifications.TestEventKey, ID: eventID, TenantID: tf.f.TenantID()})
	require.NoError(t, err)
	tf.run(t)
	tf.connectors.set()
	available, err := tf.channels.Available(tf.committed)
	require.NoError(t, err)
	require.NotContains(t, available, notifications.ChannelTelegram)
	_, err = tf.channels.Process(tf.committed)
	require.NoError(t, err)
	got = tf.statuses(t, eventID)
	require.Equal(t, notifications.DeliverySkipped, got[notifications.ChannelTelegram].Status)
	require.Equal(t, notifications.ErrorCodeChannelNotConfigured, got[notifications.ChannelTelegram].ErrorCode)
}

func TestTelegramChannel_ChangingTheBotRemovesLinks(t *testing.T) {
	tf := setupTelegram(t)
	tf.commit(t)
	tenantBot := uuid.NewString()
	tf.connectors.set(tenantBot, platformBot)
	token, connection := tf.start(t, tf.user)
	require.Equal(t, platformBot, connection, "without a choice the channel sends through the platform bot")
	_, err := tf.links.CompleteLinkUpdate(tf.webhook(), platformBot, token, "777", "@ada", "4:1")
	require.NoError(t, err)

	removed, err := tf.conns.Set(tf.committed, notifications.ChannelTelegram, platformBot)
	require.NoError(t, err)
	require.False(t, removed, "choosing the bot the channel already sends through keeps the links")
	_, err = tf.conns.Set(tf.committed, notifications.ChannelTelegram, "platform:other")
	require.Error(t, err, "only a working connection can be chosen")

	removed, err = tf.conns.Set(tf.committed, notifications.ChannelTelegram, tenantBot)
	require.NoError(t, err)
	require.True(t, removed)
	status, err := tf.links.StatusOf(tf.committed, tf.user.ID(), notifications.ChannelTelegram)
	require.NoError(t, err)
	require.False(t, status.Linked)
	require.Empty(t, status.Revoked, "the person sees «Connect Telegram» again")
	_, connection = tf.start(t, tf.user)
	require.Equal(t, tenantBot, connection)
}

func TestNotificationPreferences_OptInChannels(t *testing.T) {
	tf := setupTelegram(t)
	tf.commit(t)
	optIn := notifications.ChannelSpec{Channel: "whatsapp", Order: 50, Address: notifications.AddressNone}
	require.NoError(t, tf.registry.Register(optIn, &fakeProvider{channel: "whatsapp"}))
	prefs := persistence.NewNotificationPreferenceRepository(persistence.WithOptInChannels(func() []notifications.Channel { return []notifications.Channel{"whatsapp"} }))
	disabled, err := prefs.Disabled(tf.committed, []uint{tf.userID})
	require.NoError(t, err)
	assert.Equal(t, []notifications.Channel{"whatsapp"}, disabled[tf.userID], "an opt-in channel is off until the person turns it on")
	require.NoError(t, prefs.SetDisabled(tf.committed, tf.userID, []notifications.Channel{notifications.ChannelEmail}))
	disabled, err = prefs.Disabled(tf.committed, []uint{tf.userID})
	require.NoError(t, err)
	assert.ElementsMatch(t, []notifications.Channel{notifications.ChannelEmail}, disabled[tf.userID])
	var rows []string
	r, err := tf.f.Pool.Query(tf.committed, `SELECT channel FROM core.notification_channel_preferences WHERE tenant_id=$1 AND user_id=$2 ORDER BY channel`, tf.f.TenantID(), int(tf.userID))
	require.NoError(t, err)
	for r.Next() {
		var c string
		require.NoError(t, r.Scan(&c))
		rows = append(rows, c)
	}
	assert.Equal(t, []string{"email", "whatsapp"}, rows, "a row of an opt-in channel means on, of another channel off")
}

func TestNotificationChannelCheck_AcceptsNewChannelsAndRefusesMalformed(t *testing.T) {
	tf := setupTelegram(t)
	tf.commit(t)
	_, err := tf.f.Pool.Exec(tf.committed, `INSERT INTO core.notification_channel_preferences(tenant_id,user_id,channel) VALUES($1,$2,'telegram')`, tf.f.TenantID(), int(tf.userID))
	require.NoError(t, err)
	_, err = tf.f.Pool.Exec(tf.committed, `INSERT INTO core.notification_channel_preferences(tenant_id,user_id,channel) VALUES($1,$2,'Telegram')`, tf.f.TenantID(), int(tf.userID))
	require.Error(t, err)
}
