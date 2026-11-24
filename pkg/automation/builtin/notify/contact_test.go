package notify_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/modules/core/notifications"
	"github.com/iota-uz/iota-sdk/pkg/automation/builtin/notify"
	"github.com/iota-uz/iota-sdk/pkg/automation/engine"
	"github.com/iota-uz/iota-sdk/pkg/automation/graph"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

func contactParams(channel string) map[string]any {
	return map[string]any{"channel": channel, "subject": "Reminder", "body": "Your order is ready"}
}

func contacts(t *testing.T, ref notifications.ObjectRef, contact *notifications.Contact) *notifications.ContactResolvers {
	t.Helper()
	reg := notifications.NewContactResolvers()
	require.NoError(t, reg.Register(ref.Entity, notifications.ContactResolverFunc(func(_ context.Context, tenantID uuid.UUID, got notifications.ObjectRef) (notifications.Contact, error) {
		assert.Equal(t, tenant, tenantID)
		assert.Equal(t, ref, got)
		if contact == nil {
			return notifications.Contact{}, notifications.ErrContactNotFound
		}
		return *contact, nil
	})))
	return reg
}

var client = notifications.ObjectRef{Entity: "crm.client", Key: "42"}

func TestSendToContactQueuesTheMessageForTheRecordOfTheRun(t *testing.T) {
	s := &sender{resolved: notifications.ResolvedAudience{Addressees: []notifications.Addressee{notifications.RecordAddressee(client.Entity, client.Key)}},
		receipt: notifications.AudienceReceipt{NoticeID: notice, Recipients: 1}}
	book := contacts(t, client, &notifications.Contact{Targets: []notifications.Target{{Channel: notifications.ChannelSMS, Address: "+998901234567"}}})
	nt := notify.NewSendToContact(notify.Config{Sender: s, Contacts: book})

	res, err := apply(t, nt, step(), contactParams("sms"))
	require.NoError(t, err)
	assert.Equal(t, notify.OutcomeSent, res.Outcome)
	assert.Equal(t, notice.String(), res.Output["notice_id"])
	assert.Equal(t, "sms", res.Output["channel"])
	require.Len(t, s.sends, 1)
	req := s.sends[0]
	assert.Equal(t, []notifications.ObjectRef{client}, req.Audience.Records, "the record defaults to the subject of the run")
	assert.Equal(t, []notifications.Channel{notifications.ChannelSMS}, req.Channels, "one channel")
	assert.Equal(t, notify.EventKey, req.Event.Key)
	assert.Equal(t, "Reminder", req.Event.Data["title"])
	assert.Equal(t, "Your order is ready", req.Event.Data["body"])
	assert.Equal(t, notifications.Producer{InstanceID: runID.String(), Occurrence: "ping:" + token.String() + ":3"}, req.Producer)
}

func TestSendToContactTakesTheRecordTheNodeNames(t *testing.T) {
	other := notifications.ObjectRef{Entity: "hrm.employee", Key: "9"}
	s := &sender{receipt: notifications.AudienceReceipt{NoticeID: notice, Recipients: 1}}
	book := contacts(t, other, &notifications.Contact{Targets: []notifications.Target{{Channel: notifications.ChannelEmail, Address: "a@b.c"}}})
	nt := notify.NewSendToContact(notify.Config{Sender: s, Contacts: book})
	p := contactParams("email")
	p["record"] = map[string]any{"entity": other.Entity, "key": other.Key}
	res, err := apply(t, nt, step(), p)
	require.NoError(t, err)
	assert.Equal(t, notify.OutcomeSent, res.Outcome)
	assert.Equal(t, []notifications.ObjectRef{other}, s.sends[0].Audience.Records)

	p["record"] = map[string]any{"entity": other.Entity}
	_, err = apply(t, nt, step(), p)
	var se *engine.StepError
	require.ErrorAs(t, err, &se)
	assert.Equal(t, engine.CodeInvalidInput, se.Code)
}

func TestSendToContactWithoutAnAddressOnTheChannelHasNoContact(t *testing.T) {
	for name, contact := range map[string]*notifications.Contact{
		"the record is unknown":      nil,
		"the record has no phone":    {Targets: []notifications.Target{{Channel: notifications.ChannelEmail, Address: "a@b.c"}}},
		"the phone is blank":         {Targets: []notifications.Target{{Channel: notifications.ChannelSMS, Address: "  "}}},
		"the record has no contacts": {},
	} {
		t.Run(name, func(t *testing.T) {
			s := &sender{receipt: notifications.AudienceReceipt{NoticeID: notice, Recipients: 1}}
			nt := notify.NewSendToContact(notify.Config{Sender: s, Contacts: contacts(t, client, contact)})
			res, err := apply(t, nt, step(), contactParams("sms"))
			require.NoError(t, err)
			assert.Equal(t, notify.OutcomeNoContact, res.Outcome)
			assert.Equal(t, "", res.Output["notice_id"])
			assert.Equal(t, "sms", res.Output["channel"])
			assert.Empty(t, s.sends, "nothing is queued for a record that cannot be reached")
		})
	}
}

func TestSendToContactReportsABrokenContactLookup(t *testing.T) {
	s := &sender{}
	nt := notify.NewSendToContact(notify.Config{Sender: s, Contacts: notifications.NewContactResolvers()})
	_, err := apply(t, nt, step(), contactParams("sms"))
	var se *engine.StepError
	require.ErrorAs(t, err, &se)
	assert.Equal(t, notify.CodeNoContactResolver, se.Code)

	reg := notifications.NewContactResolvers()
	require.NoError(t, reg.Register(client.Entity, notifications.ContactResolverFunc(func(context.Context, uuid.UUID, notifications.ObjectRef) (notifications.Contact, error) {
		return notifications.Contact{}, serrors.New(serrors.Unavailable, "crm is down")
	})))
	nt = notify.NewSendToContact(notify.Config{Sender: s, Contacts: reg})
	_, err = apply(t, nt, step(), contactParams("sms"))
	require.ErrorAs(t, err, &se)
	assert.Equal(t, notify.CodeContactLookup, se.Code)
	assert.True(t, se.Retryable)
	assert.NotContains(t, se.Message, "crm is down", "what the resolver said stays out of the run")
	assert.Empty(t, s.sends)
}

func TestSendToContactFailsWhenTheDeliveryLayerRefuses(t *testing.T) {
	s := &sender{sendErr: serrors.New(serrors.FailedPrecondition, "notification channel is not available: sms")}
	book := contacts(t, client, &notifications.Contact{Targets: []notifications.Target{{Channel: notifications.ChannelSMS, Address: "+998901234567"}}})
	nt := notify.NewSendToContact(notify.Config{Sender: s, Contacts: book})
	_, err := apply(t, nt, step(), contactParams("sms"))
	var se *engine.StepError
	require.ErrorAs(t, err, &se)
	assert.Equal(t, "unavailable", se.Code)
}

func TestSendToContactNeedsARecordAndAnExternalChannel(t *testing.T) {
	nt := notify.NewSendToContact(notify.Config{Sender: &sender{}})
	for name, tc := range map[string]struct {
		run    engine.StepContext
		params map[string]any
	}{
		"in-app is not a contact channel": {step(), contactParams("in_app")},
		"an unknown channel":              {step(), contactParams("fax")},
		"a run about nothing":             {engine.StepContext{TenantID: tenant, Subject: engine.SubjectRef{Entity: graph.SubjectNone}}, contactParams("sms")},
		"a conversation is no record":     {engine.StepContext{TenantID: tenant, Subject: engine.SubjectRef{Entity: graph.SubjectConversation, Key: "c"}}, contactParams("sms")},
		"an empty text":                   {step(), map[string]any{"channel": "sms", "body": " "}},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := apply(t, nt, tc.run, tc.params)
			var se *engine.StepError
			require.ErrorAs(t, err, &se)
			assert.Equal(t, engine.CodeInvalidInput, se.Code)
		})
	}
}

func TestSendToContactPreviewChecksTheContactWithoutSending(t *testing.T) {
	s := &sender{resolved: notifications.ResolvedAudience{Addressees: []notifications.Addressee{notifications.RecordAddressee(client.Entity, client.Key)}}}
	book := contacts(t, client, &notifications.Contact{Targets: []notifications.Target{{Channel: notifications.ChannelSMS, Address: "+998901234567"}}})
	nt := notify.NewSendToContact(notify.Config{Sender: s, Contacts: book})
	res, err := nt.(engine.EffectPreviewer).PreviewEffect(context.Background(), step(), graph.Node{ID: "ping"}, graph.Effect{Params: contactParams("sms")})
	require.NoError(t, err)
	assert.Equal(t, notify.OutcomeSent, res.Outcome)
	assert.Empty(t, s.sends)
	assert.Len(t, s.resolves, 1, "the audience is resolved as a send would")

	res, err = nt.(engine.EffectPreviewer).PreviewEffect(context.Background(), step(), graph.Node{ID: "ping"}, graph.Effect{Params: contactParams("email")})
	require.NoError(t, err)
	assert.Equal(t, notify.OutcomeNoContact, res.Outcome)
}
