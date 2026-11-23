package engine_test

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/pkg/automation/catalog"
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

type order struct {
	Amount int    `json:"amount"`
	Note   string `json:"note"`
	Secret string `json:"secret"`
}

const (
	evCreated = "acme.order.created.v1"
	evPaid    = "acme.order.paid.v1"
	permRead  = "Orders.Read"
	permSec   = "Orders.Secret"
)

var (
	orderCreated = native.Event[order](evCreated, native.EventSpec{Subject: "acme.order", ReadPermission: permRead, Sensitive: map[string][]string{"secret": {permSec}}})
	orderPaid    = native.Event[order](evPaid, native.EventSpec{Subject: "acme.order"})
)

type nativeEvents []native.EventDefinition

func (n nativeEvents) Events() []native.EventDefinition { return n }

// specNode is a node type of a fixed spec.
type specNode struct{ spec graph.NodeSpec }

func (n specNode) Spec() graph.NodeSpec              { return n.spec }
func (n specNode) Requires() []catalog.ConnectorKind { return nil }

// noteNode is an effect: it writes a row of test_effects in the step's transaction.
type noteNode struct{ specNode }

// emitNode publishes acme.order.created.v1 about the run's subject in the step's transaction.
type emitNode struct{ specNode }

func (emitNode) ApplyEffect(ctx context.Context, tx pgx.Tx, run engine.StepContext, _ graph.Node, _ graph.Effect) (map[string]any, error) {
	_, err := events.Publish(ctx, tx, spi.Scope{Subject: spi.Subject{TenantID: run.TenantID}}, orderCreated, &spi.ObjectRef{Entity: "acme.order", Key: run.Subject.Key}, order{Amount: 1})
	return map[string]any{}, err
}

func (n *noteNode) ApplyEffect(ctx context.Context, tx pgx.Tx, run engine.StepContext, node graph.Node, e graph.Effect) (map[string]any, error) {
	text, _ := e.Params["text"].(string)
	if _, err := tx.Exec(ctx, `INSERT INTO test_effects (run_id, node_id, text) VALUES ($1, $2, $3)`, run.RunID, node.ID, text); err != nil {
		return nil, err
	}
	return map[string]any{"text": text}, nil
}

// permNode records whether the step's run-as holds the secret permission now.
type permNode struct{ specNode }

func (permNode) ApplyEffect(ctx context.Context, tx pgx.Tx, run engine.StepContext, node graph.Node, _ graph.Effect) (map[string]any, error) {
	text := "lacks"
	if run.RunAs.Has(permSec) {
		text = "has"
	}
	_, err := tx.Exec(ctx, `INSERT INTO test_effects (run_id, node_id, text) VALUES ($1, $2, $3)`, run.RunID, node.ID, text)
	return map[string]any{}, err
}

// codeNode is a node with code a human reviews.
type codeNode struct{ specNode }

func (codeNode) Review(params map[string]any) engine.CodeNodeReview {
	src, _ := params["source"].(string)
	return engine.CodeNodeReview{Lang: "ts", Source: src, Secrets: []string{"TOKEN"}}
}

func (n codeNode) ApplyEffect(context.Context, pgx.Tx, engine.StepContext, graph.Node, graph.Effect) (map[string]any, error) {
	return map[string]any{}, nil
}

// approveNode is a human task.
type approveNode struct{ specNode }

func (approveNode) Human() bool { return true }

// tasks is the TaskBridge double: a task for user 1.
type tasks struct{}

func (tasks) Begin(_ context.Context, _ pgx.Tx, run engine.StepContext, _ graph.Node, _ json.RawMessage) (engine.WaitSpec, error) {
	user := int64(1)
	return engine.WaitSpec{Kind: "task", Task: &workflow.TaskSpec{Assignees: []workflow.TaskAssignee{{Kind: workflow.TaskAssignUser, UserID: &user}}}}, nil
}

type env struct {
	t        *testing.T
	db       *pgxpool.Pool
	tenant   uuid.UUID
	subjects *hosttest.Subjects
	reg      catalog.Registry
	note     *noteNode
	logger   *slog.Logger
}

var allPerms = []string{engine.PermRead, engine.PermAuthor, engine.PermEnable, engine.PermOperate, permRead, permSec}

func newEnv(t *testing.T) *env {
	t.Helper()
	h := itf.NewHarness(t, itf.HarnessConfig{
		Name:      "automation_engine",
		Database:  itf.DatabaseConfig{Provisioning: itf.ProvisioningPerTestDatabase},
		Isolation: itf.IsolationConfig{Mode: itf.IsolationCommitted},
	})
	scope := h.Scope(t)
	e := &env{t: t, db: scope.Pool, tenant: scope.Tenant.ID, subjects: hosttest.NewSubjects(),
		logger: slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))}
	_, err := e.db.Exec(context.Background(), `CREATE TABLE test_effects (run_id uuid NOT NULL, node_id text NOT NULL, text text NOT NULL)`)
	require.NoError(t, err)
	e.subjects.Grant(1, allPerms...)
	e.subjects.Grant(2, allPerms...)

	reg := catalog.NewRegistry(nil)
	require.NoError(t, reg.RegisterTriggers(engine.Triggers()))
	require.NoError(t, reg.RegisterTriggers(catalog.FromNativeEvents(nativeEvents{orderCreated, orderPaid})))
	for _, spec := range append(graph.BuiltinNodes(nil, []string{evPaid, evCreated, engine.EventRunFinished}), graph.FlowNodes(nil)...) {
		require.NoError(t, reg.RegisterNode(specNode{spec}))
	}
	noteSpec := graph.NodeSpec{Type: "acme.note", Title: "Note", Behavior: graph.BehaviorEffect, FailedOutcome: "failed", Outcomes: []string{"done", "failed"},
		Params:    graph.MustSchemaParams(`{"type":"object","properties":{"text":{"type":"string"}},"required":["text"],"additionalProperties":false}`),
		Templates: []string{"text"}, Output: expr.Fields{{Name: "text", Field: expr.StringField}}}
	e.note = &noteNode{specNode: specNode{noteSpec}}
	require.NoError(t, reg.RegisterNode(e.note))
	permSpec := graph.NodeSpec{Type: "acme.perm", Title: "Perm", Behavior: graph.BehaviorEffect,
		Params: graph.MustSchemaParams(`{"type":"object","properties":{},"additionalProperties":false}`)}
	require.NoError(t, reg.RegisterNode(permNode{specNode{permSpec}}))
	emitSpec := graph.NodeSpec{Type: "acme.emit", Title: "Emit", Behavior: graph.BehaviorEffect,
		Params: graph.MustSchemaParams(`{"type":"object","properties":{},"additionalProperties":false}`)}
	require.NoError(t, reg.RegisterNode(emitNode{specNode{emitSpec}}))
	codeSpec := graph.NodeSpec{Type: "acme.code", Title: "Code", Behavior: graph.BehaviorEffect,
		Params: graph.MustSchemaParams(`{"type":"object","properties":{"source":{"type":"string"}},"required":["source"],"additionalProperties":false}`)}
	require.NoError(t, reg.RegisterNode(codeNode{specNode{codeSpec}}))
	approveSpec := graph.NodeSpec{Type: "acme.approve", Title: "Approve", Behavior: graph.BehaviorCall, Outcomes: []string{"approve", "reject"},
		Params: graph.MustSchemaParams(`{"type":"object","properties":{},"additionalProperties":false}`)}
	require.NoError(t, reg.RegisterNode(approveNode{specNode{approveSpec}}))
	e.reg = reg
	return e
}

func (e *env) newTenant() uuid.UUID {
	e.t.Helper()
	id := uuid.New()
	_, err := e.db.Exec(context.Background(),
		`INSERT INTO tenants (id, name, domain, created_at, updated_at) VALUES ($1, $2, $3, NOW(), NOW())`, id, "t-"+id.String()[:8], id.String()[:8]+".test")
	require.NoError(e.t, err)
	return id
}

// proc is one process: a workflow engine, the automation runtime and the outbox dispatcher. Two procs
// on one env are a restart on the same database.
type proc struct {
	e    *env
	wf   *workflow.Engine
	rt   *engine.Runtime
	disp *events.Dispatcher
}

func (e *env) proc(tune ...func(*engine.Config)) *proc {
	e.t.Helper()
	p := &proc{e: e}
	wf, err := workflow.New(workflow.Config{
		DB: e.db, Subjects: e.subjects,
		NativeSource: func() (map[string]workflow.NativeContract, error) { return p.rt.Natives(), nil },
		LeaseTTL:     3 * time.Second, Heartbeat: time.Second, PollInterval: 50 * time.Millisecond, RetryBackoff: 10 * time.Millisecond,
		MaxRunFailures: 20, Logger: e.logger,
	})
	require.NoError(e.t, err)
	e.t.Cleanup(wf.Shutdown)
	cfg := engine.Config{DB: e.db, Workflows: wf, Users: e.subjects, Catalog: e.reg, Tasks: tasks{}, Logger: e.logger}
	for _, f := range tune {
		f(&cfg)
	}
	rt, err := engine.NewRuntime(cfg)
	require.NoError(e.t, err)
	p.wf, p.rt = wf, rt
	d, err := events.NewDispatcher(events.Config{
		DB: e.db, Consumers: events.StaticConsumers(rt.Consumer()), Subjects: e.subjects, PollInterval: 50 * time.Millisecond,
		BackoffBase: time.Millisecond, BackoffMax: 2 * time.Millisecond, MaxAttempts: 3, BlockedRecheck: time.Millisecond, Logger: e.logger,
	})
	require.NoError(e.t, err)
	e.t.Cleanup(d.Stop)
	p.disp = d
	return p
}

// pass delivers pending events and runs ready instances once.
func (p *proc) pass() {
	p.e.t.Helper()
	ctx := context.Background()
	_, err := p.disp.RunOnce(ctx)
	require.NoError(p.e.t, err)
	_, err = p.wf.RunOnce(ctx)
	require.NoError(p.e.t, err)
	require.NoError(p.e.t, p.rt.Maintain(ctx))
}

// until drives the process until cond holds.
func (p *proc) until(cond func() bool, msg string, args ...any) {
	p.e.t.Helper()
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		p.pass()
		if cond() {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	require.Failf(p.e.t, "condition not met", msg, args...)
}

func (e *env) actor(tenant uuid.UUID, user int64, origin engine.Origin, perms ...string) engine.Actor {
	if perms == nil {
		perms = allPerms
	}
	return engine.Actor{TenantID: tenant, UserID: user, Origin: origin, Subject: spi.Subject{TenantID: tenant, UserID: user, Kind: spi.SubjectInitiator, Permissions: perms}}
}

func (e *env) human() engine.Actor { return e.actor(e.tenant, 1, engine.OriginHuman) }

// create saves a graph and enables its first version as a human.
func (e *env) create(p *proc, a engine.Actor, g string) engine.Automation {
	e.t.Helper()
	ctx := context.Background()
	am, v, err := p.rt.Service().Create(ctx, a, engine.CreateInput{Graph: json.RawMessage(g)})
	require.NoError(e.t, err)
	rv, err := p.rt.Service().EnableReview(ctx, a, am.ID, v.Version)
	require.NoError(e.t, err)
	am, err = p.rt.Service().Enable(ctx, a, am.ID, engine.EnableInput{Version: v.Version, CodeDigest: rv.CodeDigest})
	require.NoError(e.t, err)
	return am
}

func (e *env) publish(tenant uuid.UUID, ev *native.EventDef[order], key string, o order, initiator int64) string {
	e.t.Helper()
	ctx := context.Background()
	tx, err := e.db.Begin(ctx)
	require.NoError(e.t, err)
	defer func() { _ = tx.Rollback(ctx) }()
	id, err := events.Publish(ctx, tx, spi.Scope{Subject: spi.Subject{TenantID: tenant, UserID: initiator}}, ev, &spi.ObjectRef{Entity: "acme.order", Key: key}, o)
	require.NoError(e.t, err)
	require.NoError(e.t, tx.Commit(ctx))
	return id
}

func (e *env) runs(p *proc, a engine.Actor, f engine.RunFilter) []engine.RunSummary {
	e.t.Helper()
	page, err := p.rt.Service().ListRuns(context.Background(), a, f)
	require.NoError(e.t, err)
	return page.Items
}

func (e *env) count(sql string, args ...any) int {
	e.t.Helper()
	var n int
	require.NoError(e.t, e.db.QueryRow(context.Background(), sql, args...).Scan(&n))
	return n
}

func (e *env) effects(run uuid.UUID) []string {
	e.t.Helper()
	rows, err := e.db.Query(context.Background(), `SELECT node_id || ':' || text FROM test_effects WHERE run_id = $1 ORDER BY node_id, text`, run)
	require.NoError(e.t, err)
	out, err := pgx.CollectRows(rows, pgx.RowTo[string])
	require.NoError(e.t, err)
	return out
}

func (e *env) run(p *proc, a engine.Actor, id uuid.UUID) engine.RunDetail {
	e.t.Helper()
	d, err := p.rt.Service().GetRun(context.Background(), a, id)
	require.NoError(e.t, err)
	return d
}

func (e *env) status(p *proc, id uuid.UUID) engine.RunStatus {
	e.t.Helper()
	return e.run(p, e.human(), id).Status
}

func spiSubject(tenant uuid.UUID, user int64) spi.Subject {
	return spi.Subject{TenantID: tenant, UserID: user, Kind: spi.SubjectInitiator, Permissions: allPerms}
}

// publishAt publishes an event as having happened at at: deadline tests fix the event's time against
// the wait's stored deadline instead of relying on when the test gets to publish it.
func (e *env) publishAt(tenant uuid.UUID, ev *native.EventDef[order], key string, o order, at time.Time) string {
	e.t.Helper()
	id := e.publish(tenant, ev, key, o, 0)
	_, err := e.db.Exec(context.Background(), `UPDATE extruntime.outbox_events SET occurred_at = $2 WHERE id = $1`, id, at)
	require.NoError(e.t, err)
	return id
}

// deadline is the stored deadline of the run's open event wait.
func (e *env) deadline(q interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}) time.Time {
	e.t.Helper()
	var at time.Time
	require.NoError(e.t, q.QueryRow(context.Background(), `SELECT timeout_at FROM automation.waits WHERE status = 'open' AND kind = 'event' ORDER BY created_at DESC LIMIT 1`).Scan(&at))
	return at
}
