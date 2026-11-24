package notify_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/notification"
	"github.com/iota-uz/iota-sdk/modules/core/notifications"
	"github.com/iota-uz/iota-sdk/pkg/automation/builtin/notify"
	"github.com/iota-uz/iota-sdk/pkg/automation/catalog"
	"github.com/iota-uz/iota-sdk/pkg/automation/engine"
	"github.com/iota-uz/iota-sdk/pkg/automation/expr"
	"github.com/iota-uz/iota-sdk/pkg/automation/graph"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

// sender is an AudienceSender double: it records what it was asked and answers what it was told to.
type sender struct {
	resolved    notifications.ResolvedAudience
	resolveErr  error
	receipt     notifications.AudienceReceipt
	sendErr     error
	sends       []notifications.AudienceRequest
	resolves    []notifications.AudienceRequest
	tenants     []uuid.UUID
	sendTenants []uuid.UUID
}

func (s *sender) SendTx(ctx context.Context, _ pgx.Tx, req notifications.AudienceRequest) (notifications.AudienceReceipt, error) {
	s.sends = append(s.sends, req)
	id, _ := composables.UseTenantID(ctx)
	s.sendTenants = append(s.sendTenants, id)
	return s.receipt, s.sendErr
}

func (s *sender) Resolve(ctx context.Context, req notifications.AudienceRequest) (notifications.ResolvedAudience, error) {
	s.resolves = append(s.resolves, req)
	id, _ := composables.UseTenantID(ctx)
	s.tenants = append(s.tenants, id)
	return s.resolved, s.resolveErr
}

func resolvedUsers(ids ...uint) notifications.ResolvedAudience {
	return notifications.ResolvedAudience{UserIDs: ids}
}

var (
	tenant = uuid.MustParse("00000000-0000-0000-0000-0000000000aa")
	runID  = uuid.MustParse("00000000-0000-0000-0000-0000000000bb")
	token  = uuid.MustParse("00000000-0000-0000-0000-0000000000cc")
	notice = uuid.MustParse("00000000-0000-0000-0000-0000000000dd")
)

func step() engine.StepContext {
	return engine.StepContext{TenantID: tenant, RunID: runID, TokenID: token, Seq: 3, NodeID: "ping", Subject: engine.SubjectRef{Entity: "crm.client", Key: "42"}}
}

func apply(t *testing.T, nt catalog.NodeType, run engine.StepContext, params map[string]any) (engine.EffectResult, error) {
	t.Helper()
	// What the engine hands a node: its params as JSON, numbers as json.Number.
	raw, err := json.Marshal(params)
	require.NoError(t, err)
	var p map[string]any
	require.NoError(t, expr.UnmarshalJSON(raw, &p))
	return nt.(engine.OutcomeEffectNode).ApplyEffectResult(context.Background(), nil, run, graph.Node{ID: "ping", Type: nt.Spec().Type}, graph.Effect{Node: nt.Spec().Type, Params: p})
}

func notifyParams(recipients map[string]any) map[string]any {
	return map[string]any{"recipients": recipients, "title": "Order paid", "body": "Order 42 was paid", "action_url": "/orders/42"}
}

func TestNotifyQueuesTheNoticeInTheStepTransaction(t *testing.T) {
	s := &sender{resolved: resolvedUsers(7, 8), receipt: notifications.AudienceReceipt{NoticeID: notice, Recipients: 2}}
	nt := notify.NewNotify(notify.Config{Sender: s, BaseURL: "https://erp.example.com"})
	params := notifyParams(map[string]any{
		"users": []any{7, int64(8)}, "groups": []any{"7a8c0e5e-0000-4000-8000-000000000001"}, "roles": []any{"12"},
	})
	params["channels"] = []any{"email", "in_app", "email"}
	params["level"] = "critical"
	res, err := apply(t, nt, step(), params)
	require.NoError(t, err)
	assert.Equal(t, notify.OutcomeSent, res.Outcome)
	assert.Equal(t, notice.String(), res.Output["notice_id"])
	assert.Equal(t, "2", res.Output["recipients"].(json.Number).String())
	assert.Empty(t, res.Output["skipped"])

	require.Len(t, s.sends, 1)
	req := s.sends[0]
	assert.Equal(t, tenant, s.sendTenants[0], "the delivery layer reads the tenant from the context")
	assert.Equal(t, notify.EventKey, req.Event.Key)
	assert.Equal(t, tenant, req.Event.TenantID)
	assert.Equal(t, "Order paid", req.Event.Data["title"])
	assert.Equal(t, "Order 42 was paid", req.Event.Data["body"])
	assert.Equal(t, "/orders/42", req.Event.Data["action_url"])
	assert.Equal(t, notifications.SubjectReference{Type: "crm.client", ID: "42"}, req.Event.Subject, "the subject is the run's: its access is checked per user at delivery")
	assert.Empty(t, req.Event.DedupeKey, "the identity comes from the producer")
	assert.Equal(t, notifications.Producer{InstanceID: runID.String(), Occurrence: "ping:" + token.String() + ":3"}, req.Producer)
	assert.Equal(t, []uint{7, 8}, req.Audience.Users)
	assert.Equal(t, []uuid.UUID{uuid.MustParse("7a8c0e5e-0000-4000-8000-000000000001")}, req.Audience.Groups)
	assert.Equal(t, []uint{12}, req.Audience.Roles)
	assert.Equal(t, []notifications.Channel{notifications.ChannelEmail, notifications.ChannelInApp}, req.Channels)
	assert.Equal(t, notification.LevelError, req.Level, "critical is the SDK's error level")
	assert.Nil(t, req.Source, "only participants need the trigger event")
}

func TestNotifyStepsHaveTheirOwnProducer(t *testing.T) {
	s := &sender{resolved: resolvedUsers(7), receipt: notifications.AudienceReceipt{NoticeID: notice, Recipients: 1}}
	nt := notify.NewNotify(notify.Config{Sender: s})
	p := notifyParams(map[string]any{"users": []any{7}})
	_, err := apply(t, nt, step(), p)
	require.NoError(t, err)
	_, err = apply(t, nt, step(), p)
	require.NoError(t, err)
	again := step()
	again.Seq = 4 // the same node visited again
	_, err = apply(t, nt, again, p)
	require.NoError(t, err)
	require.Len(t, s.sends, 3)
	assert.Equal(t, s.sends[0].Producer, s.sends[1].Producer, "the same step applied again is the same producer")
	assert.NotEqual(t, s.sends[0].Producer, s.sends[2].Producer, "a second visit of the node is a new notice")
}

func TestNotifyWithNobodyLeftQueuesNothing(t *testing.T) {
	skipped := []notifications.SkippedRecipient{{Kind: notifications.RecipientUser, Ref: "7", Reason: notifications.SkipInactive}}
	s := &sender{receipt: notifications.AudienceReceipt{Skipped: skipped}}
	nt := notify.NewNotify(notify.Config{Sender: s})
	res, err := apply(t, nt, step(), notifyParams(map[string]any{"users": []any{7}}))
	require.NoError(t, err)
	assert.Equal(t, notify.OutcomeNoRecipients, res.Outcome)
	assert.Len(t, s.sends, 1, "the delivery layer is asked, and queues nothing for an empty audience: its receipt names no notice")
	assert.Empty(t, s.resolves, "who was left out comes with the receipt of the send, not from a second resolution")
	assert.Empty(t, res.Output["notice_id"])
	assert.Equal(t, "0", res.Output["recipients"].(json.Number).String())
	assert.Equal(t, []any{map[string]any{"kind": "user", "ref": "7", "reason": "inactive"}}, res.Output["skipped"])

	// An expression that gave an empty list names nobody at all: the sender is not asked.
	s = &sender{}
	nt = notify.NewNotify(notify.Config{Sender: s})
	res, err = apply(t, nt, step(), notifyParams(map[string]any{"users": []any{}}))
	require.NoError(t, err)
	assert.Equal(t, notify.OutcomeNoRecipients, res.Outcome)
	assert.Empty(t, s.sends)
	assert.Empty(t, s.resolves)
}

func TestNotifyReportsWhatTheDeliveryLayerRefuses(t *testing.T) {
	run := func(err error) error {
		s := &sender{resolved: resolvedUsers(7), sendErr: err}
		_, got := apply(t, notify.NewNotify(notify.Config{Sender: s}), step(), notifyParams(map[string]any{"users": []any{7}}))
		return got
	}
	var se *engine.StepError

	err := run(serrors.New(serrors.FailedPrecondition, "notification channel is not available: sms"))
	require.ErrorAs(t, err, &se)
	assert.Equal(t, "unavailable", se.Code, "a platform that is not configured or a channel that is missing is unavailable")

	err = run(serrors.New(serrors.Invalid, "too many notification recipients"))
	require.ErrorAs(t, err, &se)
	assert.Equal(t, "invalid", se.Code, "any other error carries its code")
	assert.Contains(t, se.Message, "too many")

	err = run(serrors.New(serrors.Conflict, "notification identity belongs to an event rule"))
	require.ErrorAs(t, err, &se)
	assert.Equal(t, "conflict", se.Code)

	long := run(serrors.New(serrors.Invalid, strings.Repeat("я", 1000)))
	require.ErrorAs(t, long, &se)
	assert.LessOrEqual(t, len(se.Message), 500)
	assert.Equal(t, strings.ToValidUTF8(se.Message, ""), se.Message, "the message is cut on a character")

	own := &engine.StepError{Code: "custom", Message: "x"}
	require.ErrorAs(t, run(own), &se)
	assert.Equal(t, "custom", se.Code)
}

func TestNotifyRefusesWhatItCannotSend(t *testing.T) {
	nt := notify.NewNotify(notify.Config{Sender: &sender{}, BaseURL: "https://erp.example.com"})
	for name, mutate := range map[string]func(map[string]any){
		"empty title":      func(p map[string]any) { p["title"] = "  " },
		"empty body":       func(p map[string]any) { p["body"] = "" },
		"long title":       func(p map[string]any) { p["title"] = strings.Repeat("x", 301) },
		"long cyrillic":    func(p map[string]any) { p["title"] = strings.Repeat("я", 151) }, // 302 bytes: the inbox counts bytes
		"long body":        func(p map[string]any) { p["body"] = strings.Repeat("я", 5001) },
		"link to //host":   func(p map[string]any) { p["action_url"] = "https://erp.example.com//evil.com" },
		"encoded //host":   func(p map[string]any) { p["action_url"] = "/%2F%2Fevil.com" },
		"external link":    func(p map[string]any) { p["action_url"] = "https://evil.example.com/x" },
		"unknown channel":  func(p map[string]any) { p["channels"] = []any{"fax"} },
		"unknown level":    func(p map[string]any) { p["level"] = "fatal" },
		"fractional user":  func(p map[string]any) { p["recipients"] = map[string]any{"users": []any{1.5}} },
		"zero user":        func(p map[string]any) { p["recipients"] = map[string]any{"users": []any{0}} },
		"string user":      func(p map[string]any) { p["recipients"] = map[string]any{"users": []any{"7"}} },
		"bad group":        func(p map[string]any) { p["recipients"] = map[string]any{"groups": []any{"nope"}} },
		"bad role":         func(p map[string]any) { p["recipients"] = map[string]any{"roles": []any{"admin"}} },
		"users not a list": func(p map[string]any) { p["recipients"] = map[string]any{"users": "7"} },
	} {
		t.Run(name, func(t *testing.T) {
			p := notifyParams(map[string]any{"users": []any{7}})
			mutate(p)
			_, err := apply(t, nt, step(), p)
			var se *engine.StepError
			require.ErrorAs(t, err, &se)
			assert.Equal(t, engine.CodeInvalidInput, se.Code)
		})
	}
}

func TestNotifyLinksStayInTheApplication(t *testing.T) {
	s := &sender{resolved: resolvedUsers(7), receipt: notifications.AudienceReceipt{NoticeID: notice, Recipients: 1}}
	nt := notify.NewNotify(notify.Config{Sender: s, BaseURL: "https://erp.example.com"})
	for link, want := range map[string]string{
		"/orders/42?tab=1":                      "/orders/42?tab=1",
		"https://erp.example.com/orders/42":     "/orders/42",
		"https://ERP.example.com/orders/42?a=1": "/orders/42?a=1",
		"https://erp.example.com":               "/",
	} {
		p := notifyParams(map[string]any{"users": []any{7}})
		p["action_url"] = link
		_, err := apply(t, nt, step(), p)
		require.NoError(t, err, link)
		assert.Equal(t, want, s.sends[len(s.sends)-1].Event.Data["action_url"], link)
	}
	for _, link := range []string{
		"//evil.example.com/x", "https://evil.example.com/orders", "https://erp.example.com.evil.com/x",
		"https://user@erp.example.com/x", "javascript:alert(1)", "orders/42", "/a\\b", "ftp://erp.example.com/x",
	} {
		p := notifyParams(map[string]any{"users": []any{7}})
		p["action_url"] = link
		_, err := apply(t, nt, step(), p)
		require.Error(t, err, link)
	}
	// Without a known address only paths of the application are links.
	bare := notify.NewNotify(notify.Config{Sender: s})
	p := notifyParams(map[string]any{"users": []any{7}})
	p["action_url"] = "https://erp.example.com/orders"
	_, err := apply(t, bare, step(), p)
	require.Error(t, err)
}

func TestNotifyParticipantsNeedTheTriggerEvent(t *testing.T) {
	cat := notifications.NewCatalog()
	def := notifications.Definition{Module: "acme", Key: "acme.order.created.v1", Name: map[string]string{"en": "Created"},
		RecipientKeys: []notifications.RecipientDefinition{{Key: "owner", Name: map[string]string{"en": "Owner"}, Resolve: func(context.Context, notifications.Event) ([]uint, error) { return nil, nil }}},
		DedupeKey:     func(e notifications.Event) string { return "created:" + e.ID }}
	require.NoError(t, cat.Register(def))
	s := &sender{resolved: resolvedUsers(7), receipt: notifications.AudienceReceipt{NoticeID: notice, Recipients: 1}}
	nt := notify.NewNotify(notify.Config{Sender: s, Catalog: cat})
	p := notifyParams(map[string]any{"participants": []any{"owner"}})

	_, err := apply(t, nt, step(), p)
	var se *engine.StepError
	require.ErrorAs(t, err, &se)
	assert.Equal(t, notify.CodeNoTriggerEvent, se.Code, "a run without an event has no participants")
	assert.Empty(t, s.sends)

	run := step()
	run.Trigger = &engine.TriggerEvent{Key: "acme.order.created.v1", ID: "ev-1", Payload: json.RawMessage(`{"amount": 5}`), ActorUserID: 2}
	res, err := apply(t, nt, run, p)
	require.NoError(t, err)
	assert.Equal(t, notify.OutcomeSent, res.Outcome)
	require.Len(t, s.sends, 1)
	src := s.sends[0].Source
	require.NotNil(t, src)
	assert.Equal(t, "acme.order.created.v1", src.Key)
	assert.Equal(t, "ev-1", src.ID)
	assert.Equal(t, tenant, src.TenantID)
	assert.JSONEq(t, `{"amount": 5}`, string(src.Payload))
	assert.Equal(t, uint(2), src.ActorUserID, "the actor participant is the initiator of the trigger event")
	assert.Equal(t, "created:ev-1", src.DedupeKey, "the source carries the identity its definition gives the event")
	assert.Equal(t, []string{"owner"}, s.sends[0].Audience.Participants)
}

func TestNotifyPreviewResolvesWithoutSending(t *testing.T) {
	s := &sender{resolved: notifications.ResolvedAudience{
		UserIDs:    []uint{7, 8},
		Addressees: []notifications.Addressee{notifications.RecordAddressee("crm.client", "42")},
		Skipped:    []notifications.SkippedRecipient{{Kind: notifications.RecipientGroup, Ref: "g", Reason: notifications.SkipNotFound}},
	}}
	nt := notify.NewNotify(notify.Config{Sender: s})
	res, err := nt.(engine.EffectPreviewer).PreviewEffect(context.Background(), step(), graph.Node{ID: "ping"}, graph.Effect{Params: notifyParams(map[string]any{"users": []any{7, 8}})})
	require.NoError(t, err)
	assert.Equal(t, notify.OutcomeSent, res.Outcome)
	assert.Empty(t, s.sends, "a dry run sends nothing")
	assert.Equal(t, tenant, s.tenants[0])
	assert.Equal(t, "3", res.Output["recipients"].(json.Number).String())
	assert.Equal(t, map[string]any{"users": []any{json.Number("7"), json.Number("8")}, "records": []any{"record:crm.client/42"}}, res.Output["resolved"])
	assert.Len(t, res.Output["skipped"], 1)

	s.resolved = notifications.ResolvedAudience{}
	res, err = nt.(engine.EffectPreviewer).PreviewEffect(context.Background(), step(), graph.Node{ID: "ping"}, graph.Effect{Params: notifyParams(map[string]any{"users": []any{7}})})
	require.NoError(t, err)
	assert.Equal(t, notify.OutcomeNoRecipients, res.Outcome)

	s.resolveErr = serrors.New(serrors.Invalid, "unknown notification participant: nobody")
	_, err = nt.(engine.EffectPreviewer).PreviewEffect(context.Background(), step(), graph.Node{ID: "ping"}, graph.Effect{Params: notifyParams(map[string]any{"users": []any{7}})})
	var se *engine.StepError
	require.ErrorAs(t, err, &se)
	assert.Equal(t, "invalid", se.Code)
}

func TestAnUnclassifiedErrorIsTheNodesFailure(t *testing.T) {
	s := &sender{resolved: resolvedUsers(7), sendErr: errors.New("ERROR: relation \"secret_table\" does not exist (SQLSTATE 42P01)")}
	_, err := apply(t, notify.NewNotify(notify.Config{Sender: s}), step(), notifyParams(map[string]any{"users": []any{7}}))
	var se *engine.StepError
	require.ErrorAs(t, err, &se, "an unclassified error is the node's failure with a code")
	assert.Equal(t, "internal", se.Code)
	assert.NotContains(t, se.Message, "secret_table", "the text of a failed statement stays out of the run")
}

func TestNotifyTitleAndBodyAreCountedInBytesLikeTheInbox(t *testing.T) {
	s := &sender{resolved: resolvedUsers(7), receipt: notifications.AudienceReceipt{NoticeID: notice, Recipients: 1}}
	nt := notify.NewNotify(notify.Config{Sender: s})
	p := notifyParams(map[string]any{"users": []any{7}})
	p["title"] = strings.Repeat("я", 150) // 300 bytes
	p["body"] = strings.Repeat("я", 5000) // 10000 bytes
	_, err := apply(t, nt, step(), p)
	require.NoError(t, err)
}
