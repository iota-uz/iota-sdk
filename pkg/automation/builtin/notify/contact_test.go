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
	"github.com/iota-uz/iota-sdk/pkg/extruntime/host"
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
	assert.Empty(t, res.Output["notice_id"])
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
	raw, err := json.Marshal(res.Output)
	require.NoError(t, err)
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

// The engine prepares the effect before the step's transaction opens: the contact is checked then, and the send
// in the transaction asks nothing more of the contact resolver.
func TestSendToContactPreparesTheContactCheckBeforeTheTransaction(t *testing.T) {
	noAddress := []notifications.SkippedRecipient{{Kind: notifications.RecipientRecord, Ref: "crm.client/42", Reason: notifications.SkipNoAddress}}
	prepareAs := func(s *sender, run engine.StepContext) *engine.PreparedEffect {
		nt := notify.NewSendToContact(notify.Config{Sender: s})
		v, err := nt.(engine.EffectPreparer).PrepareEffect(context.Background(), run, graph.Node{ID: "ping"}, graph.Effect{Params: contactParams("sms")})
		return &engine.PreparedEffect{Value: v, Err: err}
	}
	applyWith := func(s *sender, prepared *engine.PreparedEffect) (engine.EffectResult, error) {
		run := step()
		run.Prepared = prepared
		return apply(t, notify.NewSendToContact(notify.Config{Sender: s}), run, contactParams("sms"))
	}

	// A client without an address: prepared as unreachable, the node takes no_contact and queues nothing.
	s := &sender{resolved: notifications.ResolvedAudience{Skipped: noAddress}}
	prepared := prepareAs(s, step())
	require.NoError(t, prepared.Err)
	require.Len(t, s.resolves, 1)
	assert.True(t, s.resolves[0].CheckContacts, "the contact is checked while preparing")
	res, err := applyWith(s, prepared)
	require.NoError(t, err)
	assert.Equal(t, notify.OutcomeNoContact, res.Outcome)
	assert.Empty(t, s.sends, "nothing is queued, and the transaction asked nothing")
	assert.Equal(t, []any{map[string]any{"kind": "record", "ref": "crm.client/42", "reason": "no_address"}}, res.Output["skipped"])

	// A reachable client: the send in the transaction does not check the contact again.
	s = &sender{resolved: clientAudience(), receipt: notifications.AudienceReceipt{NoticeID: notice, Recipients: 1}}
	res, err = applyWith(s, prepareAs(s, step()))
	require.NoError(t, err)
	assert.Equal(t, notify.OutcomeSent, res.Outcome)
	require.Len(t, s.sends, 1)
	assert.False(t, s.sends[0].CheckContacts, "the contact was checked before the transaction")

	// A preparation made for another record, or one that is exhausted (what the step reads kept changing), never
	// lets the record be queued unchecked: the step stops with a platform error, which the engine retries, and
	// nothing is queued, whatever the record's address is.
	elsewhere := step()
	elsewhere.Subject = engine.SubjectRef{Entity: "crm.client", Key: "7"}
	for name, prepared := range map[string]*engine.PreparedEffect{
		"another record": prepareAs(&sender{resolved: clientAudience()}, elsewhere),
		"exhausted":      {Exhausted: true},
		"empty":          {},
	} {
		s = &sender{receipt: notifications.AudienceReceipt{NoticeID: notice, Recipients: 1}}
		_, err = applyWith(s, prepared)
		require.Error(t, err, name)
		assert.True(t, host.IsInfrastructure(err), "%s: a retry, not an outcome", name)
		assert.Empty(t, s.sends, "%s: nothing is queued", name)
	}

	// A preparation that failed fails the node with its error, and queues nothing.
	s = &sender{resolveErr: serrors.New(serrors.FailedPrecondition, "notification channel is not available: sms")}
	prepared = prepareAs(s, step())
	require.Error(t, prepared.Err)
	_, err = applyWith(s, prepared)
	var se *engine.StepError
	require.ErrorAs(t, err, &se)
	assert.Equal(t, "unavailable", se.Code)
	assert.Empty(t, s.sends)

	// An entity without a resolver leaves nobody, but it is not no_contact: the send refuses it as invalid input.
	s = &sender{
		resolved: notifications.ResolvedAudience{Skipped: []notifications.SkippedRecipient{{Kind: notifications.RecipientRecord, Ref: "crm.client/42", Reason: notifications.SkipNoContactResolver}}},
		sendErr:  serrors.New(serrors.Invalid, "no contact resolver is registered for the recipient entity"),
	}
	_, err = applyWith(s, prepareAs(s, step()))
	require.ErrorAs(t, err, &se)
	assert.Equal(t, "invalid", se.Code)
	assert.Len(t, s.sends, 1)
}

// An agent's dry run previews offline: the record's contact is not checked, since a package's resolver would
// run its code, and the preview says so.
func TestSendToContactPreviewOfflineChecksNoContact(t *testing.T) {
	s := &sender{resolved: clientAudience()}
	nt := notify.NewSendToContact(notify.Config{Sender: s})
	_, local := nt.(engine.OfflinePreviewer)
	require.True(t, local, "send_to_contact previews offline")
	run := step()
	run.Offline = true
	res, err := nt.(engine.EffectPreviewer).PreviewEffect(context.Background(), run, graph.Node{ID: "ping"}, graph.Effect{Params: contactParams("sms")})
	require.NoError(t, err)
	require.Len(t, s.resolves, 1)
	assert.False(t, s.resolves[0].CheckContacts, "no contact resolver runs offline")
	assert.Equal(t, false, res.Output["contact_checked"])
	assert.Empty(t, s.sends)
}
