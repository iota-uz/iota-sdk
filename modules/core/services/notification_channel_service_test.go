package services_test

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/internet"
	"github.com/iota-uz/iota-sdk/modules/core/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/modules/core/notifications"
	"github.com/iota-uz/iota-sdk/modules/core/services"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/constants"
	"github.com/iota-uz/iota-sdk/pkg/itf"
	"github.com/stretchr/testify/require"
)

// fakeProvider is a synthetic channel adapter; it never sends anything real.
type fakeProvider struct {
	channel notifications.Channel
	mu      sync.Mutex
	err     error
	sent    []notifications.Message
	inTx    bool
}

func (p *fakeProvider) Channel() notifications.Channel { return p.channel }
func (p *fakeProvider) Send(ctx context.Context, m notifications.Message) (notifications.Receipt, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if ctx.Value(constants.TxKey) != nil {
		p.inTx = true
	}
	if p.err != nil {
		return notifications.Receipt{}, p.err
	}
	p.sent = append(p.sent, m)
	return notifications.Receipt{ProviderMessageID: fmt.Sprintf("%s-%s", p.channel, m.DeliveryID)}, nil
}
func (p *fakeProvider) setErr(err error) { p.mu.Lock(); p.err = err; p.mu.Unlock() }
func (p *fakeProvider) count() int       { p.mu.Lock(); defer p.mu.Unlock(); return len(p.sent) }

type fakeAddresses map[notifications.Channel]string

func (a fakeAddresses) Resolve(_ context.Context, _ uuid.UUID, _ uint, c notifications.Channel) (string, error) {
	if v, ok := a[c]; ok {
		return v, nil
	}
	return "", notifications.ErrNoAddress
}

type channelFixture struct {
	f                *itf.TestEnvironment
	email, sms, push *fakeProvider
	channels         *services.NotificationChannelService
	dispatch         *services.NotificationDispatchService
	router           *services.NotificationRoutingService
	userID           uint
	committed        context.Context
}

func setupChannels(t *testing.T, addresses fakeAddresses) *channelFixture {
	t.Helper()
	c := &channelFixture{
		email: &fakeProvider{channel: notifications.ChannelEmail},
		sms:   &fakeProvider{channel: notifications.ChannelSMS},
		push:  &fakeProvider{channel: notifications.ChannelPush},
	}
	c.f = setupNotificationChannelTest(t, []notifications.ChannelAdapter{c.email, c.sms, c.push}, addresses)
	c.channels = itf.GetService[services.NotificationChannelService](c.f)
	c.dispatch = itf.GetService[services.NotificationDispatchService](c.f)
	c.router = itf.GetService[services.NotificationRoutingService](c.f)
	users := persistence.NewUserRepository(persistence.NewUploadRepository())
	u, err := users.Create(c.f.Ctx, user.New("Channel", "Recipient", internet.MustParseEmail(fmt.Sprintf("channel%s@example.com", uuid.NewString()[:8])), user.UILanguageEN, user.WithTenantID(c.f.TenantID())))
	require.NoError(t, err)
	c.userID = u.ID()
	return c
}

func (c *channelFixture) commit(t *testing.T) {
	t.Helper()
	require.NoError(t, c.f.Tx.Commit(c.f.Ctx))
	c.committed = userCommittedCtx(c.f)
}

func (c *channelFixture) statuses(t *testing.T, eventID string) map[notifications.Channel]notifications.Delivery {
	t.Helper()
	list, err := c.channels.Deliveries(c.committed, notifications.TestEventKey, eventID)
	require.NoError(t, err)
	byChannel := map[notifications.Channel]notifications.Delivery{}
	for _, d := range list {
		byChannel[d.Channel] = d
	}
	return byChannel
}

func (c *channelFixture) run(t *testing.T) {
	t.Helper()
	worked, err := c.dispatch.Process(c.committed)
	require.NoError(t, err)
	require.True(t, worked)
}

var allChannels = []notifications.Channel{notifications.ChannelInApp, notifications.ChannelEmail, notifications.ChannelSMS, notifications.ChannelPush}

func TestNotificationChannels_FullDeliveryThroughRule(t *testing.T) {
	c := setupChannels(t, fakeAddresses{notifications.ChannelEmail: "a@example.com", notifications.ChannelSMS: "+998900000000", notifications.ChannelPush: "device-1"})
	require.NoError(t, c.router.SaveRule(c.f.Ctx, notifications.Rule{EventKey: notifications.TestEventKey, Enabled: true, UserIDs: []uint{c.userID}, Channels: allChannels}))
	eventID := uuid.NewString()
	n, err := c.dispatch.Enqueue(c.f.Ctx, notifications.Event{Key: notifications.TestEventKey, ID: eventID, TenantID: c.f.TenantID()})
	require.NoError(t, err)
	require.Equal(t, 1, n)
	c.commit(t)
	require.Zero(t, c.email.count(), "providers must not run before the producer transaction commits")
	c.run(t)
	processed, err := c.channels.Process(c.committed)
	require.NoError(t, err)
	require.Equal(t, 3, processed)
	got := c.statuses(t, eventID)
	require.Len(t, got, 4)
	for _, ch := range allChannels {
		require.Equal(t, notifications.DeliverySent, got[ch].Status, ch)
	}
	require.Equal(t, "email-"+got[notifications.ChannelEmail].ID.String(), got[notifications.ChannelEmail].ProviderReceipt)
	require.Equal(t, "a@example.com", c.email.sent[0].Address)
	require.Equal(t, "device-1", c.push.sent[0].Address)
	require.False(t, c.email.inTx || c.sms.inTx || c.push.inTx, "providers must not run inside a database transaction")
	var inbox int
	require.NoError(t, c.f.Pool.QueryRow(c.committed, "SELECT COUNT(*) FROM core.notifications WHERE tenant_id=$1 AND user_id=$2", c.f.TenantID(), c.userID).Scan(&inbox))
	require.Equal(t, 1, inbox)
}

func TestNotificationChannels_PartialDeliveryMissingContactAndSingleChannelRetry(t *testing.T) {
	c := setupChannels(t, fakeAddresses{notifications.ChannelEmail: "a@example.com", notifications.ChannelSMS: "+998900000000"})
	c.sms.setErr(errors.New("provider unavailable"))
	require.NoError(t, c.router.SaveRule(c.f.Ctx, notifications.Rule{EventKey: notifications.TestEventKey, Enabled: true, UserIDs: []uint{c.userID}, Channels: allChannels}))
	eventID := uuid.NewString()
	_, err := c.dispatch.Enqueue(c.f.Ctx, notifications.Event{Key: notifications.TestEventKey, ID: eventID, TenantID: c.f.TenantID()})
	require.NoError(t, err)
	c.commit(t)
	c.run(t)
	_, err = c.channels.Process(c.committed)
	require.NoError(t, err)
	got := c.statuses(t, eventID)
	require.Equal(t, notifications.DeliverySent, got[notifications.ChannelInApp].Status)
	require.Equal(t, notifications.DeliverySent, got[notifications.ChannelEmail].Status)
	require.Equal(t, notifications.DeliveryPending, got[notifications.ChannelSMS].Status, "transient provider failure stays retryable")
	require.Equal(t, 1, got[notifications.ChannelSMS].Attempts)
	require.NotContains(t, got[notifications.ChannelSMS].LastError, "unavailable", "provider error text must not be stored")
	require.Equal(t, notifications.DeliveryNoAddress, got[notifications.ChannelPush].Status)
	// A partially delivered event is not reported as one success.
	require.NotEqual(t, notifications.DeliverySent, got[notifications.ChannelSMS].Status)

	c.sms.setErr(nil)
	require.NoError(t, c.channels.Retry(c.committed, got[notifications.ChannelPush].ID), "a recipient who added a device can be retried")
	require.ErrorIs(t, c.channels.Retry(c.committed, got[notifications.ChannelEmail].ID), notifications.ErrDeliveryNotFound, "a sent delivery is never repeated")
	_, err = c.f.Pool.Exec(c.committed, "UPDATE core.notification_deliveries SET available_at=NOW() WHERE id=$1", got[notifications.ChannelSMS].ID)
	require.NoError(t, err)
	_, err = c.channels.Process(c.committed)
	require.NoError(t, err)
	after := c.statuses(t, eventID)
	require.Equal(t, notifications.DeliverySent, after[notifications.ChannelSMS].Status)
	require.Equal(t, 1, c.email.count(), "retrying SMS must not resend email")
	require.Equal(t, 1, c.sms.count())
	require.Equal(t, notifications.DeliveryNoAddress, after[notifications.ChannelPush].Status)
}

func TestNotificationChannels_PermanentFailureAndExhaustedAttempts(t *testing.T) {
	c := setupChannels(t, fakeAddresses{notifications.ChannelEmail: "a@example.com", notifications.ChannelSMS: "+998900000000"})
	c.email.setErr(fmt.Errorf("rejected: %w", notifications.ErrPermanent))
	c.sms.setErr(errors.New("down"))
	require.NoError(t, c.router.SaveRule(c.f.Ctx, notifications.Rule{EventKey: notifications.TestEventKey, Enabled: true, UserIDs: []uint{c.userID}, Channels: []notifications.Channel{notifications.ChannelEmail, notifications.ChannelSMS}}))
	eventID := uuid.NewString()
	_, err := c.dispatch.Enqueue(c.f.Ctx, notifications.Event{Key: notifications.TestEventKey, ID: eventID, TenantID: c.f.TenantID()})
	require.NoError(t, err)
	c.commit(t)
	c.run(t)
	for range notifications.MaxDeliveryAttempts {
		_, err = c.f.Pool.Exec(c.committed, "UPDATE core.notification_deliveries SET available_at=NOW() WHERE tenant_id=$1 AND status='pending'", c.f.TenantID())
		require.NoError(t, err)
		_, err = c.channels.Process(c.committed)
		require.NoError(t, err)
	}
	got := c.statuses(t, eventID)
	require.Equal(t, notifications.DeliveryFailed, got[notifications.ChannelEmail].Status)
	require.Equal(t, 1, got[notifications.ChannelEmail].Attempts, "permanent errors are not retried")
	require.Equal(t, notifications.DeliveryFailed, got[notifications.ChannelSMS].Status)
	require.Equal(t, notifications.MaxDeliveryAttempts, got[notifications.ChannelSMS].Attempts)
	require.NoError(t, c.channels.Retry(c.committed, got[notifications.ChannelSMS].ID))
	require.Equal(t, notifications.DeliveryPending, c.statuses(t, eventID)[notifications.ChannelSMS].Status)
	require.Equal(t, notifications.DeliveryFailed, c.statuses(t, eventID)[notifications.ChannelEmail].Status)
}

func TestNotificationChannels_AddressedSendIgnoresEventRuleAndRollsBack(t *testing.T) {
	c := setupChannels(t, fakeAddresses{notifications.ChannelEmail: "a@example.com"})
	// The administrator disabled the event; an explicit producer still reaches its recipients.
	require.NoError(t, c.router.SaveRule(c.f.Ctx, notifications.Rule{EventKey: notifications.TestEventKey, Enabled: false}))
	c.commit(t)
	rolledBack := uuid.NewString()
	err := composables.InTx(c.committed, func(tx context.Context) error {
		n, err := c.dispatch.EnqueueAddressed(tx, services.AddressedRequest{Event: notifications.Event{Key: notifications.TestEventKey, ID: rolledBack, TenantID: c.f.TenantID()}, RecipientIDs: []uint{c.userID}, Channels: []notifications.Channel{notifications.ChannelEmail}})
		require.NoError(t, err)
		require.Equal(t, 1, n)
		return errors.New("business transaction failed")
	})
	require.Error(t, err)
	var jobs int
	require.NoError(t, c.f.Pool.QueryRow(c.committed, "SELECT COUNT(*) FROM core.notification_dispatches WHERE tenant_id=$1 AND event_id=$2", c.f.TenantID(), rolledBack).Scan(&jobs))
	require.Zero(t, jobs, "rollback of the business transaction cancels the enqueue")

	eventID := uuid.NewString()
	n, err := c.dispatch.EnqueueAddressed(c.committed, services.AddressedRequest{Event: notifications.Event{Key: notifications.TestEventKey, ID: eventID, TenantID: c.f.TenantID()}, RecipientIDs: []uint{c.userID, c.userID}, Channels: []notifications.Channel{notifications.ChannelEmail}})
	require.NoError(t, err)
	require.Equal(t, 1, n)
	c.run(t)
	_, err = c.channels.Process(c.committed)
	require.NoError(t, err)
	got := c.statuses(t, eventID)
	require.Len(t, got, 1)
	require.Equal(t, notifications.DeliverySent, got[notifications.ChannelEmail].Status)
	rule, err := c.router.Rule(c.committed, notifications.TestEventKey)
	require.NoError(t, err)
	require.False(t, rule.Enabled, "addressed send must not change the admin rule")
}

func TestNotificationChannels_RejectsUnavailableChannels(t *testing.T) {
	f := setupNotificationTest(t)
	router := itf.GetService[services.NotificationRoutingService](f)
	err := router.SaveRule(f.Ctx, notifications.Rule{EventKey: notifications.TestEventKey, Enabled: true, RecipientKeys: nil, UserIDs: []uint{}, Channels: []notifications.Channel{notifications.ChannelSMS}})
	require.Error(t, err)
	dispatch := itf.GetService[services.NotificationDispatchService](f)
	_, err = dispatch.EnqueueAddressed(f.Ctx, services.AddressedRequest{Event: notifications.Event{Key: notifications.TestEventKey, ID: uuid.NewString(), TenantID: f.TenantID()}, RecipientIDs: []uint{1}, Channels: []notifications.Channel{notifications.ChannelPush}})
	require.Error(t, err)
	require.Equal(t, []notifications.Channel{notifications.ChannelInApp}, router.Channels())
}

func TestNotificationChannels_ExhaustedLeaseFailsWithoutAnotherProviderCall(t *testing.T) {
	c := setupChannels(t, fakeAddresses{notifications.ChannelEmail: "a@example.com"})
	require.NoError(t, c.router.SaveRule(c.f.Ctx, notifications.Rule{EventKey: notifications.TestEventKey, Enabled: true, UserIDs: []uint{c.userID}, Channels: []notifications.Channel{notifications.ChannelEmail}}))
	eventID := uuid.NewString()
	_, err := c.dispatch.Enqueue(c.f.Ctx, notifications.Event{Key: notifications.TestEventKey, ID: eventID, TenantID: c.f.TenantID()})
	require.NoError(t, err)
	c.commit(t)
	c.run(t)
	// The fifth outcome was never recorded and its lease expired.
	_, err = c.f.Pool.Exec(c.committed, "UPDATE core.notification_deliveries SET attempts=$2,available_at=NOW() WHERE tenant_id=$1", c.f.TenantID(), notifications.MaxDeliveryAttempts)
	require.NoError(t, err)
	processed, err := c.channels.Process(c.committed)
	require.NoError(t, err)
	require.Zero(t, processed)
	require.Zero(t, c.email.count())
	require.Equal(t, notifications.DeliveryFailed, c.statuses(t, eventID)[notifications.ChannelEmail].Status)
}

func TestNotificationDeliveryRepository_StaleOutcomeIsDropped(t *testing.T) {
	c := setupChannels(t, fakeAddresses{notifications.ChannelEmail: "a@example.com"})
	require.NoError(t, c.router.SaveRule(c.f.Ctx, notifications.Rule{EventKey: notifications.TestEventKey, Enabled: true, UserIDs: []uint{c.userID}, Channels: []notifications.Channel{notifications.ChannelEmail}}))
	eventID := uuid.NewString()
	_, err := c.dispatch.Enqueue(c.f.Ctx, notifications.Event{Key: notifications.TestEventKey, ID: eventID, TenantID: c.f.TenantID()})
	require.NoError(t, err)
	c.commit(t)
	c.run(t)
	repo := persistence.NewNotificationDeliveryRepository()
	first, err := repo.Claim(c.committed, 10, 0)
	require.NoError(t, err)
	require.Len(t, first, 1)
	second, err := repo.Claim(c.committed, 10, 0)
	require.NoError(t, err)
	require.Len(t, second, 1)
	require.Equal(t, first[0].Attempts+1, second[0].Attempts)
	require.NoError(t, repo.Record(c.committed, second[0].ID, second[0].Attempts, notifications.DeliverySent, "new", "", 0))
	require.NoError(t, repo.Record(c.committed, first[0].ID, first[0].Attempts, notifications.DeliveryFailed, "", "stale", 0))
	got := c.statuses(t, eventID)[notifications.ChannelEmail]
	require.Equal(t, notifications.DeliverySent, got.Status)
	require.Equal(t, "new", got.ProviderReceipt)
}
