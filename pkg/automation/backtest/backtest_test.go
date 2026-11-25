package backtest_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/pkg/automation/backtest"
	"github.com/iota-uz/iota-sdk/pkg/automation/catalog"
	"github.com/iota-uz/iota-sdk/pkg/automation/connectors"
	"github.com/iota-uz/iota-sdk/pkg/automation/engine"
	"github.com/iota-uz/iota-sdk/pkg/automation/expr"
	"github.com/iota-uz/iota-sdk/pkg/automation/graph"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/events"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/host/hosttest"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/native"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/spi"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/workflow"
	"github.com/iota-uz/iota-sdk/pkg/itf"
)

func TestMain(m *testing.M) {
	// The migrator reads ./migrations relative to the working directory.
	if err := os.Chdir("../../.."); err != nil {
		panic(err)
	}
	os.Exit(m.Run())
}

type message struct {
	Text   string `json:"text"`
	Secret string `json:"secret"`
}

const (
	evMessage = "acme.message.v1"
	permSec   = "Acme.Secret"
)

var messageEvent = native.Event[message](evMessage, native.EventSpec{Subject: "acme.client", Sensitive: map[string][]string{"secret": {permSec}}})

type nativeEvents []native.EventDefinition

func (n nativeEvents) Events() []native.EventDefinition { return n }

type specNode struct{ spec graph.NodeSpec }

func (n specNode) Spec() graph.NodeSpec              { return n.spec }
func (n specNode) Requires() []catalog.ConnectorKind { return nil }

const textParams = `{"type":"object","properties":{"text":{"type":"string"}},"additionalProperties":false}`

// classify is a model's decision: sales when the text says buy, else support. Its calls are counted.
type classify struct{ calls *atomic.Int64 }

func (classify) Spec() graph.NodeSpec {
	return graph.NodeSpec{Type: "acme.classify", Title: "Classify", Behavior: graph.BehaviorCall, DryRun: graph.DryRunExecute,
		Params: graph.MustSchemaParams(textParams), Templates: []string{"text"}, Outcomes: []string{"sales", "support", "failed"}, FailedOutcome: "failed",
		Output: expr.Fields{{Name: "label", Field: expr.OptionalOf(expr.StringField)}, {Name: "usage", Field: expr.OptionalOf(expr.Field{Kind: expr.Dyn})}}}
}
func (classify) Requires() []catalog.ConnectorKind { return nil }
func (classify) Idempotent() bool                  { return true }
func (c classify) Execute(_ context.Context, in catalog.CallInput) (catalog.CallResult, error) {
	c.calls.Add(1)
	text, _ := in.Params["text"].(string)
	label := "support"
	if strings.Contains(text, "buy") {
		label = "sales"
	}
	return catalog.CallResult{Outcome: label, Output: map[string]any{"label": label, "usage": map[string]any{"cost_usd": 0.01, "input_tokens": 10, "output_tokens": 1}}}, nil
}
func (classify) DecisionKey(in catalog.CallInput) (string, bool) {
	text, _ := in.Params["text"].(string)
	return engine.DecisionKey("classify", text, "acme-1", nil)
}

// send sends a message out: a backtest must never call it.
type send struct{ calls *atomic.Int64 }

func (send) Spec() graph.NodeSpec {
	return graph.NodeSpec{Type: "acme.send", Title: "Send", Behavior: graph.BehaviorCall, DryRun: graph.DryRunCapture,
		Params: graph.MustSchemaParams(textParams), Templates: []string{"text"}, Outcomes: []string{"sent", "failed"}, FailedOutcome: "failed", DoneOutcome: "sent",
		Output: expr.Fields{{Name: "message_id", Field: expr.OptionalOf(expr.StringField)}}}
}
func (send) Requires() []catalog.ConnectorKind { return nil }
func (send) Idempotent() bool                  { return false }
func (s send) Execute(context.Context, catalog.CallInput) (catalog.CallResult, error) {
	s.calls.Add(1)
	return catalog.CallResult{Outcome: "sent", Output: map[string]any{"message_id": "m-1"}}, nil
}

// graphV1 routes by the classifier's decision on the message; graphV2 classifies a fixed text, so every
// message goes to sales.
const graphV1 = `{"name": "Route", "triggers": [{"id": "m", "event": "acme.message.v1"}],
 "nodes": [
  {"id": "cls", "type": "acme.classify", "params": {"text": "{{ event.text }}"}},
  {"id": "sales", "type": "acme.send", "params": {"text": "to sales: {{ event.text }}"}},
  {"id": "support", "type": "acme.send", "params": {"text": "to support"}},
  {"id": "fin", "type": "end", "params": {}}
 ],
 "edges": [
  {"from": {"node": "m", "outcome": "fired"}, "to": "cls"},
  {"from": {"node": "cls", "outcome": "sales"}, "to": "sales"},
  {"from": {"node": "cls", "outcome": "support"}, "to": "support"},
  {"from": {"node": "cls", "outcome": "failed"}, "to": "fin"},
  {"from": {"node": "sales", "outcome": "sent"}, "to": "fin"}, {"from": {"node": "sales", "outcome": "failed"}, "to": "fin"},
  {"from": {"node": "support", "outcome": "sent"}, "to": "fin"}, {"from": {"node": "support", "outcome": "failed"}, "to": "fin"}
 ]}`

var graphV2 = strings.Replace(graphV1, `"text": "{{ event.text }}"}}`, `"text": "buy {{ event.text }}"}}`, 1)

type env struct {
	t         *testing.T
	db        *pgxpool.Pool
	tenant    uuid.UUID
	subjects  *hosttest.Subjects
	rt        *engine.Runtime
	svc       *backtest.Svc
	decisions *atomic.Int64
	sent      *atomic.Int64
	store     connectors.Store
	dir       connectors.Directory
}

var allPerms = []string{engine.PermRead, engine.PermAuthor, engine.PermEnable, engine.PermOperate}

func newEnv(t *testing.T) *env { return newEnvWith(t, nil) }

// newEnvWith is newEnv whose catalog and backtest configuration extra adjusts (it runs before the
// runtime is built).
func newEnvWith(t *testing.T, extra func(e *env, reg catalog.Registry, cfg *backtest.Config)) *env {
	t.Helper()
	h := itf.NewHarness(t, itf.HarnessConfig{
		Name:      "automation_backtest",
		Database:  itf.DatabaseConfig{Provisioning: itf.ProvisioningPerTestDatabase},
		Isolation: itf.IsolationConfig{Mode: itf.IsolationCommitted},
	})
	scope := h.Scope(t)
	e := &env{t: t, db: scope.Pool, tenant: scope.Tenant.ID, subjects: hosttest.NewSubjects(), decisions: &atomic.Int64{}, sent: &atomic.Int64{}}
	e.subjects.Grant(1, allPerms...)
	e.subjects.Grant(2, allPerms...)
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	keys, err := connectors.NewKeyring("k1", map[string][]byte{"k1": []byte("0123456789abcdef0123456789abcdef")})
	require.NoError(t, err)
	e.store = connectors.NewStore(e.db, keys, nil, nil, nil)
	reg := catalog.NewRegistry(e.store)
	e.dir = connectors.NewDirectory(reg)
	require.NoError(t, reg.RegisterTriggers(engine.Triggers()))
	require.NoError(t, reg.RegisterTriggers(catalog.FromNativeEvents(nativeEvents{messageEvent})))
	for _, spec := range append(graph.BuiltinNodes(nil, []string{evMessage}), graph.FlowNodes(nil)...) {
		require.NoError(t, reg.RegisterNode(specNode{spec}))
	}
	require.NoError(t, reg.RegisterNode(classify{calls: e.decisions}))
	require.NoError(t, reg.RegisterNode(send{calls: e.sent}))
	var rt *engine.Runtime
	wf, err := workflow.New(workflow.Config{
		DB: e.db, Subjects: e.subjects, Logger: logger,
		NativeSource: func() (map[string]workflow.NativeContract, error) { return rt.Natives(), nil },
	})
	require.NoError(t, err)
	t.Cleanup(wf.Shutdown)
	rt, err = engine.NewRuntime(engine.Config{DB: e.db, Workflows: wf, Users: e.subjects, Catalog: reg, Logger: logger})
	require.NoError(t, err)
	e.rt = rt
	cfg := backtest.Config{DB: e.db, Automations: rt.Service(), Authoring: rt.Authoring(), Catalog: reg, Users: e.subjects,
		Logger: logger, LeaseTTL: time.Second}
	if extra != nil {
		extra(e, reg, &cfg)
	}
	e.svc, err = backtest.New(cfg)
	require.NoError(t, err)
	return e
}

func (e *env) actor(user int64, origin engine.Origin) engine.Actor {
	return engine.Actor{TenantID: e.tenant, UserID: user, Origin: origin,
		Subject: spi.Subject{TenantID: e.tenant, UserID: user, Kind: spi.SubjectInitiator, Permissions: allPerms}}
}

func (e *env) human() engine.Actor { return e.actor(1, engine.OriginHuman) }

func (e *env) publish(key string, m message, at time.Time) {
	e.t.Helper()
	ctx := context.Background()
	tx, err := e.db.Begin(ctx)
	require.NoError(e.t, err)
	defer func() { _ = tx.Rollback(ctx) }()
	id, err := events.Publish(ctx, tx, spi.Scope{Subject: spi.Subject{TenantID: e.tenant}}, messageEvent, &spi.ObjectRef{Entity: "acme.client", Key: key}, m)
	require.NoError(e.t, err)
	_, err = tx.Exec(ctx, `UPDATE extruntime.outbox_events SET occurred_at = $2 WHERE id = $1`, id, at)
	require.NoError(e.t, err)
	require.NoError(e.t, tx.Commit(ctx))
}

// drain runs the worker until no job is due.
func (e *env) drain() {
	e.t.Helper()
	for i := 0; i < 50; i++ {
		worked, err := e.svc.RunOnce(context.Background())
		require.NoError(e.t, err)
		if !worked {
			return
		}
	}
	e.t.Fatal("the worker did not run dry")
}

func (e *env) run(a engine.Actor, in backtest.RunInput) backtest.Report {
	e.t.Helper()
	ctx := context.Background()
	id, err := e.svc.Run(ctx, a, in)
	require.NoError(e.t, err)
	e.drain()
	rep, err := e.svc.Report(ctx, a, id)
	require.NoError(e.t, err)
	require.Equal(e.t, backtest.StatusDone, rep.Backtest.Status, rep.Backtest.Error)
	return rep
}

func classifier(rep backtest.Report, node string) backtest.ClassifierMetrics {
	for _, c := range rep.Classifiers {
		if c.Node == node {
			return c
		}
	}
	return backtest.ClassifierMetrics{}
}

func outcome(c backtest.ClassifierMetrics, o string) backtest.OutcomeMetrics {
	for _, m := range c.Outcomes {
		if m.Outcome == o {
			return m
		}
	}
	return backtest.OutcomeMetrics{}
}

// A dataset of 30 days of the journal runs on two versions of the graph; the comparison names the
// examples whose route changed; nothing is sent; the journal's pruning keeps the dataset; a repeated
// cached backtest does not call the provider.
func TestJournalDatasetOnTwoVersions(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	a := e.human()
	am, _, err := e.rt.Service().Create(ctx, a, engine.CreateInput{Graph: json.RawMessage(graphV1)})
	require.NoError(t, err)
	now := time.Now()
	e.publish("c-1", message{Text: "I want to buy", Secret: "pin-1"}, now.Add(-2*24*time.Hour))
	e.publish("c-2", message{Text: "it broke"}, now.Add(-10*24*time.Hour))
	e.publish("c-3", message{Text: "refund please"}, now.Add(-20*24*time.Hour))
	e.publish("c-old", message{Text: "older than the window"}, now.Add(-40*24*time.Hour))

	ds, err := e.svc.CreateDataset(ctx, a, backtest.CreateDatasetInput{AutomationID: am.ID, Name: "Last 30 days"})
	require.NoError(t, err)
	job, err := e.svc.Import(ctx, a, ds, backtest.ImportSource{Kind: backtest.ImportJournal, Window: backtest.TimeWindow{From: now.Add(-30 * 24 * time.Hour)}})
	require.NoError(t, err)
	e.drain()
	ij, err := e.svc.ImportStatus(ctx, a, job)
	require.NoError(t, err)
	assert.Equal(t, backtest.StatusDone, ij.Status, ij.Error)
	assert.Equal(t, 3, ij.Imported)

	// The same window again brings nothing new.
	_, err = e.svc.Import(ctx, a, ds, backtest.ImportSource{Kind: backtest.ImportJournal, Window: backtest.TimeWindow{From: now.Add(-30 * 24 * time.Hour)}})
	require.NoError(t, err)
	e.drain()
	d, err := e.svc.GetDataset(ctx, a, ds)
	require.NoError(t, err)
	assert.Equal(t, 3, d.Examples)
	assert.Equal(t, 3, d.Imports[0].Duplicates)

	// A sensitive field the author may not read is not copied into the dataset.
	page, err := e.svc.Examples(ctx, a, ds, backtest.ExampleFilter{})
	require.NoError(t, err)
	require.Len(t, page.Items, 3)
	for _, x := range page.Items {
		assert.NotContains(t, string(x.Event), "pin-1")
		assert.Equal(t, "acme.client", x.Subject.Entity)
	}

	// Pruning the journal keeps the dataset.
	_, err = e.db.Exec(ctx, `DELETE FROM extruntime.outbox_events WHERE tenant_id = $1`, e.tenant)
	require.NoError(t, err)

	v2, err := e.rt.Service().SaveVersion(ctx, a, am.ID, engine.SaveInput{BaseVersion: 1, Graph: json.RawMessage(graphV2)})
	require.NoError(t, err)

	r1 := e.run(a, backtest.RunInput{DatasetID: ds, Version: 1})
	assert.Equal(t, 3, r1.Examples)
	assert.Equal(t, 3, r1.Ends[backtest.EndEnded])
	assert.Equal(t, 3, r1.Cost.LiveCalls, "the first cached backtest decides live")
	assert.InDelta(t, 0.03, r1.Cost.LiveUSD, 1e-9)
	cls := classifier(r1, "cls")
	assert.True(t, cls.Decision)
	assert.Equal(t, map[string]int{"sales": 1, "support": 2}, cls.Distribution)
	effects := map[string]int{}
	for _, s := range r1.Effects {
		effects[s.Node] = s.Count
	}
	assert.Equal(t, map[string]int{"sales": 1, "support": 2}, effects, "the sends the graph would have made")

	r2 := e.run(a, backtest.RunInput{DatasetID: ds, Version: v2.Version})
	assert.Equal(t, map[string]int{"sales": 3}, classifier(r2, "cls").Distribution)

	cmp, err := e.svc.Compare(ctx, a, r1.Backtest.ID, r2.Backtest.ID)
	require.NoError(t, err)
	assert.Equal(t, 3, cmp.CommonExamples)
	assert.Equal(t, 2, cmp.Changed, "the two support messages go to sales in v2")
	for _, df := range cmp.Diffs {
		assert.Contains(t, df.Changes, backtest.OutcomeChange{Node: "cls", A: "support", B: "sales"})
		assert.Equal(t, []string{"support"}, df.EffectsA)
		assert.Equal(t, []string{"sales"}, df.EffectsB)
	}

	// Again on v1, cached: every decision comes from the cache.
	calls := e.decisions.Load()
	r3 := e.run(a, backtest.RunInput{DatasetID: ds, Version: 1, Decisions: backtest.DecisionsCached})
	assert.Equal(t, calls, e.decisions.Load(), "a repeated cached backtest calls no provider")
	assert.Equal(t, 3, r3.Cost.CachedCalls)
	assert.Zero(t, r3.Cost.LiveCalls)
	assert.Zero(t, r3.Backtest.LiveCalls)

	assert.Zero(t, e.sent.Load(), "a backtest sends nothing")
	var runs int
	require.NoError(t, e.db.QueryRow(ctx, `SELECT count(*) FROM automation.runs`).Scan(&runs))
	assert.Zero(t, runs)

	n, err := e.svc.Count(ctx, a, am.ID)
	require.NoError(t, err)
	assert.Equal(t, 3, n)
	est, err := e.svc.Estimate(ctx, a, backtest.RunInput{DatasetID: ds, Decisions: backtest.DecisionsLive})
	require.NoError(t, err)
	assert.Equal(t, []string{"cls"}, est.DecisionNodes)
	assert.Equal(t, 3, est.MaxCalls)
	require.NotNil(t, est.CostUSD)
	assert.InDelta(t, 0.03, *est.CostUSD, 1e-9)
}

// A synthetic event's expected outcome counts in precision and recall; a manual label overrides it.
func TestSyntheticLabelsAndManualOverride(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	a := e.human()
	am, _, err := e.rt.Service().Create(ctx, a, engine.CreateInput{Graph: json.RawMessage(graphV1)})
	require.NoError(t, err)
	ds, err := e.svc.CreateDataset(ctx, a, backtest.CreateDatasetInput{AutomationID: am.ID, Name: "Hand-made"})
	require.NoError(t, err)
	add := func(text, want string) uuid.UUID {
		id, err := e.svc.AddSynthetic(ctx, a, ds, backtest.SyntheticEvent{Kind: evMessage, Payload: json.RawMessage(`{"text": "` + text + `"}`),
			Subject: engine.SubjectRef{Entity: "acme.client", Key: "c-9"}, Title: text, Expected: map[string]string{"cls": want}})
		require.NoError(t, err)
		return id
	}
	add("buy now", "sales")
	add("help", "support")
	wrong := add("I may buy later", "support") // the classifier says sales: a mismatch

	rep := e.run(a, backtest.RunInput{DatasetID: ds})
	cls := classifier(rep, "cls")
	assert.Equal(t, 3, cls.Labeled)
	assert.Equal(t, 2, cls.Correct)
	sales := outcome(cls, "sales")
	assert.Equal(t, 1, sales.TruePositives)
	assert.Equal(t, 1, sales.FalsePositives)
	require.NotNil(t, sales.Precision)
	assert.InDelta(t, 0.5, *sales.Precision, 1e-9)
	require.NotNil(t, sales.Recall)
	assert.InDelta(t, 1.0, *sales.Recall, 1e-9)
	support := outcome(cls, "support")
	assert.InDelta(t, 0.5, *support.Recall, 1e-9)
	require.Len(t, rep.Mismatches, 1)
	assert.Equal(t, backtest.Mismatch{ExampleID: wrong, Title: "I may buy later", OccurredAt: rep.Mismatches[0].OccurredAt,
		Node: "cls", Expected: "support", Actual: "sales", LabelSource: backtest.LabelExpected}, rep.Mismatches[0])

	// The person says it is sales after all: the manual label wins, and the labels carry over to the
	// same backtest's report and to the next backtest.
	require.NoError(t, e.svc.Label(ctx, a, wrong, "cls", "sales"))
	rep, err = e.svc.Report(ctx, a, rep.Backtest.ID)
	require.NoError(t, err)
	cls = classifier(rep, "cls")
	assert.Equal(t, 3, cls.Correct)
	assert.Empty(t, rep.Mismatches)
	page, err := e.svc.Examples(ctx, a, ds, backtest.ExampleFilter{})
	require.NoError(t, err)
	for _, x := range page.Items {
		if x.ID == wrong {
			assert.Equal(t, backtest.Label{Outcome: "sales", Source: backtest.LabelManual, Expected: "support"}, x.Labels["cls"])
		}
	}
	res, err := e.svc.Results(ctx, a, rep.Backtest.ID, backtest.ResultFilter{Node: "cls"})
	require.NoError(t, err)
	assert.Equal(t, 3, res.Total)

	// Removing the manual label brings the expected outcome back.
	require.NoError(t, e.svc.Label(ctx, a, wrong, "cls", ""))
	res, err = e.svc.Results(ctx, a, rep.Backtest.ID, backtest.ResultFilter{Node: "cls", Mismatch: true})
	require.NoError(t, err)
	require.Equal(t, 1, res.Total)
	assert.Equal(t, wrong, res.Items[0].Example.ID)
	assert.Equal(t, []string{"cls"}, res.Items[0].Mismatches)
	assert.Zero(t, e.sent.Load())
}

// An agent may build a dataset and run a cached backtest, which reads the decision cache only; live is a
// person's.
func TestAgentRunsCachedOnly(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	agent := e.actor(2, engine.OriginAgent)
	am, _, err := e.rt.Service().Create(ctx, agent, engine.CreateInput{Graph: json.RawMessage(graphV1)})
	require.NoError(t, err)
	ds, err := e.svc.CreateDataset(ctx, agent, backtest.CreateDatasetInput{AutomationID: am.ID, Name: "Agent's"})
	require.NoError(t, err)
	_, err = e.svc.AddSynthetic(ctx, agent, ds, backtest.SyntheticEvent{Kind: evMessage, Payload: json.RawMessage(`{"text": "buy"}`), Expected: map[string]string{"cls": "sales"}})
	require.NoError(t, err)

	_, err = e.svc.Run(ctx, agent, backtest.RunInput{DatasetID: ds, Decisions: backtest.DecisionsLive})
	require.ErrorIs(t, err, engine.ErrHumanRequired)

	rep := e.run(agent, backtest.RunInput{DatasetID: ds})
	assert.Zero(t, e.decisions.Load(), "an agent's backtest calls no provider")
	assert.Equal(t, 1, rep.Cost.Missing)
	assert.Equal(t, 1, classifier(rep, "cls").Undecided)
	assert.Empty(t, rep.Mismatches, "a decision missing from the cache is not a mismatch")

	// Once a person's backtest decided it, the agent's reads it from the cache.
	e.run(e.human(), backtest.RunInput{DatasetID: ds})
	assert.Equal(t, int64(1), e.decisions.Load())
	rep = e.run(agent, backtest.RunInput{DatasetID: ds})
	assert.Equal(t, int64(1), e.decisions.Load())
	assert.Equal(t, 1, rep.Cost.CachedCalls)
	assert.Equal(t, 1, classifier(rep, "cls").Correct)
}

// Limits and rights: 20 datasets per automation, Automations.Author for everything, a cancelled
// backtest stays cancelled.
func TestLimitsAndRights(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	a := e.human()
	am, _, err := e.rt.Service().Create(ctx, a, engine.CreateInput{Graph: json.RawMessage(graphV1)})
	require.NoError(t, err)
	var first uuid.UUID
	for i := 0; i < backtest.MaxDatasets; i++ {
		id, err := e.svc.CreateDataset(ctx, a, backtest.CreateDatasetInput{AutomationID: am.ID, Name: "d"})
		require.NoError(t, err)
		if i == 0 {
			first = id
		}
	}
	_, err = e.svc.CreateDataset(ctx, a, backtest.CreateDatasetInput{AutomationID: am.ID, Name: "one too many"})
	require.ErrorIs(t, err, engine.ErrLimitExceeded)

	reader := a
	reader.Subject.Permissions = []string{engine.PermRead}
	_, err = e.svc.ListDatasets(ctx, reader, am.ID)
	require.ErrorIs(t, err, engine.ErrForbidden)

	_, err = e.svc.Run(ctx, a, backtest.RunInput{DatasetID: first})
	require.ErrorIs(t, err, engine.ErrInvalidInput, "an empty dataset does not run")
	_, err = e.svc.AddSynthetic(ctx, a, first, backtest.SyntheticEvent{Kind: evMessage, Payload: json.RawMessage(`{"text": "x"}`)})
	require.NoError(t, err)
	id, err := e.svc.Run(ctx, a, backtest.RunInput{DatasetID: first})
	require.NoError(t, err)
	require.NoError(t, e.svc.Cancel(ctx, a, id))
	e.drain()
	rep, err := e.svc.Report(ctx, a, id)
	require.NoError(t, err)
	assert.Equal(t, backtest.StatusCancelled, rep.Backtest.Status)
	assert.Zero(t, e.decisions.Load())

	// Another tenant's dataset is not found.
	other := engine.Actor{TenantID: uuid.New(), UserID: 1, Origin: engine.OriginHuman, Subject: spi.Subject{Permissions: allPerms}}
	_, err = e.svc.GetDataset(ctx, other, first)
	require.ErrorIs(t, err, engine.ErrNotFound)
}

const (
	kindMail = catalog.ConnectorKind("acmemail")
	evMail   = "acmemail.received.v1"
)

// mailConnector is a connector whose history is a list of messages; n more are generated after them.
type mailConnector struct {
	history []backtest.Event
	n       int
}

func (m *mailConnector) Kind() catalog.ConnectorKind { return kindMail }
func (m *mailConnector) Descriptor() connectors.Descriptor {
	return connectors.Descriptor{Kind: kindMail, Title: map[string]string{"en": "Mail"}, Auth: connectors.AuthAPIKey, Inbound: true,
		Credentials: json.RawMessage(`{"type":"object","properties":{"token":{"type":"string","x-secret":true}},"additionalProperties":false}`)}
}
func (m *mailConnector) Test(context.Context, connectors.Credentials) (connectors.Identity, error) {
	return connectors.Identity{Display: "box"}, nil
}
func (m *mailConnector) Nodes() []catalog.NodeType          { return nil }
func (m *mailConnector) Triggers() []catalog.TriggerSource { return []catalog.TriggerSource{mailEvents{}} }

type mailEvents struct{}

func (mailEvents) Requires() []catalog.ConnectorKind { return nil }
func (mailEvents) Events() []graph.EventSpec {
	return []graph.EventSpec{{Key: evMail, Title: "Mail", Subject: graph.SubjectConversation,
		Payload: expr.Fields{{Name: "text", Field: expr.StringField}}}}
}

func (m *mailConnector) ImportHistory(ctx context.Context, ref backtest.ConnectionRef, w backtest.TimeWindow, _ json.RawMessage, emit func(backtest.Event) error) error {
	if ref.Credentials["token"] != "tok" {
		return errors.New("not opened with its credentials")
	}
	for _, ev := range m.history {
		if err := emit(ev); err != nil {
			return err
		}
	}
	for i := 0; i < m.n; i++ {
		if err := emit(backtest.Event{Kind: evMail, Key: fmt.Sprintf("gen-%d", i), Subject: engine.SubjectRef{Entity: graph.SubjectConversation, Key: "c"},
			Payload: json.RawMessage(`{"text": "generated"}`), OccurredAt: w.From.Add(time.Minute)}); err != nil {
			return err
		}
	}
	return nil
}

const mailGraph = `{"name": "Mail", "triggers": [{"id": "m", "event": "acmemail.received.v1"}],
 "nodes": [{"id": "cls", "type": "acme.classify", "params": {"text": "{{ event.text }}"}}, {"id": "fin", "type": "end", "params": {}}],
 "edges": [{"from": {"node": "m", "outcome": "fired"}, "to": "cls"},
  {"from": {"node": "cls", "outcome": "sales"}, "to": "fin"}, {"from": {"node": "cls", "outcome": "support"}, "to": "fin"}, {"from": {"node": "cls", "outcome": "failed"}, "to": "fin"}]}`

// A connection's history is imported through its connector, by the one who may use the connection; an
// event already in the dataset is not imported twice; past the limit of a dataset the import says how
// many events were left out.
func TestConnectionHistoryImport(t *testing.T) {
	mc := &mailConnector{}
	e := newEnvWith(t, func(e *env, _ catalog.Registry, cfg *backtest.Config) {
		require.NoError(t, e.dir.Register(mc))
		cfg.Connections, cfg.Connectors = e.store, e.dir
	})
	ctx := context.Background()
	a := e.human()
	conn, err := e.store.Save(ctx, connectors.Connection{TenantID: e.tenant, Kind: kindMail, Name: "box", DisplayName: "Box", CreatedBy: 1},
		connectors.Credentials{"token": "tok"})
	require.NoError(t, err)
	am, _, err := e.rt.Service().Create(ctx, a, engine.CreateInput{Graph: json.RawMessage(mailGraph)})
	require.NoError(t, err)
	ds, err := e.svc.CreateDataset(ctx, a, backtest.CreateDatasetInput{AutomationID: am.ID, Name: "Mailbox"})
	require.NoError(t, err)
	now := time.Now()
	mc.history = []backtest.Event{
		{Kind: evMail, Key: "<1@x>", Title: "Buy", Subject: engine.SubjectRef{Entity: graph.SubjectConversation, Key: "c-1"}, Payload: json.RawMessage(`{"text": "buy", "extra": "dropped"}`), OccurredAt: now.Add(-time.Hour)},
		{Kind: evMail, Key: "<1@x>", Title: "Buy again", Payload: json.RawMessage(`{"text": "buy"}`), OccurredAt: now.Add(-time.Hour)},
		{Kind: "acme.unknown.v1", Key: "<2@x>", Payload: json.RawMessage(`{}`)},
	}

	src, err := e.svc.Sources(ctx, a, am.ID)
	require.NoError(t, err)
	require.Len(t, src, 1)
	assert.Equal(t, backtest.SourceOption{Kind: backtest.ImportConnection, ConnectionID: conn.ID, Title: "Box", ConnectorKind: string(kindMail)}, src[0])

	// Another author without the right to the connection may not import its history.
	other := e.actor(2, engine.OriginHuman)
	_, err = e.svc.Import(ctx, other, ds, backtest.ImportSource{Kind: backtest.ImportConnection, ConnectionID: conn.ID, Window: backtest.TimeWindow{From: now.Add(-24 * time.Hour)}})
	require.ErrorIs(t, err, engine.ErrForbidden)
	srcOther, err := e.svc.Sources(ctx, other, am.ID)
	require.NoError(t, err)
	assert.Empty(t, srcOther)

	job, err := e.svc.Import(ctx, a, ds, backtest.ImportSource{Kind: backtest.ImportConnection, ConnectionID: conn.ID, Window: backtest.TimeWindow{From: now.Add(-24 * time.Hour)}})
	require.NoError(t, err)
	e.drain()
	ij, err := e.svc.ImportStatus(ctx, a, job)
	require.NoError(t, err)
	assert.Equal(t, backtest.StatusDone, ij.Status, ij.Error)
	assert.Equal(t, 1, ij.Imported)
	assert.Equal(t, 1, ij.Duplicates)
	assert.Equal(t, 1, ij.Unreadable)
	page, err := e.svc.Examples(ctx, a, ds, backtest.ExampleFilter{})
	require.NoError(t, err)
	require.Len(t, page.Items, 1)
	assert.Equal(t, backtest.SourceHistory, page.Items[0].Source)
	assert.JSONEq(t, `{"kind": "acmemail.received.v1", "text": "buy"}`, string(page.Items[0].Event), "undeclared fields are not kept")

	rep := e.run(a, backtest.RunInput{DatasetID: ds})
	assert.Equal(t, map[string]int{"sales": 1}, classifier(rep, "cls").Distribution)

	// Past MaxExamples the import counts what it left out.
	mc.history, mc.n = nil, backtest.MaxExamples+2
	job, err = e.svc.Import(ctx, a, ds, backtest.ImportSource{Kind: backtest.ImportConnection, ConnectionID: conn.ID, Window: backtest.TimeWindow{From: now.Add(-24 * time.Hour)}})
	require.NoError(t, err)
	e.drain()
	ij, err = e.svc.ImportStatus(ctx, a, job)
	require.NoError(t, err)
	assert.Equal(t, backtest.MaxExamples-1, ij.Imported)
	assert.Equal(t, 3, ij.Overflow)
}
