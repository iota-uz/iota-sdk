package services_test

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/modules/core/notifications"
	"github.com/iota-uz/iota-sdk/pkg/automation/builtin/notify"
	"github.com/iota-uz/iota-sdk/pkg/automation/catalog"
	"github.com/iota-uz/iota-sdk/pkg/automation/engine"
	"github.com/iota-uz/iota-sdk/pkg/automation/expr"
	"github.com/iota-uz/iota-sdk/pkg/automation/graph"
	"github.com/iota-uz/iota-sdk/pkg/composition"
	"github.com/iota-uz/iota-sdk/pkg/itf"
)

// automationFixture runs the notify nodes of automation graphs against the real delivery layer: the
// notice is queued in the transaction of the step, delivered by the workers after it commits.
type automationFixture struct {
	*audienceFixture
	run engine.StepContext
}

func setupAutomation(t *testing.T) *automationFixture {
	t.Helper()
	c := &automationFixture{audienceFixture: setupAudience(t, 3)}
	// The automations module of the composition has registered automations.notify.v1.
	_, ok := c.catalog.Get(notify.EventKey)
	require.True(t, ok)
	c.run = engine.StepContext{
		TenantID: c.f.TenantID(), RunID: uuid.New(), TokenID: uuid.New(), Seq: 1, NodeID: "ping",
		Subject: engine.SubjectRef{Entity: "acme.policy", Key: "p-1"},
	}
	return c
}

func (c *automationFixture) node(t *testing.T, typ string) catalog.NodeType {
	t.Helper()
	cfg := notify.Config{Sender: c.sender, Catalog: c.catalog, Contacts: itf.GetService[notifications.ContactResolvers](c.f), Pool: c.f.Pool, BaseURL: "https://erp.example.com"}
	switch typ {
	case notify.NodeNotify:
		return notify.NewNotify(cfg)
	case notify.NodeSendToContact:
		return notify.NewSendToContact(cfg)
	}
	t.Fatalf("no node %s", typ)
	return nil
}

// step applies the node as the engine does: in a transaction of the step, with nothing but the tenant's
// pool-less background context, and commits it.
func (c *automationFixture) step(t *testing.T, nt catalog.NodeType, run engine.StepContext, params map[string]any) (engine.EffectResult, error) {
	t.Helper()
	raw, err := json.Marshal(params)
	require.NoError(t, err)
	var p map[string]any
	require.NoError(t, expr.UnmarshalJSON(raw, &p))
	ctx := context.Background()
	tx, err := c.f.Pool.Begin(ctx)
	require.NoError(t, err)
	res, err := nt.(engine.OutcomeEffectNode).ApplyEffectResult(ctx, tx, run, graph.Node{ID: run.NodeID, Type: nt.Spec().Type}, graph.Effect{Node: nt.Spec().Type, Params: p})
	if err != nil {
		require.NoError(t, tx.Rollback(ctx))
		return res, err
	}
	require.NoError(t, tx.Commit(ctx))
	return res, nil
}

func (c *automationFixture) identity(t *testing.T) string {
	t.Helper()
	var id string
	require.NoError(t, c.f.Pool.QueryRow(c.committed, "SELECT event_id FROM core.notification_notices WHERE tenant_id=$1 AND event_key=$2", c.f.TenantID(), notify.EventKey).Scan(&id))
	return id
}

func TestAutomationNotify_RoleOfThreeDeliversToTwoWhenOneLacksAccessToTheSubject(t *testing.T) {
	c := setupAutomation(t)
	role := c.role(t, c.people...)
	c.access.Deny(c.people[0])

	res, err := c.step(t, c.node(t, notify.NodeNotify), c.run, map[string]any{
		"recipients": map[string]any{"roles": []string{strconv.FormatUint(uint64(role), 10)}},
		"title":      "Policy renewed", "body": "Policy p-1 was renewed", "action_url": "/policies/p-1",
	})
	require.NoError(t, err)
	require.Equal(t, notify.OutcomeSent, res.Outcome)
	require.Equal(t, "3", res.Output["recipients"].(json.Number).String())

	c.drain(t)
	deliveries := c.deliveries(t, notify.EventKey, c.identity(t))
	require.Len(t, deliveries, 3)
	sent := 0
	for _, d := range deliveries {
		if d.UserID == c.people[0] {
			require.Equal(t, notifications.DeliverySkipped, d.Status)
			require.Equal(t, notifications.ErrorCodeSubjectAccessDenied, d.ErrorCode)
			continue
		}
		require.Equal(t, notifications.DeliverySent, d.Status)
		sent++
	}
	require.Equal(t, 2, sent)
	// The graph's text reaches the inbox.
	require.Equal(t, 2, c.count(t, "SELECT COUNT(*) FROM core.notifications WHERE tenant_id=$1 AND title=$2 AND body=$3 AND action_url=$4",
		c.f.TenantID(), "Policy renewed", "Policy p-1 was renewed", "/policies/p-1"))
}

func TestAutomationNotify_ReplayedStepDoesNotDuplicateTheNotice(t *testing.T) {
	c := setupAutomation(t)
	nt := c.node(t, notify.NodeNotify)
	params := map[string]any{"recipients": map[string]any{"users": c.people[:2]}, "title": "Hello", "body": "Once"}

	first, err := c.step(t, nt, c.run, params)
	require.NoError(t, err)
	second, err := c.step(t, nt, c.run, params) // the same step applied again
	require.NoError(t, err)
	require.Equal(t, notify.OutcomeSent, second.Outcome)
	require.Equal(t, first.Output["notice_id"], second.Output["notice_id"], "the replay returns the first notice")
	notices, jobs := c.written(t)
	require.Equal(t, 1, notices)
	require.Equal(t, 1, jobs)
	c.drain(t)
	require.Equal(t, 2, c.count(t, "SELECT COUNT(*) FROM core.notifications WHERE tenant_id=$1 AND title='Hello'", c.f.TenantID()), "each user got it once")

	// The step that failed before its commit left nothing, so applying it again queues the notice.
	other := c.run
	other.Seq = 2
	tx, err := c.f.Pool.Begin(context.Background())
	require.NoError(t, err)
	_, err = nt.(engine.OutcomeEffectNode).ApplyEffectResult(context.Background(), tx, other, graph.Node{ID: other.NodeID, Type: notify.NodeNotify}, graph.Effect{Params: map[string]any{
		"recipients": map[string]any{"users": []any{json.Number(strconv.FormatUint(uint64(c.people[0]), 10))}}, "title": "Rolled back", "body": "x"}})
	require.NoError(t, err)
	require.NoError(t, tx.Rollback(context.Background()))
	notices, _ = c.written(t)
	require.Equal(t, 1, notices, "a rolled-back step leaves no notice")
}

func TestAutomationNotify_ParticipantsOfTheTriggerEvent(t *testing.T) {
	c := setupAutomation(t)
	nt := c.node(t, notify.NodeNotify)
	params := map[string]any{"recipients": map[string]any{"participants": []string{"created_user"}}, "title": "Hi", "body": "There"}

	// A run without an event has no participants.
	_, err := c.step(t, nt, c.run, params)
	var se *engine.StepError
	require.ErrorAs(t, err, &se)
	require.Equal(t, notify.CodeNoTriggerEvent, se.Code)

	// The participant is read from the event that started the run.
	run := c.run
	run.Trigger = &engine.TriggerEvent{Key: participantEventKey, ID: uuid.NewString(), Payload: json.RawMessage(`{"created_user": ` + strconv.FormatUint(uint64(c.people[2]), 10) + `}`)}
	res, err := c.step(t, nt, run, params)
	require.NoError(t, err)
	require.Equal(t, notify.OutcomeSent, res.Outcome)
	require.Equal(t, "1", res.Output["recipients"].(json.Number).String())
	c.drain(t)
	require.Equal(t, 1, c.count(t, "SELECT COUNT(*) FROM core.notifications WHERE tenant_id=$1 AND title='Hi' AND user_id=$2", c.f.TenantID(), c.people[2]))

	// A participant the event's definition does not have is a failure of the node, and queues nothing.
	run.Seq++
	bad := map[string]any{"recipients": map[string]any{"participants": []string{"nobody"}}, "title": "Hi", "body": "There"}
	_, err = c.step(t, nt, run, bad)
	require.ErrorAs(t, err, &se)
	require.Equal(t, "invalid", se.Code)
	notices, _ := c.written(t)
	require.Equal(t, 1, notices)
}

func TestAutomationSendToContact_ClientWithoutAPhoneHasNoContact(t *testing.T) {
	c := setupAutomation(t)
	run := c.run
	run.Subject = engine.SubjectRef{Entity: customerRef.Entity, Key: customerRef.Key}
	nt := c.node(t, notify.NodeSendToContact)
	params := map[string]any{"channel": "sms", "subject": "Renewal", "body": "Your policy expires soon"}

	// The customer has an email and no phone.
	c.contacts.Add(customerRef, notifications.Contact{Targets: []notifications.Target{{Channel: notifications.ChannelEmail, Address: "client@example.com"}}})
	res, err := c.step(t, nt, run, params)
	require.NoError(t, err)
	require.Equal(t, notify.OutcomeNoContact, res.Outcome)
	notices, jobs := c.written(t)
	require.Zero(t, notices+jobs, "nothing is queued for a client that cannot be reached")

	// With a phone the message is queued and the SMS provider gets it.
	c.contacts.Add(customerRef, notifications.Contact{Targets: []notifications.Target{{Channel: notifications.ChannelSMS, Address: "+998901234567"}}})
	res, err = c.step(t, nt, run, params)
	require.NoError(t, err)
	require.Equal(t, notify.OutcomeSent, res.Outcome)
	require.Equal(t, "sms", res.Output["channel"])
	c.drain(t)
	sent := c.sms.SentTo("+998901234567")
	require.Len(t, sent, 1)
	require.Contains(t, sent[0].Text, "Your policy expires soon")
	require.Empty(t, c.email.Sent(), "only the chosen channel is used")
}

// A platform without the requested channel is a failure with the code unavailable, not a crash of the run.
func TestAutomationNotify_UnavailableChannelFailsWithTheCodeUnavailable(t *testing.T) {
	f := setupNotificationTest(t) // no channel adapters: only the inbox
	sender, err := composition.Resolve[notifications.AudienceSender](f.Container)
	require.NoError(t, err)
	nt := notify.NewNotify(notify.Config{Sender: sender})
	run := engine.StepContext{TenantID: f.TenantID(), RunID: uuid.New(), TokenID: uuid.New(), Seq: 1, NodeID: "ping"}
	params := map[string]any{"recipients": map[string]any{"users": []any{json.Number(strconv.FormatUint(uint64(f.User.ID()), 10))}}, "title": "t", "body": "b", "channels": []any{"email"}}
	_, err = nt.(engine.OutcomeEffectNode).ApplyEffectResult(context.Background(), f.Tx, run, graph.Node{ID: "ping", Type: notify.NodeNotify}, graph.Effect{Node: notify.NodeNotify, Params: params})
	var se *engine.StepError
	require.ErrorAs(t, err, &se)
	require.Equal(t, notify.CodeUnavailable, se.Code)
}

// The inbox counts bytes, not characters: what the node accepts, the delivery worker delivers.
func TestAutomationNotify_CyrillicTextWithinTheLimitsIsDelivered(t *testing.T) {
	c := setupAutomation(t)
	title, body := strings.Repeat("я", 150), strings.Repeat("ю", 5000) // 300 and 10000 bytes
	res, err := c.step(t, c.node(t, notify.NodeNotify), c.run, map[string]any{"recipients": map[string]any{"users": c.people[:1]}, "title": title, "body": body})
	require.NoError(t, err)
	require.Equal(t, notify.OutcomeSent, res.Outcome)
	c.drain(t)
	require.Equal(t, 1, c.count(t, "SELECT COUNT(*) FROM core.notifications WHERE tenant_id=$1 AND title=$2 AND user_id=$3", c.f.TenantID(), title, c.people[0]))
	require.Zero(t, c.count(t, "SELECT COUNT(*) FROM core.notification_dispatches WHERE tenant_id=$1 AND (completed_at IS NULL OR attempts>0)", c.f.TenantID()), "the job was not poisoned")

	run := c.run
	run.Seq++
	_, err = c.step(t, c.node(t, notify.NodeNotify), run, map[string]any{"recipients": map[string]any{"users": c.people[:1]}, "title": title + "я", "body": "x"})
	var se *engine.StepError
	require.ErrorAs(t, err, &se)
	require.Equal(t, engine.CodeInvalidInput, se.Code)
}

// The actor of the trigger event is its initiator, which the run keeps and the node hands the delivery layer.
func TestAutomationNotify_ActorParticipantIsTheInitiatorOfTheEvent(t *testing.T) {
	c := setupAutomation(t)
	const key = "acme.actor.created.v1"
	d := notifications.TestDefinition()
	d.Key = key
	d.RecipientKeys = []notifications.RecipientDefinition{notifications.ActorRecipient()}
	require.NoError(t, c.catalog.Register(d))
	nt := c.node(t, notify.NodeNotify)
	params := map[string]any{"recipients": map[string]any{"participants": []string{"actor"}}, "title": "Hi", "body": "Actor"}

	run := c.run
	run.Trigger = &engine.TriggerEvent{Key: key, ID: uuid.NewString(), Payload: json.RawMessage(`{}`), ActorUserID: int64(c.people[1])}
	res, err := c.step(t, nt, run, params)
	require.NoError(t, err)
	require.Equal(t, notify.OutcomeSent, res.Outcome)
	c.drain(t)
	require.Equal(t, 1, c.count(t, "SELECT COUNT(*) FROM core.notifications WHERE tenant_id=$1 AND title='Hi' AND user_id=$2", c.f.TenantID(), c.people[1]))

	// An event without an initiator has no actor: nobody is left.
	run.Seq++
	run.Trigger = &engine.TriggerEvent{Key: key, ID: uuid.NewString(), Payload: json.RawMessage(`{}`)}
	res, err = c.step(t, nt, run, params)
	require.NoError(t, err)
	require.Equal(t, notify.OutcomeNoRecipients, res.Outcome)
}

// Who was left out comes from the resolution that queued the notice.
func TestAutomationNotify_SkippedComeWithTheReceipt(t *testing.T) {
	c := setupAutomation(t)
	group := c.group(t, c.f.TenantID(), c.people...)
	_, err := c.f.Pool.Exec(c.committed, "UPDATE users SET status='pending_onboarding' WHERE id=$1", c.people[2])
	require.NoError(t, err)
	res, err := c.step(t, c.node(t, notify.NodeNotify), c.run, map[string]any{"recipients": map[string]any{"groups": []string{group.String()}}, "title": "Hi", "body": "Skipped"})
	require.NoError(t, err)
	require.Equal(t, notify.OutcomeSent, res.Outcome)
	require.Equal(t, "2", res.Output["recipients"].(json.Number).String())
	require.Equal(t, []any{map[string]any{"kind": "user", "ref": strconv.FormatUint(uint64(c.people[2]), 10), "reason": notifications.SkipInactive}}, res.Output["skipped"])
}

// The notify node needs no second connection while the step's transaction holds the only one.
func TestAutomationNotify_NeedsNoSecondConnectionInTheStepTransaction(t *testing.T) {
	c := setupAutomation(t)
	cfg, err := pgxpool.ParseConfig(c.f.Pool.Config().ConnString())
	require.NoError(t, err)
	cfg.MaxConns = 1
	single, err := pgxpool.NewWithConfig(context.Background(), cfg)
	require.NoError(t, err)
	t.Cleanup(single.Close)
	nt := notify.NewNotify(notify.Config{Sender: c.sender, Catalog: c.catalog, Pool: single})

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	tx, err := single.Begin(ctx) // the step's transaction holds the pool's only connection
	require.NoError(t, err)
	res, err := nt.(engine.OutcomeEffectNode).ApplyEffectResult(ctx, tx, c.run, graph.Node{ID: c.run.NodeID, Type: notify.NodeNotify}, graph.Effect{Params: map[string]any{
		"recipients": map[string]any{"users": []any{json.Number(strconv.FormatUint(uint64(c.people[0]), 10))}}, "title": "One", "body": "Connection"}})
	require.NoError(t, err)
	require.Equal(t, notify.OutcomeSent, res.Outcome)
	require.NoError(t, tx.Commit(ctx))
}
