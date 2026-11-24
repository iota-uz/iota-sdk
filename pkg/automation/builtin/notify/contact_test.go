package notify_test

import (
	"context"
	"encoding/json"
	"testing"

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

var client = notifications.ObjectRef{Entity: "crm.client", Key: "42"}

func clientAudience() notifications.ResolvedAudience {
	return notifications.ResolvedAudience{Addressees: []notifications.Addressee{notifications.RecordAddressee(client.Entity, client.Key)}}
}

func TestSendToContactQueuesTheMessageForTheRecordOfTheRun(t *testing.T) {
	s := &sender{receipt: notifications.AudienceReceipt{NoticeID: notice, Recipients: 1}}
	nt := notify.NewSendToContact(notify.Config{Sender: s})

	res, err := apply(t, nt, step(), contactParams("sms"))
	require.NoError(t, err)
	assert.Equal(t, notify.OutcomeSent, res.Outcome)
	assert.Equal(t, notice.String(), res.Output["notice_id"])
	assert.Equal(t, "sms", res.Output["channel"])
	require.Len(t, s.sends, 1)
	req := s.sends[0]
	assert.Equal(t, []notifications.ObjectRef{client}, req.Audience.Records, "the record defaults to the subject of the run")
	assert.Equal(t, []notifications.Channel{notifications.ChannelSMS}, req.Channels, "one channel")
	assert.True(t, req.CheckContacts, "the delivery layer checks the record's contact in the step's transaction")
	assert.Equal(t, notify.EventKey, req.Event.Key)
	assert.Equal(t, "Reminder", req.Event.Data["title"])
	assert.Equal(t, "Your order is ready", req.Event.Data["body"])
	assert.Equal(t, notifications.Producer{InstanceID: runID.String(), Occurrence: "ping:" + token.String() + ":3"}, req.Producer)
	assert.Empty(t, s.resolves, "the node resolves nothing itself")
}

func TestSendToContactTakesTheRecordTheNodeNames(t *testing.T) {
	other := notifications.ObjectRef{Entity: "hrm.employee", Key: "9"}
	s := &sender{receipt: notifications.AudienceReceipt{NoticeID: notice, Recipients: 1}}
	nt := notify.NewSendToContact(notify.Config{Sender: s})
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

// The delivery layer keeps the record only when it has an address on the channel; with nobody left the
// receipt names no notice and the node takes no_contact.
func TestSendToContactWithoutAnAddressOnTheChannelHasNoContact(t *testing.T) {
	skipped := []notifications.SkippedRecipient{{Kind: notifications.RecipientRecord, Ref: "crm.client/42", Reason: notifications.SkipNoAddress}}
	s := &sender{receipt: notifications.AudienceReceipt{Skipped: skipped}}
	nt := notify.NewSendToContact(notify.Config{Sender: s})
	res, err := apply(t, nt, step(), contactParams("sms"))
	require.NoError(t, err)
	assert.Equal(t, notify.OutcomeNoContact, res.Outcome)
	assert.Equal(t, "", res.Output["notice_id"])
	assert.Equal(t, "sms", res.Output["channel"])
	assert.Equal(t, []any{map[string]any{"kind": "record", "ref": "crm.client/42", "reason": "no_address"}}, res.Output["skipped"])
}

func TestSendToContactFailsWhenTheDeliveryLayerRefuses(t *testing.T) {
	var se *engine.StepError
	// A channel that is not available fails the node whether or not the record has an address: the delivery
	// layer checks the channel before it reads the contact.
	s := &sender{sendErr: serrors.New(serrors.FailedPrecondition, "notification channel is not available: sms")}
	_, err := apply(t, notify.NewSendToContact(notify.Config{Sender: s}), step(), contactParams("sms"))
	require.ErrorAs(t, err, &se)
	assert.Equal(t, "unavailable", se.Code)

	s = &sender{sendErr: serrors.New(serrors.Invalid, "no contact resolver is registered for the recipient entity")}
	_, err = apply(t, notify.NewSendToContact(notify.Config{Sender: s}), step(), contactParams("sms"))
	require.ErrorAs(t, err, &se)
	assert.Equal(t, "invalid", se.Code)

	s = &sender{sendErr: serrors.New(serrors.Unavailable, "the contact of crm.client could not be read")}
	_, err = apply(t, notify.NewSendToContact(notify.Config{Sender: s}), step(), contactParams("sms"))
	require.ErrorAs(t, err, &se)
	assert.True(t, se.Retryable)
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

func TestSendToContactPreviewResolvesLikeASend(t *testing.T) {
	s := &sender{resolved: clientAudience()}
	nt := notify.NewSendToContact(notify.Config{Sender: s})
	preview := func() (engine.EffectResult, error) {
		return nt.(engine.EffectPreviewer).PreviewEffect(context.Background(), step(), graph.Node{ID: "ping"}, graph.Effect{Params: contactParams("sms")})
	}
	res, err := preview()
	require.NoError(t, err)
	assert.Equal(t, notify.OutcomeSent, res.Outcome)
	assert.Empty(t, s.sends)
	require.Len(t, s.resolves, 1)
	assert.True(t, s.resolves[0].CheckContacts, "the preview checks the contact as a send does")
	assert.Equal(t, map[string]any{"records": []any{"crm.client/42"}}, res.Output["resolved"])

	// Without an address the delivery layer leaves the record out: no_contact, with the reason in the trace.
	s.resolved = notifications.ResolvedAudience{Skipped: []notifications.SkippedRecipient{{Kind: notifications.RecipientRecord, Ref: "crm.client/42", Reason: notifications.SkipNoAddress}}}
	res, err = preview()
	require.NoError(t, err)
	assert.Equal(t, notify.OutcomeNoContact, res.Outcome)
	assert.Equal(t, map[string]any{"records": []any{}}, res.Output["resolved"])
	assert.Equal(t, []any{map[string]any{"kind": "record", "ref": "crm.client/42", "reason": "no_address"}}, res.Output["skipped"])
	raw, _ := json.Marshal(res.Output)
	assert.NotContains(t, string(raw), "+998", "a contact's address stays out of the trace")

	// A channel that is not available is a failure, as in a run, address or not.
	s.resolveErr = serrors.New(serrors.FailedPrecondition, "notification channel is not available: sms")
	_, err = preview()
	var se *engine.StepError
	require.ErrorAs(t, err, &se)
	assert.Equal(t, "unavailable", se.Code)

	// An entity without a resolver is refused by a send as invalid input, so the preview refuses it too.
	s.resolveErr = nil
	s.resolved = notifications.ResolvedAudience{Skipped: []notifications.SkippedRecipient{{Kind: notifications.RecipientRecord, Ref: "crm.client/42", Reason: notifications.SkipNoContactResolver}}}
	_, err = preview()
	require.ErrorAs(t, err, &se)
	assert.Equal(t, "invalid", se.Code)
}
