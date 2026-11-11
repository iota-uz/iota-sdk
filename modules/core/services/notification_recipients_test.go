package services_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules"
	"github.com/iota-uz/iota-sdk/modules/core"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/internet"
	"github.com/iota-uz/iota-sdk/modules/core/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/modules/core/notifications"
	"github.com/iota-uz/iota-sdk/modules/core/notifications/notificationstest"
	"github.com/iota-uz/iota-sdk/modules/core/services"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/defaults"
	"github.com/iota-uz/iota-sdk/pkg/itf"
	"github.com/stretchr/testify/require"
)

// recipientsFixture wires the core notification stack with fake providers, a
// fake contact book and recording access/policy hooks against real PostgreSQL.
type recipientsFixture struct {
	f                *itf.TestEnvironment
	committed        context.Context
	email, sms, push *notificationstest.Adapter
	contacts         *notificationstest.Contacts
	access           *notificationstest.Access
	policy           *notificationstest.Policy
	channels         *services.NotificationChannelService
	dispatch         *services.NotificationDispatchService
	router           *services.NotificationRoutingService
	catalog          *notifications.Catalog
	renderers        *notifications.ContentRenderers
	users            []uint
}

var customerRef = notifications.ObjectRef{Entity: "acme.customer", Key: "c-1"}

func setupRecipients(t *testing.T, userAddresses fakeAddresses) *recipientsFixture {
	t.Helper()
	c := &recipientsFixture{
		email: notificationstest.NewEmail(), sms: notificationstest.NewSMS(), push: notificationstest.NewPush(),
		contacts: notificationstest.NewContacts(), access: notificationstest.NewAccess(), policy: notificationstest.NewPolicy(),
	}
	components := modules.Components()
	for i, component := range components {
		if component.Descriptor().Name == "core" {
			components[i] = core.NewComponent(&core.ModuleOptions{
				DisableNotificationWorker: true, PermissionSchema: defaults.PermissionSchema(),
				NotificationChannels:         []notifications.ChannelAdapter{c.email, c.sms, c.push},
				NotificationAddressResolver:  userAddresses,
				NotificationContactResolvers: map[string]notifications.ContactResolver{customerRef.Entity: c.contacts},
				NotificationSubjectAccess:    c.access,
				NotificationDeliveryPolicy:   c.policy,
			})
		}
	}
	c.f = itf.Setup(t, itf.WithComponents(components...), itf.WithUser(itf.User()))
	c.channels = itf.GetService[services.NotificationChannelService](c.f)
	c.dispatch = itf.GetService[services.NotificationDispatchService](c.f)
	c.router = itf.GetService[services.NotificationRoutingService](c.f)
	c.catalog = itf.GetService[notifications.Catalog](c.f)
	c.renderers = itf.GetService[notifications.ContentRenderers](c.f)
	repo := persistence.NewUserRepository(persistence.NewUploadRepository())
	for _, name := range []string{"First", "Second"} {
		u, err := repo.Create(c.f.Ctx, user.New(name, "Recipient", internet.MustParseEmail(fmt.Sprintf("%s%s@example.com", name, uuid.NewString()[:8])), user.UILanguageEN, user.WithTenantID(c.f.TenantID())))
		require.NoError(t, err)
		c.users = append(c.users, u.ID())
	}
	require.NoError(t, c.f.Tx.Commit(c.f.Ctx))
	c.committed = userCommittedCtx(c.f)
	return c
}

func (c *recipientsFixture) event(id string) notifications.Event {
	return notifications.Event{Key: notifications.TestEventKey, ID: id, TenantID: c.f.TenantID(), Subject: notifications.SubjectReference{Type: "acme.policy", ID: "p-1"}}
}

// drain runs the dispatch worker until idle, then one channel worker pass.
func (c *recipientsFixture) drain(t *testing.T) {
	t.Helper()
	for range 10 {
		worked, err := c.dispatch.Process(c.committed)
		require.NoError(t, err)
		if !worked {
			break
		}
	}
	_, err := c.channels.Process(c.committed)
	require.NoError(t, err)
}

func (c *recipientsFixture) deliveries(t *testing.T, eventKey, identity string) []notifications.Delivery {
	t.Helper()
	list, err := c.channels.Deliveries(c.committed, eventKey, identity)
	require.NoError(t, err)
	return list
}

func (c *recipientsFixture) count(t *testing.T, query string, args ...any) int {
	t.Helper()
	var n int
	require.NoError(t, c.f.Pool.QueryRow(c.committed, query, args...).Scan(&n))
	return n
}

func (c *recipientsFixture) backoffElapsed(t *testing.T) {
	t.Helper()
	_, err := c.f.Pool.Exec(c.committed, "UPDATE core.notification_deliveries SET available_at=NOW() WHERE tenant_id=$1 AND status IN ('pending','unresolved')", c.f.TenantID())
	require.NoError(t, err)
}

func byChannel(list []notifications.Delivery) map[notifications.Channel][]notifications.Delivery {
	out := map[notifications.Channel][]notifications.Delivery{}
	for _, d := range list {
		out[d.Channel] = append(out[d.Channel], d)
	}
	return out
}

func targetsOf(list []notifications.Delivery) map[string]notifications.Delivery {
	out := map[string]notifications.Delivery{}
	for _, d := range list {
		out[d.TargetAddress] = d
	}
	return out
}

func TestNotificationRecipients_ExternalCustomerWithoutAccountGetsSMSAndEmail(t *testing.T) {
	c := setupRecipients(t, nil)
	c.contacts.Add(customerRef, notifications.Contact{Language: "ru", Targets: []notifications.Target{
		{Channel: notifications.ChannelSMS, Address: "+998900000001"},
		{Channel: notifications.ChannelEmail, Address: "customer@example.com"},
		{Channel: notifications.ChannelEmail, Address: "billing@example.com"},
	}})
	c.policy.Deny("billing@example.com", "no_consent")
	usersBefore := c.count(t, "SELECT COUNT(*) FROM users WHERE tenant_id=$1", c.f.TenantID())

	id := uuid.NewString()
	receipt, err := c.dispatch.EnqueueExplicit(c.committed, services.AddressedRequest{
		Event: c.event(id), Recipients: []notifications.Addressee{notifications.RecordAddressee(customerRef.Entity, customerRef.Key)},
		Channels: []notifications.Channel{notifications.ChannelSMS, notifications.ChannelEmail},
		Producer: notifications.Producer{InvocationID: "inv-1"},
	})
	require.NoError(t, err)
	require.Equal(t, 1, receipt.Recipients)
	require.NotEqual(t, uuid.Nil, receipt.NoticeID)
	require.Zero(t, c.contacts.Calls(), "contacts are resolved after commit by the worker, not while enqueueing")
	require.Empty(t, c.sms.Calls())

	c.drain(t)
	var identity string
	require.NoError(t, c.f.Pool.QueryRow(c.committed, "SELECT event_id FROM core.notification_notices WHERE tenant_id=$1 AND id=$2", c.f.TenantID(), receipt.NoticeID).Scan(&identity))
	require.Contains(t, identity, "addressed:producer:invocation:inv-1:", "the dedupe identity derives from the producer")
	got := byChannel(c.deliveries(t, notifications.TestEventKey, identity))
	require.Len(t, got[notifications.ChannelSMS], 1)
	sms := got[notifications.ChannelSMS][0]
	require.Equal(t, notifications.DeliverySent, sms.Status)
	require.Equal(t, notifications.RecipientRecord, sms.RecipientKind)
	require.Zero(t, sms.UserID, "an external customer is not an ERP user")
	require.Equal(t, &customerRef, sms.Record)
	require.Equal(t, "+998900000001", sms.TargetAddress)
	require.Equal(t, "ru", sms.Language, "the contact's language wins")
	require.NotEmpty(t, c.sms.Sent()[0].Text)
	msg := c.sms.Sent()[0]
	require.Equal(t, notifications.RecipientRecord, msg.RecipientKind)
	require.Equal(t, customerRef, msg.Recipient)
	require.Equal(t, notifications.ObjectRef{Entity: "acme.policy", Key: "p-1"}, msg.Subject, "subject is separate from the recipient")
	require.Equal(t, "inv-1", msg.Producer.InvocationID)

	emails := targetsOf(got[notifications.ChannelEmail])
	require.Len(t, emails, 2)
	require.Equal(t, notifications.DeliverySent, emails["customer@example.com"].Status)
	require.Equal(t, notifications.DeliverySkipped, emails["billing@example.com"].Status, "business policy stops one target")
	require.Equal(t, "policy_denied:no_consent", emails["billing@example.com"].ErrorCode)
	require.Len(t, c.email.Sent(), 1)
	sent := c.email.Sent()[0]
	require.NotEmpty(t, sent.EmailSubject)
	require.NotEmpty(t, sent.HTML)
	require.NotEmpty(t, sent.Text)

	require.Empty(t, got[notifications.ChannelInApp], "in-app exists only for user recipients")
	require.Equal(t, usersBefore, c.count(t, "SELECT COUNT(*) FROM users WHERE tenant_id=$1", c.f.TenantID()), "no fake user is created for the contact")
	require.Zero(t, c.count(t, "SELECT COUNT(*) FROM core.notifications WHERE tenant_id=$1", c.f.TenantID()))
	require.False(t, c.contacts.CalledInTransaction() || c.sms.CalledInTransaction() || c.email.CalledInTransaction())

	attempts, err := c.channels.Attempts(c.committed, sms.ID)
	require.NoError(t, err)
	require.Len(t, attempts, 1)
	require.Equal(t, notifications.AttemptSent, attempts[0].Outcome)
	require.NotEmpty(t, attempts[0].ProviderReceipt)
	require.NotNil(t, attempts[0].FinishedAt)
}

func TestNotificationRecipients_DirectAddressAndMissingAddressIsVisible(t *testing.T) {
	c := setupRecipients(t, nil)
	id := uuid.NewString()
	_, err := c.dispatch.EnqueueExplicit(c.committed, services.AddressedRequest{
		Event: c.event(id),
		Recipients: []notifications.Addressee{
			notifications.AddressAddressee(map[notifications.Channel]string{notifications.ChannelSMS: "+998911111111", notifications.ChannelEmail: "walkin@example.com"}),
			notifications.AddressAddressee(map[notifications.Channel]string{notifications.ChannelSMS: "+998922222222"}),
		},
		Channels: []notifications.Channel{notifications.ChannelSMS, notifications.ChannelEmail},
	})
	require.NoError(t, err)
	c.drain(t)
	got := byChannel(c.deliveries(t, notifications.TestEventKey, "addressed:"+id))
	require.Len(t, got[notifications.ChannelSMS], 2)
	for _, d := range got[notifications.ChannelSMS] {
		require.Equal(t, notifications.DeliverySent, d.Status)
		require.Equal(t, notifications.RecipientAddress, d.RecipientKind)
		require.Zero(t, d.UserID)
		require.Nil(t, d.Record)
	}
	require.Len(t, got[notifications.ChannelEmail], 2)
	var noAddress, sent int
	for _, d := range got[notifications.ChannelEmail] {
		switch d.Status {
		case notifications.DeliveryNoAddress:
			noAddress++
			require.Equal(t, notifications.ErrorCodeNoAddress, d.ErrorCode)
		case notifications.DeliverySent:
			sent++
			require.Equal(t, "walkin@example.com", d.TargetAddress)
		}
	}
	require.Equal(t, 1, noAddress, "the second person has no email: a visible outcome, not overall success")
	require.Equal(t, 1, sent)
	require.Len(t, c.email.Sent(), 1)
}

func TestNotificationRecipients_UserInAppAndPushFanOutWithPartialFailure(t *testing.T) {
	c := setupRecipients(t, nil)
	userID := c.users[0]
	for _, device := range []string{"d1", "d2", "d3", "d4"} {
		require.NoError(t, c.channels.RegisterDevice(c.committed, notifications.PushDevice{UserID: userID, DeviceID: device, Token: "tok-" + device, Platform: notifications.PushAndroid}))
	}
	c.push.FailAddress("tok-d2", errors.New("provider down"))
	c.push.FailAddress("tok-d3", fmt.Errorf("lost reply: %w", notifications.ErrAmbiguous))
	c.push.FailAddress("tok-d4", fmt.Errorf("invalid token: %w", notifications.ErrPermanent))

	id := uuid.NewString()
	_, err := c.dispatch.EnqueueExplicit(c.committed, services.AddressedRequest{Event: c.event(id), RecipientIDs: []uint{userID}, Channels: []notifications.Channel{notifications.ChannelInApp, notifications.ChannelPush}})
	require.NoError(t, err)
	c.drain(t)
	got := byChannel(c.deliveries(t, notifications.TestEventKey, "addressed:"+id))

	require.Len(t, got[notifications.ChannelInApp], 1)
	inApp := got[notifications.ChannelInApp][0]
	require.Equal(t, notifications.DeliverySent, inApp.Status)
	var inboxID uuid.UUID
	var readAt *string
	require.NoError(t, c.f.Pool.QueryRow(c.committed, "SELECT id, read_at::text FROM core.notifications WHERE tenant_id=$1 AND user_id=$2", c.f.TenantID(), userID).Scan(&inboxID, &readAt))
	require.Equal(t, inboxID, inApp.InboxNotificationID, "the in-app delivery links to the inbox row")
	require.Nil(t, readAt, "read/unread stays an inbox property")

	devices := map[string]notifications.Delivery{}
	for _, d := range got[notifications.ChannelPush] {
		devices[d.TargetDeviceID] = d
		require.Equal(t, userID, d.UserID)
		require.Equal(t, notifications.RecipientUser, d.RecipientKind)
	}
	require.Len(t, devices, 4, "one delivery per device")
	require.Equal(t, notifications.DeliverySent, devices["d1"].Status)
	require.Equal(t, notifications.DeliveryPending, devices["d2"].Status, "a transient failure of one device stays retryable")
	require.Equal(t, notifications.DeliveryAmbiguous, devices["d3"].Status)
	require.Equal(t, notifications.DeliveryFailed, devices["d4"].Status)
	require.Equal(t, notifications.ErrorCodeProviderAmbiguous, devices["d3"].ErrorCode)
	require.Equal(t, "tok-d1", devices["d1"].TargetAddress)
	require.Len(t, c.push.SentTo("tok-d1"), 1)

	outcome := func(deviceID string) []notifications.AttemptOutcome {
		attempts, err := c.channels.Attempts(c.committed, devices[deviceID].ID)
		require.NoError(t, err)
		var out []notifications.AttemptOutcome
		for _, a := range attempts {
			out = append(out, a.Outcome)
		}
		return out
	}
	require.Equal(t, []notifications.AttemptOutcome{notifications.AttemptSent}, outcome("d1"))
	require.Equal(t, []notifications.AttemptOutcome{notifications.AttemptFailed}, outcome("d2"))
	require.Equal(t, []notifications.AttemptOutcome{notifications.AttemptAmbiguous}, outcome("d3"), "an unknown provider outcome is distinguishable from failure")
	require.Equal(t, []notifications.AttemptOutcome{notifications.AttemptPermanent}, outcome("d4"))

	// Ambiguous and permanent deliveries are not retried automatically, and the
	// healthy sibling is never resent.
	c.backoffElapsed(t)
	_, err = c.channels.Process(c.committed)
	require.NoError(t, err)
	require.Len(t, c.push.Calls(), 5, "only d2 ran again")
	c.push.FailAddress("tok-d2", nil)
	c.backoffElapsed(t)
	_, err = c.channels.Process(c.committed)
	require.NoError(t, err)
	require.Len(t, c.push.SentTo("tok-d2"), 1)
	require.Len(t, c.push.SentTo("tok-d1"), 1, "a sibling failure never repeats a delivered device")

	// An operator retries the ambiguous delivery; the provider idempotency key
	// stays the same across attempts.
	c.push.FailAddress("tok-d3", nil)
	require.NoError(t, c.channels.Retry(c.committed, devices["d3"].ID))
	_, err = c.channels.Process(c.committed)
	require.NoError(t, err)
	var d3 []notifications.Message
	for _, m := range c.push.Calls() {
		if m.Address == "tok-d3" {
			d3 = append(d3, m)
		}
	}
	require.Len(t, d3, 2)
	require.Equal(t, d3[0].IdempotencyKey, d3[1].IdempotencyKey)
	require.Equal(t, 2, d3[1].Attempt)
	require.Len(t, c.push.SentTo("tok-d3"), 1)
	require.False(t, c.push.CalledInTransaction())
}

func TestNotificationRecipients_UserWithoutDeviceOrAddressIsVisible(t *testing.T) {
	c := setupRecipients(t, fakeAddresses{notifications.ChannelEmail: "first@example.com"})
	id := uuid.NewString()
	_, err := c.dispatch.EnqueueExplicit(c.committed, services.AddressedRequest{Event: c.event(id), RecipientIDs: []uint{c.users[0]}, Channels: []notifications.Channel{notifications.ChannelEmail, notifications.ChannelPush, notifications.ChannelSMS}})
	require.NoError(t, err)
	c.drain(t)
	got := byChannel(c.deliveries(t, notifications.TestEventKey, "addressed:"+id))
	require.Equal(t, notifications.DeliverySent, got[notifications.ChannelEmail][0].Status)
	require.Equal(t, notifications.DeliveryNoAddress, got[notifications.ChannelPush][0].Status, "no registered device")
	require.Equal(t, notifications.DeliveryNoAddress, got[notifications.ChannelSMS][0].Status)
	require.Empty(t, got[notifications.ChannelPush][0].TargetAddress)
	stats, err := c.channels.Stats(c.committed)
	require.NoError(t, err)
	var noAddress int64
	for _, s := range stats {
		if s.Status == notifications.DeliveryNoAddress {
			noAddress += s.Count
		}
	}
	require.EqualValues(t, 2, noAddress)

	// Registering a device and retrying the unreachable delivery fixes it.
	require.NoError(t, c.channels.RegisterDevice(c.committed, notifications.PushDevice{UserID: c.users[0], DeviceID: "late", Token: "tok-late", Platform: notifications.PushIOS}))
	require.NoError(t, c.channels.Retry(c.committed, got[notifications.ChannelPush][0].ID))
	_, err = c.channels.Process(c.committed)
	require.NoError(t, err)
	after := byChannel(c.deliveries(t, notifications.TestEventKey, "addressed:"+id))
	require.Len(t, after[notifications.ChannelPush], 1)
	require.Equal(t, notifications.DeliverySent, after[notifications.ChannelPush][0].Status)
	require.Equal(t, "late", after[notifications.ChannelPush][0].TargetDeviceID)
}

func TestNotificationRecipients_RollbackCancelsEnqueueAndCommitKeepsIt(t *testing.T) {
	c := setupRecipients(t, nil)
	c.contacts.Add(customerRef, notifications.Contact{Targets: []notifications.Target{{Channel: notifications.ChannelSMS, Address: "+998900000002"}}})
	request := func(id string) services.AddressedRequest {
		return services.AddressedRequest{Event: c.event(id), Recipients: []notifications.Addressee{notifications.RecordAddressee(customerRef.Entity, customerRef.Key)}, Channels: []notifications.Channel{notifications.ChannelSMS}}
	}
	rolledBack := uuid.NewString()
	tx, err := c.f.Pool.Begin(c.committed)
	require.NoError(t, err)
	_, err = c.dispatch.EnqueueAddressedTx(c.committed, tx, request(rolledBack))
	require.NoError(t, err)
	require.NoError(t, tx.Rollback(c.committed))
	require.Zero(t, c.count(t, "SELECT COUNT(*) FROM core.notification_notices WHERE tenant_id=$1", c.f.TenantID()))
	require.Zero(t, c.count(t, "SELECT COUNT(*) FROM core.notification_dispatches WHERE tenant_id=$1", c.f.TenantID()))
	c.drain(t)
	require.Empty(t, c.sms.Calls(), "a rolled back intent is never delivered")

	committed := uuid.NewString()
	tx, err = c.f.Pool.Begin(c.committed)
	require.NoError(t, err)
	receipt, err := c.dispatch.EnqueueAddressedTx(c.committed, tx, request(committed))
	require.NoError(t, err)
	require.Zero(t, c.count(t, "SELECT COUNT(*) FROM core.notification_dispatches WHERE tenant_id=$1", c.f.TenantID()), "the intent is invisible until the business transaction commits")
	require.NoError(t, tx.Commit(c.committed))
	require.Equal(t, 1, c.count(t, "SELECT COUNT(*) FROM core.notification_notices WHERE tenant_id=$1 AND id=$2", c.f.TenantID(), receipt.NoticeID))
	c.drain(t)
	require.Len(t, c.sms.Sent(), 1)
}

func TestNotificationRecipients_EnqueueRejectsInvalidIntentBeforeCommit(t *testing.T) {
	c := setupRecipients(t, nil)
	record := notifications.RecordAddressee(customerRef.Entity, customerRef.Key)
	tests := map[string]services.AddressedRequest{
		"no resolver for entity":        {Event: c.event(uuid.NewString()), Recipients: []notifications.Addressee{notifications.RecordAddressee("acme.unknown", "1")}, Channels: []notifications.Channel{notifications.ChannelSMS}},
		"record needs external channel": {Event: c.event(uuid.NewString()), Recipients: []notifications.Addressee{record}},
		"unknown notice":                {Event: notifications.Event{Key: "acme.none.v1", ID: uuid.NewString(), TenantID: c.f.TenantID()}, Recipients: []notifications.Addressee{record}, Channels: []notifications.Channel{notifications.ChannelSMS}},
		"invalid address":               {Event: c.event(uuid.NewString()), Recipients: []notifications.Addressee{notifications.AddressAddressee(map[notifications.Channel]string{notifications.ChannelInApp: "x"})}, Channels: []notifications.Channel{notifications.ChannelSMS}},
		"user with record fields":       {Event: c.event(uuid.NewString()), Recipients: []notifications.Addressee{{Kind: notifications.RecipientUser, UserID: 1, Record: &customerRef}}},
		"invalid payload":               {Event: func() notifications.Event { e := c.event(uuid.NewString()); e.Payload = []byte("{not json"); return e }(), Recipients: []notifications.Addressee{record}, Channels: []notifications.Channel{notifications.ChannelSMS}},
		"foreign tenant":                {Event: func() notifications.Event { e := c.event(uuid.NewString()); e.TenantID = uuid.New(); return e }(), Recipients: []notifications.Addressee{record}, Channels: []notifications.Channel{notifications.ChannelSMS}},
	}
	for name, req := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := c.dispatch.EnqueueExplicit(c.committed, req)
			require.Error(t, err)
		})
	}
	require.Zero(t, c.count(t, "SELECT COUNT(*) FROM core.notification_notices WHERE tenant_id=$1", c.f.TenantID()))
}

func TestNotificationRecipients_ExplicitSendIsNotRetargetedByRuleEdits(t *testing.T) {
	c := setupRecipients(t, fakeAddresses{notifications.ChannelEmail: "someone@example.com", notifications.ChannelSMS: "+998900000009"})
	c.contacts.Add(customerRef, notifications.Contact{Targets: []notifications.Target{{Channel: notifications.ChannelSMS, Address: "+998900000003"}}})
	require.NoError(t, c.router.SaveRule(c.committed, notifications.Rule{EventKey: notifications.TestEventKey, Enabled: true, UserIDs: []uint{c.users[0]}, Channels: []notifications.Channel{notifications.ChannelInApp}}))

	id := uuid.NewString()
	_, err := c.dispatch.EnqueueExplicit(c.committed, services.AddressedRequest{
		Event: c.event(id), RecipientIDs: []uint{c.users[1]}, Recipients: []notifications.Addressee{notifications.RecordAddressee(customerRef.Entity, customerRef.Key)},
		Channels: []notifications.Channel{notifications.ChannelInApp, notifications.ChannelSMS},
	})
	require.NoError(t, err)
	// The administrator edits and disables the rule while the send is queued.
	require.NoError(t, c.router.SaveRule(c.committed, notifications.Rule{EventKey: notifications.TestEventKey, Enabled: false, UserIDs: []uint{c.users[0]}, Channels: []notifications.Channel{notifications.ChannelEmail}}))
	c.drain(t)

	got := c.deliveries(t, notifications.TestEventKey, "addressed:"+id)
	recipients := map[string]bool{}
	for _, d := range got {
		recipients[d.RecipientKey] = true
		require.Equal(t, notifications.DeliverySent, d.Status)
	}
	require.Equal(t, map[string]bool{fmt.Sprintf("user:%d", c.users[1]): true, "record:acme.customer/c-1": true}, recipients)
	require.Zero(t, c.count(t, "SELECT COUNT(*) FROM core.notifications WHERE tenant_id=$1 AND user_id=$2", c.f.TenantID(), c.users[0]), "the rule's audience is not added to an explicit send")
	require.Empty(t, c.email.Calls(), "the rule's channels do not replace the explicit ones")
	var mode string
	require.NoError(t, c.f.Pool.QueryRow(c.committed, "SELECT mode FROM core.notification_notices WHERE tenant_id=$1 AND event_id=$2", c.f.TenantID(), "addressed:"+id).Scan(&mode))
	require.Equal(t, "explicit", mode)

	// Rule mode keeps its own semantics: the disabled rule publishes nothing.
	n, err := c.dispatch.Enqueue(c.committed, c.event(uuid.NewString()))
	require.NoError(t, err)
	require.Zero(t, n)
}

func TestNotificationRecipients_SubjectAccessDeniedIsAVisibleSkip(t *testing.T) {
	c := setupRecipients(t, fakeAddresses{notifications.ChannelEmail: "user@example.com"})
	c.contacts.Add(customerRef, notifications.Contact{Targets: []notifications.Target{{Channel: notifications.ChannelEmail, Address: "customer@example.com"}}})
	c.access.Deny(c.users[0])
	id := uuid.NewString()
	_, err := c.dispatch.EnqueueExplicit(c.committed, services.AddressedRequest{
		Event: c.event(id), RecipientIDs: c.users, Recipients: []notifications.Addressee{notifications.RecordAddressee(customerRef.Entity, customerRef.Key)},
		Channels: []notifications.Channel{notifications.ChannelInApp, notifications.ChannelEmail},
	})
	require.NoError(t, err)
	c.drain(t)

	denied, allowed := 0, 0
	for _, d := range c.deliveries(t, notifications.TestEventKey, "addressed:"+id) {
		switch {
		case d.UserID == c.users[0]:
			denied++
			require.Equal(t, notifications.DeliverySkipped, d.Status)
			require.Equal(t, notifications.ErrorCodeSubjectAccessDenied, d.ErrorCode)
			require.Zero(t, d.Attempts)
		case d.UserID == c.users[1] || d.RecipientKind == notifications.RecipientRecord:
			allowed++
			require.Equal(t, notifications.DeliverySent, d.Status)
		}
	}
	require.Equal(t, 2, denied, "in-app and email are both recorded as skipped for the denied user")
	require.Equal(t, 3, allowed)
	require.Zero(t, c.count(t, "SELECT COUNT(*) FROM core.notifications WHERE tenant_id=$1 AND user_id=$2", c.f.TenantID(), c.users[0]), "no inbox row for the denied user")
	require.Equal(t, 1, c.count(t, "SELECT COUNT(*) FROM core.notifications WHERE tenant_id=$1 AND user_id=$2", c.f.TenantID(), c.users[1]))
	require.Len(t, c.email.Sent(), 2, "the allowed user and the customer; never the denied user")
	require.Len(t, c.access.Checks(), 2, "only user recipients are checked, never the external contact")
}

type pinnedSource struct {
	snapshot string
	text     string
}

func (s pinnedSource) Name() string { return "ts-runtime" }
func (s pinnedSource) Definitions(context.Context) ([]notifications.Definition, error) {
	text := s.text
	return []notifications.Definition{{
		Module: "acme", Key: "acme.insurance.renewal.v1", Source: "ts-runtime", SnapshotRef: s.snapshot,
		Name: map[string]string{"en": "Renewal"},
		Render: func(e notifications.Event, _ string) (notifications.Content, error) {
			return notifications.Content{Title: "Renewal " + text, Body: text + " for " + e.Data["policy"]}, nil
		},
	}}, nil
}

func TestNotificationRecipients_QueuedDeliveryUsesPinnedDefinitionAfterUpdate(t *testing.T) {
	c := setupRecipients(t, nil)
	c.contacts.Add(customerRef, notifications.Contact{Targets: []notifications.Target{{Channel: notifications.ChannelEmail, Address: "customer@example.com"}}})
	require.NoError(t, c.catalog.RegisterSource(c.committed, pinnedSource{snapshot: "snap-1", text: "version one"}))

	id := uuid.NewString()
	_, err := c.dispatch.EnqueueExplicit(c.committed, services.AddressedRequest{
		Event:      notifications.Event{Key: "acme.insurance.renewal.v1", ID: id, TenantID: c.f.TenantID(), SnapshotRef: "snap-1", Data: map[string]string{"policy": "P-1"}},
		Recipients: []notifications.Addressee{notifications.RecordAddressee(customerRef.Entity, customerRef.Key)},
		Channels:   []notifications.Channel{notifications.ChannelEmail},
	})
	require.NoError(t, err)
	worked, err := c.dispatch.Process(c.committed)
	require.NoError(t, err)
	require.True(t, worked)

	// The extension is updated before the delivery is rendered: the source is
	// deactivated and a new snapshot registers new text under the same key.
	c.catalog.Deactivate("ts-runtime")
	_, active := c.catalog.Get("acme.insurance.renewal.v1")
	require.False(t, active)
	require.NoError(t, c.catalog.RegisterSource(c.committed, pinnedSource{snapshot: "snap-2", text: "version two"}))

	_, err = c.channels.Process(c.committed)
	require.NoError(t, err)
	require.Len(t, c.email.Sent(), 1)
	sent := c.email.Sent()[0]
	require.Equal(t, "Renewal version one", sent.EmailSubject, "the queued delivery renders with its pinned snapshot")
	require.Contains(t, sent.Text, "version one for P-1")
	require.Equal(t, "snap-1", sent.SnapshotRef)
	require.Equal(t, 1, sent.ContentVersion)

	// A notice enqueued against the new snapshot renders the new text.
	next := uuid.NewString()
	_, err = c.dispatch.EnqueueExplicit(c.committed, services.AddressedRequest{
		Event:      notifications.Event{Key: "acme.insurance.renewal.v1", ID: next, TenantID: c.f.TenantID(), SnapshotRef: "snap-2", Data: map[string]string{"policy": "P-2"}},
		Recipients: []notifications.Addressee{notifications.RecordAddressee(customerRef.Entity, customerRef.Key)},
		Channels:   []notifications.Channel{notifications.ChannelEmail},
	})
	require.NoError(t, err)
	c.drain(t)
	require.Len(t, c.email.Sent(), 2)
	require.Equal(t, "Renewal version two", c.email.Sent()[1].EmailSubject)
}

func TestNotificationRecipients_PluggedRendererReturnsPushPayloadAndDeactivationKeepsIt(t *testing.T) {
	c := setupRecipients(t, nil)
	c.contacts.Add(customerRef, notifications.Contact{Targets: []notifications.Target{{Channel: notifications.ChannelPush, Address: "tok-customer", DeviceID: "phone-1"}, {Channel: notifications.ChannelPush, Address: "tok-tablet", DeviceID: "tablet-1"}}})
	require.NoError(t, c.catalog.RegisterSource(c.committed, pinnedSource{snapshot: "snap-ts", text: "native fallback"}))
	require.NoError(t, c.renderers.Register("acme.insurance.renewal.v1", "snap-ts", notifications.ContentRendererFunc(func(_ context.Context, r notifications.RenderRequest) (notifications.ChannelContent, error) {
		require.Equal(t, notifications.ChannelPush, r.Channel)
		return notifications.ChannelContent{Title: "Policy renewal", Body: "Tap to renew " + string(r.Event.Payload), ActionURL: "/policies/p-1", Data: map[string]string{"policy": "p-1", "kind": "renewal"}}, nil
	})))
	require.Error(t, c.renderers.Register("acme.insurance.renewal.v1", "snap-ts", notifications.ContentRendererFunc(nil)), "a pinned renderer is never silently replaced")

	_, err := c.dispatch.EnqueueExplicit(c.committed, services.AddressedRequest{
		Event:      notifications.Event{Key: "acme.insurance.renewal.v1", ID: uuid.NewString(), TenantID: c.f.TenantID(), SnapshotRef: "snap-ts", Payload: []byte(`{"policy":"p-1"}`)},
		Recipients: []notifications.Addressee{notifications.RecordAddressee(customerRef.Entity, customerRef.Key)},
		Channels:   []notifications.Channel{notifications.ChannelPush},
	})
	require.NoError(t, err)
	c.drain(t)
	require.Len(t, c.push.Sent(), 2, "an external recipient's devices come from the contact resolver, one delivery each")
	m := c.push.SentTo("tok-customer")[0]
	require.Equal(t, "Policy renewal", m.Title)
	require.JSONEq(t, `{"policy":"p-1"}`, strings.TrimPrefix(m.Body, "Tap to renew "))
	require.Equal(t, map[string]string{"policy": "p-1", "kind": "renewal"}, m.Data)
	require.Equal(t, "phone-1", m.DeviceID)
	require.Equal(t, "/policies/p-1", m.ActionURL)
}

func TestNotificationRecipients_ProducerDedupeAndIdentityArePersisted(t *testing.T) {
	c := setupRecipients(t, nil)
	c.contacts.Add(customerRef, notifications.Contact{Targets: []notifications.Target{{Channel: notifications.ChannelSMS, Address: "+998900000004"}}})
	producer := notifications.Producer{InstanceID: "wf-1", Occurrence: "remind#2"}
	send := func(recipient notifications.Addressee) services.AddressedReceipt {
		event := c.event(uuid.NewString()) // a fresh event ID per replay: the producer identity dedupes
		tx, err := c.f.Pool.Begin(c.committed)
		require.NoError(t, err)
		receipt, err := c.dispatch.EnqueueAddressedTx(c.committed, tx, services.AddressedRequest{Event: event, Recipients: []notifications.Addressee{recipient}, Channels: []notifications.Channel{notifications.ChannelSMS}, Producer: producer})
		require.NoError(t, err)
		require.NoError(t, tx.Commit(c.committed))
		return receipt
	}
	first := send(notifications.RecordAddressee(customerRef.Entity, customerRef.Key))
	replay := send(notifications.RecordAddressee(customerRef.Entity, customerRef.Key))
	require.Equal(t, first.NoticeID, replay.NoticeID, "a replayed producer step returns the same notice")
	other := send(notifications.AddressAddressee(map[notifications.Channel]string{notifications.ChannelSMS: "+998900000005"}))
	require.NotEqual(t, first.NoticeID, other.NoticeID, "a different recipient set from the same producer is its own notice")
	require.Equal(t, 2, c.count(t, "SELECT COUNT(*) FROM core.notification_dispatches WHERE tenant_id=$1", c.f.TenantID()))

	var instance, occurrence, entity, key, mode, snapshot string
	require.NoError(t, c.f.Pool.QueryRow(c.committed, "SELECT producer_instance_id, producer_occurrence, subject_entity, subject_key, mode, snapshot_ref FROM core.notification_notices WHERE tenant_id=$1 AND id=$2", c.f.TenantID(), first.NoticeID).Scan(&instance, &occurrence, &entity, &key, &mode, &snapshot))
	require.Equal(t, []string{"wf-1", "remind#2", "acme.policy", "p-1", "explicit", ""}, []string{instance, occurrence, entity, key, mode, snapshot})
	c.drain(t)
	require.Len(t, c.sms.Sent(), 2, "two notices, one per distinct intent")
}

func TestNotificationRecipients_UnknownContactAndResolverFailure(t *testing.T) {
	c := setupRecipients(t, nil)
	id := uuid.NewString()
	_, err := c.dispatch.EnqueueExplicit(c.committed, services.AddressedRequest{Event: c.event(id), Recipients: []notifications.Addressee{notifications.RecordAddressee(customerRef.Entity, "missing")}, Channels: []notifications.Channel{notifications.ChannelSMS}})
	require.NoError(t, err)
	c.drain(t)
	got := c.deliveries(t, notifications.TestEventKey, "addressed:"+id)
	require.Len(t, got, 1)
	require.Equal(t, notifications.DeliveryNoAddress, got[0].Status)
	require.Equal(t, notifications.ErrorCodeContactNotFound, got[0].ErrorCode)

	c.contacts.Add(customerRef, notifications.Contact{Targets: []notifications.Target{{Channel: notifications.ChannelSMS, Address: "+998900000006"}}})
	c.contacts.FailWith(errors.New("crm unavailable"))
	flaky := uuid.NewString()
	_, err = c.dispatch.EnqueueExplicit(c.committed, services.AddressedRequest{Event: c.event(flaky), Recipients: []notifications.Addressee{notifications.RecordAddressee(customerRef.Entity, customerRef.Key)}, Channels: []notifications.Channel{notifications.ChannelSMS}})
	require.NoError(t, err)
	c.drain(t)
	got = c.deliveries(t, notifications.TestEventKey, "addressed:"+flaky)
	require.Equal(t, notifications.DeliveryUnresolved, got[0].Status, "a resolver outage retries; it is not a delivery failure")
	require.Equal(t, notifications.ErrorCodeResolutionFailed, got[0].ErrorCode)
	require.NotContains(t, got[0].ErrorCode, "crm", "resolver error text is never stored")
	require.Empty(t, c.sms.Calls())

	c.contacts.FailWith(nil)
	c.backoffElapsed(t)
	_, err = c.channels.Process(c.committed)
	require.NoError(t, err)
	got = c.deliveries(t, notifications.TestEventKey, "addressed:"+flaky)
	require.Len(t, got, 1)
	require.Equal(t, notifications.DeliverySent, got[0].Status)
}

func TestNotificationRecipients_TenantIsolationOfDeliveriesAndDevices(t *testing.T) {
	c := setupRecipients(t, nil)
	require.NoError(t, c.channels.RegisterDevice(c.committed, notifications.PushDevice{UserID: c.users[0], DeviceID: "d1", Token: "tok-shared", Platform: notifications.PushWeb}))
	// A token moves to the account that registers it last.
	require.NoError(t, c.channels.RegisterDevice(c.committed, notifications.PushDevice{UserID: c.users[1], DeviceID: "d9", Token: "tok-shared", Platform: notifications.PushWeb}))
	devices := persistence.NewNotificationPushDeviceRepository()
	first, err := devices.Active(c.committed, c.users[0])
	require.NoError(t, err)
	require.Empty(t, first)
	second, err := devices.Active(c.committed, c.users[1])
	require.NoError(t, err)
	require.Len(t, second, 1)
	foreign, err := devices.Active(composables.WithTenantID(c.committed, uuid.New()), c.users[1])
	require.NoError(t, err)
	require.Empty(t, foreign, "devices are tenant-scoped")
	require.NoError(t, c.channels.RevokeDevice(c.committed, c.users[1], "d9"))
	second, err = devices.Active(c.committed, c.users[1])
	require.NoError(t, err)
	require.Empty(t, second)
	require.Error(t, c.channels.RegisterDevice(c.committed, notifications.PushDevice{UserID: c.users[0], DeviceID: "x", Token: "t", Platform: "carrier-pigeon"}))
}
