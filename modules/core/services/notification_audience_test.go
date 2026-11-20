package services_test

import (
	"context"
	"fmt"
	"strconv"
	"testing"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/internet"
	"github.com/iota-uz/iota-sdk/modules/core/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/modules/core/notifications"
	"github.com/iota-uz/iota-sdk/modules/core/services"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/composition"
	"github.com/iota-uz/iota-sdk/pkg/itf"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
	"github.com/stretchr/testify/require"
)

const participantEventKey = "acme.policy.created.v1"

// audienceFixture adds people, a group, a role and a domain event whose
// "created_user" participant is read from the event data.
type audienceFixture struct {
	*recipientsFixture
	people []uint
	sender notifications.AudienceSender
}

func setupAudience(t *testing.T, people int) *audienceFixture {
	t.Helper()
	c := &audienceFixture{recipientsFixture: setupRecipients(t, nil)}
	sender, err := composition.Resolve[notifications.AudienceSender](c.f.Container)
	require.NoError(t, err)
	c.sender = sender
	repo := persistence.NewUserRepository(persistence.NewUploadRepository())
	for range people {
		u, err := repo.Create(c.committed, user.New("Audience", "Member", internet.MustParseEmail(fmt.Sprintf("audience%s@example.com", uuid.NewString()[:8])), user.UILanguageEN, user.WithTenantID(c.f.TenantID())))
		require.NoError(t, err)
		c.people = append(c.people, u.ID())
	}
	d := notifications.TestDefinition()
	d.Key = participantEventKey
	d.RecipientKeys = []notifications.RecipientDefinition{{Key: "created_user", Name: map[string]string{"en": "Creator"}, Resolve: func(_ context.Context, e notifications.Event) ([]uint, error) {
		id, err := strconv.ParseUint(e.Data["created_user"], 10, 64)
		if err != nil {
			return nil, err
		}
		return []uint{uint(id)}, nil
	}}}
	require.NoError(t, c.catalog.Register(d))
	return c
}

func (c *audienceFixture) group(t *testing.T, tenant uuid.UUID, members ...uint) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	require.NoError(t, c.f.Pool.QueryRow(c.committed, "INSERT INTO user_groups (name, tenant_id, type) VALUES ($1, $2, 'user') RETURNING id", "Audience "+uuid.NewString(), tenant).Scan(&id))
	for _, member := range members {
		_, err := c.f.Pool.Exec(c.committed, "INSERT INTO group_users (group_id, user_id) VALUES ($1, $2)", id, member)
		require.NoError(t, err)
	}
	return id
}

func (c *audienceFixture) role(t *testing.T, members ...uint) uint {
	t.Helper()
	var id uint
	require.NoError(t, c.f.Pool.QueryRow(c.committed, "INSERT INTO roles (name, tenant_id, type) VALUES ($1, $2, 'user') RETURNING id", "Audience "+uuid.NewString(), c.f.TenantID()).Scan(&id))
	for _, member := range members {
		_, err := c.f.Pool.Exec(c.committed, "INSERT INTO user_roles (user_id, role_id) VALUES ($1, $2)", member, id)
		require.NoError(t, err)
	}
	return id
}

func (c *audienceFixture) source(creator uint) *notifications.Event {
	return &notifications.Event{Key: participantEventKey, ID: uuid.NewString(), TenantID: c.f.TenantID(), Data: map[string]string{"created_user": strconv.FormatUint(uint64(creator), 10)}}
}

// send runs SendTx in its own committed transaction.
func (c *audienceFixture) send(t *testing.T, req notifications.AudienceRequest) (notifications.AudienceReceipt, error) {
	t.Helper()
	tx, err := c.f.Pool.Begin(c.committed)
	require.NoError(t, err)
	receipt, err := c.sender.SendTx(c.committed, tx, req)
	if err != nil {
		require.NoError(t, tx.Rollback(c.committed))
		return receipt, err
	}
	require.NoError(t, tx.Commit(c.committed))
	return receipt, nil
}

func (c *audienceFixture) written(t *testing.T) (int, int) {
	t.Helper()
	return c.count(t, "SELECT COUNT(*) FROM core.notification_notices WHERE tenant_id=$1", c.f.TenantID()),
		c.count(t, "SELECT COUNT(*) FROM core.notification_dispatches WHERE tenant_id=$1", c.f.TenantID())
}

func TestNotificationAudience_GroupsRolesAndParticipantsAreUnique(t *testing.T) {
	c := setupAudience(t, 5)
	p := c.people
	audience := notifications.Audience{
		Groups:       []uuid.UUID{c.group(t, c.f.TenantID(), p[0], p[1], p[2])},
		Roles:        []uint{c.role(t, p[2], p[3])},
		Participants: []string{"created_user"},
	}
	req := notifications.AudienceRequest{Event: c.event(uuid.NewString()), Audience: audience, Source: c.source(p[4])}

	resolved, err := c.sender.Resolve(c.committed, req)
	require.NoError(t, err)
	require.ElementsMatch(t, p, resolved.UserIDs)
	require.Empty(t, resolved.Skipped)
	notices, jobs := c.written(t)
	require.Zero(t, notices+jobs, "a dry run writes nothing")

	receipt, err := c.send(t, req)
	require.NoError(t, err)
	require.Equal(t, 5, receipt.Recipients)
	var stored int
	require.NoError(t, c.f.Pool.QueryRow(c.committed, "SELECT jsonb_array_length(recipient_ids) FROM core.notification_dispatches WHERE tenant_id=$1", c.f.TenantID()).Scan(&stored))
	require.Equal(t, 5, stored)
	c.drain(t)
	require.Equal(t, 5, c.count(t, "SELECT COUNT(*) FROM core.notifications WHERE tenant_id=$1", c.f.TenantID()))
}

func TestNotificationAudience_SubjectAccessIsCheckedPerRecipientAtDelivery(t *testing.T) {
	c := setupAudience(t, 3)
	id := uuid.NewString()
	_, err := c.send(t, notifications.AudienceRequest{Event: c.event(id), Audience: notifications.Audience{Groups: []uuid.UUID{c.group(t, c.f.TenantID(), c.people...)}}})
	require.NoError(t, err)
	// Access is lost after the send and before delivery.
	c.access.Deny(c.people[0])
	c.drain(t)
	statuses := map[uint]notifications.Delivery{}
	for _, d := range c.deliveries(t, notifications.TestEventKey, "addressed:"+id) {
		statuses[d.UserID] = d
	}
	require.Len(t, statuses, 3)
	require.Equal(t, notifications.DeliverySkipped, statuses[c.people[0]].Status)
	require.Equal(t, notifications.ErrorCodeSubjectAccessDenied, statuses[c.people[0]].ErrorCode)
	for _, u := range c.people[1:] {
		require.Equal(t, notifications.DeliverySent, statuses[u].Status)
	}
}

func TestNotificationAudience_RollbackLeavesNothing(t *testing.T) {
	c := setupAudience(t, 2)
	role := c.role(t, c.people...)
	tx, err := c.f.Pool.Begin(c.committed)
	require.NoError(t, err)
	receipt, err := c.sender.SendTx(c.committed, tx, notifications.AudienceRequest{Event: c.event(uuid.NewString()), Audience: notifications.Audience{Roles: []uint{role}}})
	require.NoError(t, err)
	require.Equal(t, 2, receipt.Recipients)
	require.NoError(t, tx.Rollback(c.committed))
	notices, jobs := c.written(t)
	require.Zero(t, notices)
	require.Zero(t, jobs)
}

func TestNotificationAudience_ForeignTenantGroupIsSkipped(t *testing.T) {
	c := setupAudience(t, 0)
	other := uuid.New()
	_, err := c.f.Pool.Exec(c.committed, "INSERT INTO tenants (id, name) VALUES ($1, $2)", other, "Other "+other.String())
	require.NoError(t, err)
	foreign := c.group(t, other)
	req := notifications.AudienceRequest{Event: c.event(uuid.NewString()), Audience: notifications.Audience{Groups: []uuid.UUID{foreign}, Roles: []uint{999999999}, Users: []uint{999999999}}}

	resolved, err := c.sender.Resolve(c.committed, req)
	require.NoError(t, err)
	require.Empty(t, resolved.UserIDs)
	require.ElementsMatch(t, []notifications.SkippedRecipient{
		{Kind: notifications.RecipientGroup, Ref: foreign.String(), Reason: notifications.SkipNotFound},
		{Kind: notifications.RecipientRole, Ref: "999999999", Reason: notifications.SkipNotFound},
		{Kind: notifications.RecipientUser, Ref: "999999999", Reason: notifications.SkipNotFound},
	}, resolved.Skipped)
	receipt, err := c.send(t, req)
	require.NoError(t, err)
	require.Zero(t, receipt.Recipients)
	notices, jobs := c.written(t)
	require.Zero(t, notices+jobs)
}

func TestNotificationAudience_LimitIsCountedAfterResolution(t *testing.T) {
	c := setupAudience(t, 3)
	group := c.group(t, c.f.TenantID(), c.people...)
	addresses := make([]map[notifications.Channel]string, 0, services.MaxExplicitRecipients)
	for i := range services.MaxExplicitRecipients - 2 {
		addresses = append(addresses, map[notifications.Channel]string{notifications.ChannelSMS: fmt.Sprintf("+99890%07d", i)})
	}
	// Duplicates do not count: the group's members and a repeated address.
	audience := notifications.Audience{Groups: []uuid.UUID{group}, Users: c.people[:1], Addresses: append(addresses, addresses[0])}
	channels := []notifications.Channel{notifications.ChannelInApp, notifications.ChannelSMS}
	resolved, err := c.sender.Resolve(c.committed, notifications.AudienceRequest{Event: c.event(uuid.NewString()), Audience: audience, Channels: channels})
	require.NoError(t, err)
	require.Equal(t, services.MaxExplicitRecipients+1, resolved.Total())

	_, err = c.send(t, notifications.AudienceRequest{Event: c.event(uuid.NewString()), Audience: audience, Channels: channels})
	require.Error(t, err)
	require.Equal(t, serrors.Invalid, serrors.CodeOf(err))
	notices, jobs := c.written(t)
	require.Zero(t, notices+jobs, "the whole send is rejected, recipients are never truncated")

	audience.Addresses = addresses[1:]
	receipt, err := c.send(t, notifications.AudienceRequest{Event: c.event(uuid.NewString()), Audience: audience, Channels: channels})
	require.NoError(t, err)
	require.Equal(t, services.MaxExplicitRecipients, receipt.Recipients)
}

func TestNotificationAudience_ReplayedProducerReturnsTheFirstNotice(t *testing.T) {
	c := setupAudience(t, 2)
	group := c.group(t, c.f.TenantID(), c.people...)
	producer := notifications.Producer{InstanceID: "wf-audience", Occurrence: "notify#1"}
	send := func() notifications.AudienceReceipt {
		receipt, err := c.send(t, notifications.AudienceRequest{Event: c.event(uuid.NewString()), Audience: notifications.Audience{Groups: []uuid.UUID{group}}, Producer: producer})
		require.NoError(t, err)
		return receipt
	}
	first := send()
	replay := send()
	require.Equal(t, first.NoticeID, replay.NoticeID)
	require.Equal(t, 2, replay.Recipients)
	_, jobs := c.written(t)
	require.Equal(t, 1, jobs)

	// The group is emptied between the first send and the replay.
	_, err := c.f.Pool.Exec(c.committed, "DELETE FROM group_users WHERE group_id=$1", group)
	require.NoError(t, err)
	emptied := send()
	require.Equal(t, first.NoticeID, emptied.NoticeID, "a replay returns the first notice after membership changed")
	require.Equal(t, 2, emptied.Recipients, "the receipt reports the accepted job, not the new membership")
	_, jobs = c.written(t)
	require.Equal(t, 1, jobs)
}

func TestNotificationAudience_ParticipantsAreFilteredToTheTenant(t *testing.T) {
	c := setupAudience(t, 1)
	other := uuid.New()
	_, err := c.f.Pool.Exec(c.committed, "INSERT INTO tenants (id, name) VALUES ($1, $2)", other, "Other "+other.String())
	require.NoError(t, err)
	repo := persistence.NewUserRepository(persistence.NewUploadRepository())
	foreign, err := repo.Create(composables.WithTenantID(c.committed, other), user.New("Foreign", "User", internet.MustParseEmail(fmt.Sprintf("foreign%s@example.com", uuid.NewString()[:8])), user.UILanguageEN, user.WithTenantID(other)))
	require.NoError(t, err)
	for _, id := range []uint{foreign.ID(), 999999999} {
		req := notifications.AudienceRequest{Event: c.event(uuid.NewString()), Audience: notifications.Audience{Users: c.people, Participants: []string{"created_user"}}, Source: c.source(id)}
		resolved, err := c.sender.Resolve(c.committed, req)
		require.NoError(t, err)
		require.Equal(t, c.people, resolved.UserIDs)
		require.Equal(t, []notifications.SkippedRecipient{{Kind: notifications.RecipientUser, Ref: strconv.FormatUint(uint64(id), 10), Reason: notifications.SkipNotFound}}, resolved.Skipped)
		receipt, err := c.send(t, req)
		require.NoError(t, err)
		require.Equal(t, 1, receipt.Recipients)
	}
}

func TestNotificationAudience_RejectsInvalidRequestsBeforeWriting(t *testing.T) {
	c := setupAudience(t, 1)
	tests := map[string]struct {
		req  notifications.AudienceRequest
		code serrors.Code
	}{
		"unknown participant":                 {notifications.AudienceRequest{Event: c.event(uuid.NewString()), Audience: notifications.Audience{Participants: []string{"nobody"}}, Source: c.source(c.people[0])}, serrors.Invalid},
		"participants without source":         {notifications.AudienceRequest{Event: c.event(uuid.NewString()), Audience: notifications.Audience{Participants: []string{"created_user"}}}, serrors.Invalid},
		"foreign source without participants": {notifications.AudienceRequest{Event: c.event(uuid.NewString()), Audience: notifications.Audience{Users: c.people}, Source: func() *notifications.Event { e := c.source(c.people[0]); e.TenantID = uuid.New(); return e }()}, serrors.Invalid},
		"foreign source":                      {notifications.AudienceRequest{Event: c.event(uuid.NewString()), Audience: notifications.Audience{Participants: []string{"created_user"}}, Source: func() *notifications.Event { e := c.source(c.people[0]); e.TenantID = uuid.New(); return e }()}, serrors.Invalid},
		"unknown notice":                      {notifications.AudienceRequest{Event: notifications.Event{Key: "acme.none.v1", ID: uuid.NewString(), TenantID: c.f.TenantID()}, Audience: notifications.Audience{Users: c.people}}, serrors.Invalid},
		"address without external":            {notifications.AudienceRequest{Event: c.event(uuid.NewString()), Audience: notifications.Audience{Addresses: []map[notifications.Channel]string{{notifications.ChannelSMS: "+998900000001"}}}}, serrors.Invalid},
		"invalid address":                     {notifications.AudienceRequest{Event: c.event(uuid.NewString()), Audience: notifications.Audience{Addresses: []map[notifications.Channel]string{{notifications.ChannelInApp: "x"}}}, Channels: []notifications.Channel{notifications.ChannelSMS}}, serrors.Invalid},
		"record without resolver":             {notifications.AudienceRequest{Event: c.event(uuid.NewString()), Audience: notifications.Audience{Records: []notifications.ObjectRef{{Entity: "acme.unknown", Key: "1"}}}, Channels: []notifications.Channel{notifications.ChannelSMS}}, serrors.Invalid},
		"unavailable channel":                 {notifications.AudienceRequest{Event: c.event(uuid.NewString()), Audience: notifications.Audience{Users: c.people}, Channels: []notifications.Channel{"telegram"}}, serrors.Invalid},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := c.send(t, tc.req)
			require.Error(t, err)
			require.Equal(t, tc.code, serrors.CodeOf(err))
		})
	}
	notices, jobs := c.written(t)
	require.Zero(t, notices+jobs)
}

// Rule mode and audience sends share the resolver but not the dedupe space:
// an audience send of the same event is not swallowed by the rule's job.
func TestNotificationAudience_IsIsolatedFromRules(t *testing.T) {
	c := setupAudience(t, 2)
	require.NoError(t, c.router.SaveRule(c.committed, notifications.Rule{EventKey: notifications.TestEventKey, Enabled: true, GroupIDs: []uuid.UUID{c.group(t, c.f.TenantID(), c.people[0])}}))
	event := c.event(uuid.NewString())
	n, err := c.dispatch.Enqueue(c.committed, event)
	require.NoError(t, err)
	require.Equal(t, 1, n)
	receipt, err := c.send(t, notifications.AudienceRequest{Event: event, Audience: notifications.Audience{Users: c.people[1:]}})
	require.NoError(t, err)
	require.Equal(t, 1, receipt.Recipients)
	rule, err := c.router.Rule(c.committed, notifications.TestEventKey)
	require.NoError(t, err)
	require.Len(t, rule.GroupIDs, 1, "the send does not change the rule")
	require.Empty(t, rule.UserIDs)
}

func TestNotificationAudience_UnavailableChannelIsAFailedPrecondition(t *testing.T) {
	f := setupNotificationTest(t)
	sender, err := composition.Resolve[notifications.AudienceSender](f.Container)
	require.NoError(t, err)
	_, err = sender.SendTx(f.Ctx, f.Tx, notifications.AudienceRequest{
		Event:    notifications.Event{Key: notifications.TestEventKey, ID: uuid.NewString(), TenantID: f.TenantID()},
		Audience: notifications.Audience{Users: []uint{f.User.ID()}}, Channels: []notifications.Channel{notifications.ChannelEmail},
	})
	require.Error(t, err)
	require.Equal(t, serrors.FailedPrecondition, serrors.CodeOf(err))
}

// Notices and jobs are unique per event key and identity, not per mode: rule
// mode may not take the addressed namespace, and a legacy rule job holding it
// is never reported as an audience send's receipt.
func TestNotificationAudience_RuleModeCannotTakeTheAddressedIdentity(t *testing.T) {
	c := setupAudience(t, 2)
	require.NoError(t, c.router.SaveRule(c.committed, notifications.Rule{EventKey: notifications.TestEventKey, Enabled: true, UserIDs: c.people[:1]}))
	rule := c.event(uuid.NewString())
	rule.DedupeKey = "addressed:same"
	_, err := c.dispatch.Enqueue(c.committed, rule)
	require.Error(t, err)
	require.Equal(t, serrors.Invalid, serrors.CodeOf(err))

	send := c.event(uuid.NewString())
	send.DedupeKey = "same"
	receipt, err := c.send(t, notifications.AudienceRequest{Event: send, Audience: notifications.Audience{Users: c.people}})
	require.NoError(t, err)
	require.Equal(t, 2, receipt.Recipients)

	legacy := c.event(uuid.NewString())
	legacy.DedupeKey = "addressed:legacy"
	_, err = persistence.NewNotificationDispatchRepository().Enqueue(c.committed, notifications.DispatchRequest{Event: legacy, Rule: notifications.Rule{EventKey: legacy.Key, Enabled: true}, Mode: notifications.NoticeRule, RecipientIDs: c.people[:1]})
	require.NoError(t, err)
	send.DedupeKey = "legacy"
	_, err = c.send(t, notifications.AudienceRequest{Event: send, Audience: notifications.Audience{Users: c.people}})
	require.Error(t, err)
	require.Equal(t, serrors.Conflict, serrors.CodeOf(err))

	// The worker keeps delivering the rule job accepted before the upgrade.
	c.drain(t)
	require.Zero(t, c.count(t, "SELECT COUNT(*) FROM core.notification_dispatches WHERE tenant_id=$1 AND (completed_at IS NULL OR attempts>0)", c.f.TenantID()))
	require.Equal(t, 1, c.count(t, "SELECT COUNT(*) FROM core.notification_notices WHERE tenant_id=$1 AND event_id='addressed:legacy' AND mode='rule'", c.f.TenantID()))
}

// legacyDispatchRepository implements only DispatchRepository, as SDK
// repositories written before DispatchFinder do.
type legacyDispatchRepository struct {
	notifications.DispatchRepository
}

func TestNotificationAudience_RepositoryWithoutFinder(t *testing.T) {
	c := setupAudience(t, 1)
	legacy := services.NewNotificationDispatchService(c.router, legacyDispatchRepository{persistence.NewNotificationDispatchRepository()}, itf.GetService[services.NotificationService](c.f))
	tx, err := c.f.Pool.Begin(c.committed)
	require.NoError(t, err)
	defer func() { _ = tx.Rollback(c.committed) }()
	_, err = legacy.SendTx(c.committed, tx, notifications.AudienceRequest{Event: c.event(uuid.NewString()), Audience: notifications.Audience{Users: c.people}})
	require.Error(t, err)
	require.Equal(t, serrors.FailedPrecondition, serrors.CodeOf(err), "audience sends cannot promise idempotent replays without a lookup")
	receipt, err := legacy.EnqueueAddressedTx(c.committed, tx, services.AddressedRequest{Event: c.event(uuid.NewString()), RecipientIDs: c.people})
	require.NoError(t, err, "existing explicit callers keep working")
	require.Equal(t, 1, receipt.Recipients)
}

// The stored job is looked up before participants are resolved again, so a
// participant whose record is gone does not hide the first receipt.
func TestNotificationAudience_ReplayDoesNotResolveParticipantsAgain(t *testing.T) {
	c := setupAudience(t, 1)
	producer := notifications.Producer{InvocationID: "run-participant"}
	source := c.source(c.people[0])
	first, err := c.send(t, notifications.AudienceRequest{Event: c.event(uuid.NewString()), Audience: notifications.Audience{Participants: []string{"created_user"}}, Source: source, Producer: producer})
	require.NoError(t, err)
	require.Equal(t, 1, first.Recipients)

	gone := *source
	gone.Data = map[string]string{"created_user": "deleted"} // the callback now fails
	replay, err := c.send(t, notifications.AudienceRequest{Event: c.event(uuid.NewString()), Audience: notifications.Audience{Participants: []string{"created_user"}}, Source: &gone, Producer: producer})
	require.NoError(t, err)
	require.Equal(t, first, replay)
	_, err = c.sender.Resolve(c.committed, notifications.AudienceRequest{Event: c.event(uuid.NewString()), Audience: notifications.Audience{Participants: []string{"created_user"}}, Source: &gone, Producer: producer})
	require.Error(t, err, "a dry run still resolves and reports the failure")
}
