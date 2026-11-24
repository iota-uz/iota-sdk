package engine_test

// send_to_contact through the real engine: the contact is checked before the step's transaction, the step is
// retried when what it reads keeps changing, and nothing is queued without the check.

import (
	"context"
	"encoding/json"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/modules/core/notifications"
	"github.com/iota-uz/iota-sdk/pkg/automation/builtin/notify"
	"github.com/iota-uz/iota-sdk/pkg/automation/engine"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/spi"
)

// contactSender is an AudienceSender that resolves a record as having no address and counts what it is asked.
type contactSender struct {
	mu        sync.Mutex
	resolves  int
	sends     int
	reachable bool
}

func (s *contactSender) Resolve(_ context.Context, req notifications.AudienceRequest) (notifications.ResolvedAudience, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.resolves++
	if s.reachable {
		return notifications.ResolvedAudience{Addressees: []notifications.Addressee{notifications.RecordAddressee(req.Audience.Records[0].Entity, req.Audience.Records[0].Key)}}, nil
	}
	ref := req.Audience.Records[0]
	return notifications.ResolvedAudience{Skipped: []notifications.SkippedRecipient{{Kind: notifications.RecipientRecord, Ref: ref.Entity + "/" + ref.Key, Reason: notifications.SkipNoAddress}}}, nil
}

func (s *contactSender) SendTx(_ context.Context, _ pgx.Tx, _ notifications.AudienceRequest) (notifications.AudienceReceipt, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sends++
	return notifications.AudienceReceipt{NoticeID: uuid.New(), Recipients: 1}, nil
}

func (s *contactSender) counts() (int, int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.resolves, s.sends
}

// shiftingSubjects loads a subject whose variables differ at every load while it is unsteady.
type shiftingSubjects struct {
	unsteady atomic.Bool
	loads    atomic.Int64
}

func (s *shiftingSubjects) Load(context.Context, spi.Subject, engine.SubjectRef) (json.RawMessage, error) {
	n := int64(1)
	if s.unsteady.Load() {
		n = s.loads.Add(1)
	}
	return json.RawMessage(`{"order": {"revision": ` + strconv.FormatInt(n, 10) + `}}`), nil
}

const contactGraph = `{"name": "Contact", "triggers": [{"id": "created", "event": "acme.order.created.v1"}],
 "nodes": [{"id": "sms", "type": "send_to_contact", "params": {"channel": "sms", "body": "Your order is ready"}},
  {"id": "sent", "type": "acme.note", "params": {"text": "sent"}},
  {"id": "nocontact", "type": "acme.note", "params": {"text": "no_contact"}},
  {"id": "failed", "type": "acme.note", "params": {"text": "failed"}},
  {"id": "fin", "type": "end", "params": {}}],
 "edges": [{"from": {"node": "created", "outcome": "fired"}, "to": "sms"},
  {"from": {"node": "sms", "outcome": "sent"}, "to": "sent"}, {"from": {"node": "sms", "outcome": "no_contact"}, "to": "nocontact"},
  {"from": {"node": "sms", "outcome": "failed"}, "to": "failed"},
  {"from": {"node": "sent", "outcome": "done"}, "to": "fin"}, {"from": {"node": "sent", "outcome": "failed"}, "to": "fin"},
  {"from": {"node": "nocontact", "outcome": "done"}, "to": "fin"}, {"from": {"node": "nocontact", "outcome": "failed"}, "to": "fin"},
  {"from": {"node": "failed", "outcome": "done"}, "to": "fin"}, {"from": {"node": "failed", "outcome": "failed"}, "to": "fin"}]}`

func newContactEnv(t *testing.T, sender *contactSender) *env {
	t.Helper()
	e := newEnv(t)
	require.NoError(t, e.reg.RegisterNode(notify.NewSendToContact(notify.Config{Sender: sender, Pool: e.db})))
	return e
}

// A client without an address takes no_contact in a real run, and nothing is queued.
func TestSendToContactInARunTakesNoContactWithoutQueueingAnything(t *testing.T) {
	sender := &contactSender{}
	e := newContactEnv(t, sender)
	p := e.proc()
	a := e.human()
	e.create(p, a, contactGraph)
	e.publish(e.tenant, orderCreated, "o-1", order{Amount: 7}, 2)
	run := onlyRun(t, e, p, a)
	p.until(func() bool { return e.status(p, run.RunID) == engine.RunDone }, "the run ends")
	assert.Equal(t, []string{"nocontact:no_contact"}, e.effects(run.RunID))
	resolves, sends := sender.counts()
	assert.Equal(t, 1, resolves, "the contact was checked once, before the transaction")
	assert.Zero(t, sends, "no notice was queued")
}

// While what the step reads keeps changing the contact check never settles: the step is retried and nothing is
// queued. Once it settles, the step is prepared again and takes the outcome the check gives: no_contact.
func TestSendToContactIsRetriedWhileWhatItReadsKeepsChanging(t *testing.T) {
	sender := &contactSender{}
	subjects := &shiftingSubjects{}
	subjects.unsteady.Store(true)
	e := newContactEnv(t, sender)
	p := e.proc(func(c *engine.Config) { c.Subjects = subjects })
	a := e.human()
	e.create(p, a, contactGraph)
	e.publish(e.tenant, orderCreated, "o-1", order{Amount: 7}, 2)
	run := onlyRun(t, e, p, a)

	// Several rounds of preparation fail to settle: the checks pile up, the step is not applied.
	p.until(func() bool { resolves, _ := sender.counts(); return resolves >= 4 }, "the contact is checked again and again")
	resolves, sends := sender.counts()
	assert.GreaterOrEqual(t, resolves, 4)
	assert.Zero(t, sends, "nothing is queued while the check cannot settle")
	assert.NotEqual(t, engine.RunDone, e.status(p, run.RunID), "the step is retried, not taken")
	assert.Empty(t, e.effects(run.RunID))

	// The variables settle: the next preparation fits, and the outcome is the check's.
	subjects.unsteady.Store(false)
	p.until(func() bool { return e.status(p, run.RunID) == engine.RunDone }, "the run ends once the step settles")
	assert.Equal(t, []string{"nocontact:no_contact"}, e.effects(run.RunID))
	_, sends = sender.counts()
	assert.Zero(t, sends, "no notice was queued, before or after")
}
