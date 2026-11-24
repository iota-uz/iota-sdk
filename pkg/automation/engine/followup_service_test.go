package engine_test

// Regression tests of the follow-up to the engine's review (#1395).

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/pkg/automation/catalog"
	"github.com/iota-uz/iota-sdk/pkg/automation/connectors"
	"github.com/iota-uz/iota-sdk/pkg/automation/engine"
	"github.com/iota-uz/iota-sdk/pkg/automation/graph"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/host"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/spi"
)

// dbConnections is the availability of connections in automation.connections (kind smtp).
type dbConnections struct{ db *pgxpool.Pool }

func (d dbConnections) Connected(ctx context.Context, tenant uuid.UUID) (map[catalog.ConnectorKind][]catalog.ConnectionRef, error) {
	rows, err := d.db.Query(ctx, `SELECT id::text, name FROM automation.connections WHERE tenant_id = $1 AND status = 'connected' ORDER BY name`, tenant)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[catalog.ConnectorKind][]catalog.ConnectionRef{}
	for rows.Next() {
		var r catalog.ConnectionRef
		if err := rows.Scan(&r.ID, &r.Name); err != nil {
			return nil, err
		}
		out[catalog.KindSMTP] = append(out[catalog.KindSMTP], r)
	}
	return out, rows.Err()
}

// mailNode is an effect that needs an smtp connection.
type mailNode struct{ specNode }

func (mailNode) Requires() []catalog.ConnectorKind { return []catalog.ConnectorKind{catalog.KindSMTP} }
func (mailNode) ApplyEffect(context.Context, pgx.Tx, engine.StepContext, graph.Node, graph.Effect) (map[string]any, error) {
	return map[string]any{}, nil
}

// deletingCatalog deletes the connection right after the view of its call-th ForTenant: a connection
// Delete that commits between the enable's validation and its transaction.
type deletingCatalog struct {
	catalog.Registry
	db         *pgxpool.Pool
	connection uuid.UUID
	calls      atomic.Int32
	at         atomic.Int32
	deleted    atomic.Bool
}

func (c *deletingCatalog) ForTenant(ctx context.Context, tenant uuid.UUID, opts ...catalog.ViewOption) (catalog.View, error) {
	v, err := c.Registry.ForTenant(ctx, tenant, opts...)
	if n := c.calls.Add(1); err == nil && n == c.at.Load() {
		var tag pgconn.CommandTag
		if tag, err = c.db.Exec(ctx, `DELETE FROM automation.connections WHERE id = $1`, c.connection); err == nil && tag.RowsAffected() == 1 {
			c.deleted.Store(true)
		}
	}
	return v, err
}

func TestEnableIsRefusedWhenItsConnectionWasDeletedAfterValidation(t *testing.T) {
	e := newEnv(t)
	p := e.proc()
	ctx := context.Background()
	reg := catalog.NewRegistry(dbConnections{db: e.db})
	require.NoError(t, reg.RegisterTriggers(engine.Triggers()))
	require.NoError(t, reg.RegisterTriggers(catalog.FromNativeEvents(nativeEvents{orderCreated, orderPaid})))
	for _, spec := range append(graph.BuiltinNodes(nil, []string{evPaid, evCreated, engine.EventRunFinished}), graph.FlowNodes(nil)...) {
		require.NoError(t, reg.RegisterNode(specNode{spec}))
	}
	require.NoError(t, reg.RegisterNode(mailNode{specNode{graph.NodeSpec{Type: "acme.mail", Title: "Mail", Behavior: graph.BehaviorEffect,
		Params: graph.MustSchemaParams(`{"type":"object","properties":{},"additionalProperties":false}`)}}}))
	conn := uuid.New()
	_, err := e.db.Exec(ctx, `INSERT INTO automation.connections (tenant_id, id, kind, name, display_name, status, key_id, nonce, secret, version)
		VALUES ($1, $2, 'smtp', 'mail', 'Mail', 'connected', 'k1', '\x00', '\x00', 1)`, e.tenant, conn)
	require.NoError(t, err)
	cat := &deletingCatalog{Registry: reg, db: e.db, connection: conn}
	rt := runtimeOn(t, e, p, e.db, cat)
	human := e.human()
	mail := `{"name": "Mail", "triggers": [{"id": "go", "event": "automation.manual.v1"}],
	 "nodes": [{"id": "mail", "type": "acme.mail", "params": {}}, {"id": "fin", "type": "end", "params": {}}],
	 "edges": [{"from": {"node": "go", "outcome": "fired"}, "to": "mail"}, {"from": {"node": "mail", "outcome": "done"}, "to": "fin"}]}`
	baseline, bv, err := rt.Service().Create(ctx, human, engine.CreateInput{Graph: json.RawMessage(mail)})
	require.NoError(t, err)
	am, v, err := rt.Service().Create(ctx, human, engine.CreateInput{Graph: json.RawMessage(mail)})
	require.NoError(t, err)
	// With the connection there, the graph enables.
	_, err = rt.Service().Enable(ctx, human, baseline.ID, engine.EnableInput{Version: bv.Version})
	require.NoError(t, err)
	// Enable reads the catalog twice (the triggers' rights, then the validation): the delete commits
	// after the second, before the enable's transaction.
	cat.at.Store(cat.calls.Load() + 2)
	_, err = rt.Service().Enable(ctx, human, am.ID, engine.EnableInput{Version: v.Version})
	require.True(t, cat.deleted.Load(), "the connection was deleted after the validation")
	var verr *engine.ValidationError
	require.ErrorAs(t, err, &verr, "the enable checks the graph again against the catalog of now")
	require.NotEmpty(t, verr.Errors)
	assert.Equal(t, engine.CodeConnectionUnavailable, verr.Errors[0].Code, "the refusal names the missing connection: %v", verr.Errors)
	assert.Equal(t, 0, e.count(`SELECT count(*) FROM automation.automations WHERE id = $1 AND enabled_version IS NOT NULL`, am.ID))
}

// lockWaits counts the backends of the test's database that wait for a lock in a statement matching
// like, blocked by the backend by (0: by any).
func (e *env) lockWaits(like string, by uint32) int {
	return e.count(`SELECT count(*) FROM pg_stat_activity WHERE datname = current_database() AND wait_event_type = 'Lock'
		AND query LIKE $1 AND ($2::int = 0 OR $2::int = ANY(pg_blocking_pids(pid)))`, like, int(by))
}

// waitLockWait waits, bounded, until a backend waits as lockWaits counts it.
func (e *env) waitLockWait(t *testing.T, like string, by uint32, msg string) {
	t.Helper()
	require.Eventually(t, func() bool { return e.lockWaits(like, by) > 0 }, 10*time.Second, 10*time.Millisecond, msg)
}

// hookedUsers runs hook once, on the first LoadSubject after it is armed.
type hookedUsers struct {
	host.SubjectLoader
	armed atomic.Bool
	hook  func()
}

func (u *hookedUsers) LoadSubject(ctx context.Context, tenant uuid.UUID, user int64, kind spi.SubjectKind) (spi.Subject, error) {
	if u.armed.Swap(false) {
		u.hook()
	}
	return u.SubjectLoader.LoadSubject(ctx, tenant, user, kind)
}

func TestManualStartRunsAsTheOwnerOfTheEnableThatAdmitsIt(t *testing.T) {
	e := newEnv(t)
	users := &hookedUsers{SubjectLoader: e.subjects}
	p := e.proc(func(c *engine.Config) { c.Users = users })
	ctx := context.Background()
	first := e.human()
	am := e.create(p, first, `{"name": "Manual", "triggers": [{"id": "go", "event": "automation.manual.v1"}],
	 "nodes": [{"id": "fin", "type": "end", "params": {}}], "edges": [{"from": {"node": "go", "outcome": "fired"}, "to": "fin"}]}`)
	second := e.actor(e.tenant, 2, engine.OriginHuman)
	var reEnabled atomic.Bool
	users.hook = func() {
		// While the start resolves its run-as, user 2 enables the same version again: the enable either
		// commits before the start is admitted, or it waits for the start's lock on the graph row.
		done := make(chan error, 1)
		go func() {
			_, err := p.rt.Service().Enable(ctx, second, am.ID, engine.EnableInput{Version: 1})
			done <- err
		}()
		deadline := time.After(10 * time.Second)
		for {
			select {
			case err := <-done:
				require.NoError(t, err)
				reEnabled.Store(true)
				return
			case <-deadline:
				t.Fatal("the re-enable neither committed nor waited for the start")
			case <-time.After(10 * time.Millisecond):
				if e.lockWaits("%automation.automations%FOR UPDATE%", 0) > 0 {
					return // it waits for the start: admitted under the enable of before
				}
			}
		}
	}
	users.armed.Store(true)
	ref, err := p.rt.Service().StartManual(ctx, first, am.ID, engine.ManualStart{TriggerID: "go", Input: json.RawMessage(`{}`)})
	require.NoError(t, err)
	var runAs int64
	require.NoError(t, e.db.QueryRow(ctx, `SELECT run_as_user_id FROM automation.runs WHERE id = $1`, ref.RunID).Scan(&runAs))
	if reEnabled.Load() {
		assert.Equal(t, int64(2), runAs, "admitted under the new enable: its owner")
	} else {
		assert.Equal(t, int64(1), runAs, "admitted before the new enable: the owner of then")
	}
	require.Eventually(t, func() bool {
		return e.count(`SELECT count(*) FROM automation.automations WHERE id = $1 AND owner_user_id = 2`, am.ID) == 1
	}, 10*time.Second, 20*time.Millisecond, "the re-enable commits")
}

func TestRestartLeavesOutRetainedValuesWithoutAnAccessRule(t *testing.T) {
	e := newEnv(t)
	p := e.proc()
	ctx := context.Background()
	owner := e.human()
	// The paid event has no protected field (and needs no permission) now.
	e.create(p, owner, strings.Replace(noteGraph("x"), evCreated, evPaid, 1))
	e.publish(e.tenant, orderPaid, "o-1", order{Note: "n0te", Amount: 4}, 0)
	run := onlyRun(t, e, p, owner)
	p.until(func() bool { return e.status(p, run.RunID) == engine.RunDone }, "the run ends")
	// The run read note while it was protected; the field has no access rule now.
	_, err := e.db.Exec(ctx, `UPDATE automation.runs SET sensitive = '[{"event": "acme.order.paid.v1", "path": "note"}]' WHERE id = $1`, run.RunID)
	require.NoError(t, err)
	ref, err := p.rt.Service().Restart(ctx, owner, run.RunID, 1)
	require.NoError(t, err)
	var note *string
	var amount *int
	require.NoError(t, e.db.QueryRow(ctx, `SELECT input->>'note', (input->>'amount')::int FROM automation.runs WHERE id = $1`, ref.RunID).Scan(&note, &amount))
	assert.Nil(t, note, "a value whose rule cannot be checked is not replayed unrestricted")
	require.NotNil(t, amount)
	assert.Equal(t, 4, *amount, "the rest of the input is replayed")
	p.until(func() bool { return e.status(p, ref.RunID) == engine.RunDone }, "the replay runs")
}

func TestRestartKeepsInputThatOnlyAWaitEventsProvenanceNames(t *testing.T) {
	e := newEnv(t)
	p := e.proc()
	ctx := context.Background()
	owner := e.human()
	e.create(p, owner, strings.Replace(noteGraph("x"), evCreated, evPaid, 1))
	e.publish(e.tenant, orderPaid, "o-1", order{Note: "pub", Amount: 4}, 0)
	run := onlyRun(t, e, p, owner)
	p.until(func() bool { return e.status(p, run.RunID) == engine.RunDone }, "the run ends")
	// A wait of the run matched an event (retired since) whose protected field is called note too.
	_, err := e.db.Exec(ctx, `UPDATE automation.runs SET sensitive = '[{"event": "acme.retired.v1", "path": "note"}]' WHERE id = $1`, run.RunID)
	require.NoError(t, err)
	ref, err := p.rt.Service().Restart(ctx, owner, run.RunID, 1)
	require.NoError(t, err)
	var note *string
	require.NoError(t, e.db.QueryRow(ctx, `SELECT input->>'note' FROM automation.runs WHERE id = $1`, ref.RunID).Scan(&note))
	require.NotNil(t, note, "the paid event's public note is not judged by another event's provenance")
	assert.Equal(t, "pub", *note)
}

func TestQueueRecoveryReachesAFreeSubjectBehindBusyOnes(t *testing.T) {
	e := newEnv(t)
	p := e.proc()
	ctx := context.Background()
	human := e.human()
	am := e.create(p, human, `{"name": "Chat", "triggers": [{"id": "go", "event": "automation.manual.v1"}],
	 "nodes": [{"id": "nap", "type": "delay", "params": {"duration": "1h"}}, {"id": "fin", "type": "end", "params": {}}],
	 "edges": [{"from": {"node": "go", "outcome": "fired"}, "to": "nap"}, {"from": {"node": "nap", "outcome": "done"}, "to": "fin"}]}`)
	start := func(key string) engine.RunRef {
		ref, err := p.rt.Service().StartManual(ctx, human, am.ID, engine.ManualStart{TriggerID: "go", Input: json.RawMessage(`{}`),
			Subject: &engine.SubjectRef{Entity: graph.SubjectConversation, Key: key}})
		require.NoError(t, err)
		return ref
	}
	// One busy conversation, a waiting run and a queued one, is cloned 2000 times, ten scans' worth
	// takes: each clone holds a blocked instance of its own and keeps a queued run.
	head := start("a-000")
	queuedBusy := start("a-000")
	require.Equal(t, engine.RunQueued, queuedBusy.Status)
	p.until(func() bool { return e.status(p, head.RunID) == engine.RunWaiting }, "a-000 holds its conversation")
	tx, err := e.db.Begin(ctx)
	require.NoError(t, err)
	defer func() { _ = tx.Rollback(ctx) }()
	for _, q := range []struct {
		sql  string
		args []any
	}{
		{`CREATE TEMP TABLE clones ON COMMIT DROP AS SELECT n, gen_random_uuid() AS inst, gen_random_uuid() AS head, gen_random_uuid() AS queued,
			'a-' || lpad(n::text, 4, '0') AS k FROM generate_series(1, 2000) n`, nil},
		{`INSERT INTO extruntime.workflow_instances SELECT (jsonb_populate_record(i, jsonb_build_object('id', c.inst, 'start_key', i.start_key || ':' || c.n,
			'status', 'blocked', 'blocked_reason', 'ambiguous', 'lease_owner', NULL, 'lease_until', NULL))).*
			FROM extruntime.workflow_instances i JOIN automation.runs r ON r.instance_id = i.id, clones c WHERE r.id = $1`, []any{head.RunID}},
		{`INSERT INTO automation.runs SELECT (jsonb_populate_record(r, jsonb_build_object('id', c.head, 'instance_id', c.inst, 'subject_key', c.k,
			'start_key', 'clone:' || c.n || ':head'))).* FROM automation.runs r, clones c WHERE r.id = $1`, []any{head.RunID}},
		{`INSERT INTO automation.runs SELECT (jsonb_populate_record(r, jsonb_build_object('id', c.queued, 'subject_key', c.k,
			'start_key', 'clone:' || c.n || ':queued'))).* FROM automation.runs r, clones c WHERE r.id = $1`, []any{queuedBusy.RunID}},
		{`INSERT INTO automation.subject_queue (tenant_id, subject_entity, subject_key, run_id) SELECT $1, 'conversation', k, queued FROM clones ORDER BY n`, []any{e.tenant}},
	} {
		_, err := tx.Exec(ctx, q.sql, q.args...)
		require.NoError(t, err, q.sql)
	}
	require.NoError(t, tx.Commit(ctx))
	freeHead := start("z-free")
	queued := start("z-free")
	require.Equal(t, engine.RunQueued, queued.Status)
	// The head ends without its observer promoting the queued run (a lost promotion).
	_, err = e.db.Exec(ctx, `UPDATE extruntime.workflow_instances i SET status = 'completed', finished_at = now(), lease_owner = NULL, lease_until = NULL
		FROM automation.runs r WHERE r.instance_id = i.id AND r.id = $1`, freeHead.RunID)
	require.NoError(t, err)
	// The fixture is what a scan of 200 subjects without the busy filter misses: ten batches of busy
	// subjects queued before the free one.
	assert.Equal(t, 2001, e.count(`SELECT count(DISTINCT subject_key) FROM automation.subject_queue
		WHERE seq < (SELECT min(seq) FROM automation.subject_queue WHERE subject_key = 'z-free')`), "busy subjects queued ahead of the free one")
	require.NoError(t, p.rt.Maintain(ctx))
	assert.Equal(t, 1, e.count(`SELECT count(*) FROM automation.runs WHERE id = $1 AND instance_id IS NOT NULL`, queued.RunID),
		"the free conversation's run starts although busy ones fill the batch")
	assert.Equal(t, 2001, e.count(`SELECT count(*) FROM automation.subject_queue`), "busy conversations keep their queued runs")
}

func TestQueuedCancelAndDisableDoNotDeadlock(t *testing.T) {
	e := newEnv(t)
	p := e.proc()
	ctx := context.Background()
	human := e.human()
	am := e.create(p, human, `{"name": "Chat", "triggers": [{"id": "go", "event": "automation.manual.v1"}],
	 "nodes": [{"id": "nap", "type": "delay", "params": {"duration": "1h"}}, {"id": "fin", "type": "end", "params": {}}],
	 "edges": [{"from": {"node": "go", "outcome": "fired"}, "to": "nap"}, {"from": {"node": "nap", "outcome": "done"}, "to": "fin"}]}`)
	chat := engine.SubjectRef{Entity: graph.SubjectConversation, Key: "c-1"}
	_, err := p.rt.Service().StartManual(ctx, human, am.ID, engine.ManualStart{TriggerID: "go", Input: json.RawMessage(`{}`), Subject: &chat})
	require.NoError(t, err)
	queued, err := p.rt.Service().StartManual(ctx, human, am.ID, engine.ManualStart{TriggerID: "go", Input: json.RawMessage(`{}`), Subject: &chat})
	require.NoError(t, err)
	require.Equal(t, engine.RunQueued, queued.Status)
	// The queued run's row is held while Cancel and then Disable reach it: each takes what it locks
	// first, then both wait for the row.
	holder, err := e.db.Begin(ctx)
	require.NoError(t, err)
	t.Cleanup(func() { _ = holder.Rollback(ctx) })
	_, err = holder.Exec(ctx, `SELECT 1 FROM automation.runs WHERE id = $1 FOR UPDATE`, queued.RunID)
	require.NoError(t, err)
	holderPID := holder.Conn().PgConn().PID()
	cancelled := make(chan error, 1)
	go func() { cancelled <- p.rt.Service().Cancel(ctx, human, queued.RunID) }()
	e.waitLockWait(t, "%UPDATE automation.runs%", holderPID, "Cancel waits for the held run row")
	disabled := make(chan error, 1)
	go func() {
		_, err := p.rt.Service().Disable(ctx, human, am.ID)
		disabled <- err
	}()
	require.Eventually(t, func() bool { return e.lockWaits("%", 0) >= 2 }, 10*time.Second, 10*time.Millisecond,
		"Disable waits behind Cancel or the holder")
	require.NoError(t, holder.Commit(ctx))
	if err := <-cancelled; err != nil {
		require.ErrorIs(t, err, engine.ErrRunFinished, "no deadlock abort")
	}
	require.NoError(t, <-disabled, "no deadlock abort")
	assert.Equal(t, engine.RunCancelled, e.status(p, queued.RunID))
}

func TestCancelOfAQueuedRunTakesTheSubjectLockFirst(t *testing.T) {
	e := newEnv(t)
	p := e.proc()
	ctx := context.Background()
	human := e.human()
	am := e.create(p, human, `{"name": "Chat", "triggers": [{"id": "go", "event": "automation.manual.v1"}],
	 "nodes": [{"id": "nap", "type": "delay", "params": {"duration": "1h"}}, {"id": "fin", "type": "end", "params": {}}],
	 "edges": [{"from": {"node": "go", "outcome": "fired"}, "to": "nap"}, {"from": {"node": "nap", "outcome": "done"}, "to": "fin"}]}`)
	chat := engine.SubjectRef{Entity: graph.SubjectConversation, Key: "c-1"}
	_, err := p.rt.Service().StartManual(ctx, human, am.ID, engine.ManualStart{TriggerID: "go", Input: json.RawMessage(`{}`), Subject: &chat})
	require.NoError(t, err)
	queued, err := p.rt.Service().StartManual(ctx, human, am.ID, engine.ManualStart{TriggerID: "go", Input: json.RawMessage(`{}`), Subject: &chat})
	require.NoError(t, err)
	require.Equal(t, engine.RunQueued, queued.Status)
	// A promotion holds the conversation's lock (dequeue): the cancellation waits for it before it
	// changes the run or its queue row.
	holder, err := e.db.Begin(ctx)
	require.NoError(t, err)
	t.Cleanup(func() { _ = holder.Rollback(ctx) })
	_, err = holder.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, engine.SubjectLockKey(e.tenant, chat, uuid.Nil))
	require.NoError(t, err)
	done := make(chan error, 1)
	go func() { done <- p.rt.Service().Cancel(ctx, human, queued.RunID) }()
	e.waitLockWait(t, "%pg_advisory_xact_lock%", holder.Conn().PgConn().PID(), "the cancellation waits for the conversation's lock")
	select {
	case err := <-done:
		t.Fatalf("the cancellation did not wait for the conversation's lock: %v", err)
	default:
	}
	assert.Equal(t, 1, e.count(`SELECT count(*) FROM automation.subject_queue WHERE run_id = $1`, queued.RunID), "nothing changed while the lock is held")
	require.NoError(t, holder.Commit(ctx))
	require.NoError(t, <-done)
	assert.Equal(t, engine.RunCancelled, e.status(p, queued.RunID))
}

func TestPruneDoesNotCascadeToStepsItKeeps(t *testing.T) {
	e := newEnv(t)
	p := e.proc(func(c *engine.Config) { c.Retention = engine.Retention{Steps: time.Hour, Runs: time.Hour} })
	ctx := context.Background()
	a := e.human()
	e.create(p, a, noteGraph("x"))
	e.publish(e.tenant, orderCreated, "o-1", order{}, 0)
	run := onlyRun(t, e, p, a)
	p.until(func() bool { return e.status(p, run.RunID) == engine.RunDone }, "the run ends")
	steps := e.count(`SELECT count(*) FROM automation.steps WHERE run_id = $1`, run.RunID)
	require.Positive(t, steps)
	// The run turns old enough between the step and the run deletes.
	p.rt.SetTestHook(func(ctx context.Context, _ pgx.Tx, label string) error {
		if label != "prune-runs" {
			return nil
		}
		_, err := e.db.Exec(ctx, `UPDATE extruntime.workflow_instances i SET finished_at = now() - interval '2 hours'
			FROM automation.runs r WHERE r.instance_id = i.id AND r.id = $1`, run.RunID)
		return err
	})
	_, err := p.rt.Prune(ctx)
	require.NoError(t, err)
	assert.Equal(t, steps, e.count(`SELECT count(*) FROM automation.steps WHERE run_id = $1`, run.RunID), "the run's steps were not pruned, so neither is the run")
	assert.Equal(t, 1, e.count(`SELECT count(*) FROM automation.runs WHERE id = $1`, run.RunID))
	p.rt.SetTestHook(nil)
	_, err = p.rt.Prune(ctx)
	require.NoError(t, err)
	assert.Zero(t, e.count(`SELECT count(*) FROM automation.runs WHERE id = $1`, run.RunID), "the next pass prunes the steps, then the run")
}

func TestRestartThatCannotRunKeepsThePredecessorsConversation(t *testing.T) {
	e := newEnv(t)
	p := e.proc()
	ctx := context.Background()
	human := e.human()
	am := e.create(p, human, `{"name": "Chat", "triggers": [{"id": "go", "event": "automation.manual.v1"}],
	 "nodes": [{"id": "nap", "type": "delay", "params": {"duration": "1h"}}, {"id": "fin", "type": "end", "params": {}}],
	 "edges": [{"from": {"node": "go", "outcome": "fired"}, "to": "nap"}, {"from": {"node": "nap", "outcome": "done"}, "to": "fin"}]}`)
	chat := &engine.SubjectRef{Entity: graph.SubjectConversation, Key: "c-1"}
	first, err := p.rt.Service().StartManual(ctx, human, am.ID, engine.ManualStart{TriggerID: "go", Input: json.RawMessage(`{}`), Subject: chat})
	require.NoError(t, err)
	p.until(func() bool { return e.status(p, first.RunID) == engine.RunWaiting }, "the first run waits")
	_, err = e.db.Exec(ctx, `UPDATE extruntime.workflow_instances i SET status = 'blocked', blocked_reason = 'ambiguous', lease_owner = NULL, lease_until = NULL
		FROM automation.runs r WHERE r.instance_id = i.id AND r.id = $1`, first.RunID)
	require.NoError(t, err)
	// The owner is gone: the restart records a blocked run without an instance.
	_, err = e.db.Exec(ctx, `DELETE FROM extruntime.trigger_owners WHERE trigger_id = $1`, "automation:"+am.ID.String())
	require.NoError(t, err)
	ref, err := p.rt.Service().Restart(ctx, human, first.RunID, 1)
	require.NoError(t, err)
	require.Equal(t, engine.RunBlocked, ref.Status)
	assert.Zero(t, e.count(`SELECT count(*) FROM automation.runs WHERE id = $1 AND superseded`, first.RunID),
		"a replacement that cannot run does not take the conversation from its predecessor")
}

// tracer runs hook before the statements a pool's connection sends.
type tracer struct {
	hook func(ctx context.Context, conn *pgx.Conn, sql string) context.Context
}

func (t *tracer) TraceQueryStart(ctx context.Context, conn *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
	return t.hook(ctx, conn, data.SQL)
}

func (t *tracer) TraceQueryEnd(context.Context, *pgx.Conn, pgx.TraceQueryEndData) {}

// idle reports a statement outside a transaction.
func idle(conn *pgx.Conn) bool { return conn.PgConn().TxStatus() == 'I' }

func TestSaveVersionAnswersWithWhatItCommitted(t *testing.T) {
	e := newEnv(t)
	p := e.proc()
	ctx := context.Background()
	human := e.human()
	am := e.create(p, human, noteGraph("x"))
	var armed atomic.Bool
	tr := &tracer{hook: func(ctx context.Context, conn *pgx.Conn, sql string) context.Context {
		if armed.Load() && idle(conn) && strings.Contains(sql, "FROM automation.versions") {
			// The read after the commit fails (the connection drops, say).
			c, cancel := context.WithCancel(ctx)
			cancel()
			return c
		}
		return ctx
	}}
	rt := runtimeOn(t, e, p, onePool(t, e, tr), e.reg)
	armed.Store(true)
	v, err := rt.Service().SaveVersion(ctx, human, am.ID, engine.SaveInput{BaseVersion: 1, Graph: json.RawMessage(noteGraph("y"))})
	armed.Store(false)
	require.NoError(t, err, "the saved version is read inside its transaction")
	assert.Equal(t, 2, v.Version)
}

func TestEnableAndDisableAnswerWithTheStateTheyCommitted(t *testing.T) {
	e := newEnv(t)
	p := e.proc()
	ctx := context.Background()
	human := e.human()
	am := e.create(p, human, noteGraph("x"))
	// after is the statement of the call's transaction; then is what another caller commits as soon as
	// the call reads the graph outside it.
	var after string
	var then func()
	var seen, fired atomic.Bool
	tr := &tracer{hook: func(ctx context.Context, conn *pgx.Conn, sql string) context.Context {
		switch {
		case after != "" && strings.Contains(sql, after):
			seen.Store(true)
		case seen.Load() && idle(conn) && strings.Contains(sql, "FROM automation.automations") && !fired.Swap(true):
			then()
		}
		return ctx
	}}
	rt := runtimeOn(t, e, p, onePool(t, e, tr), e.reg)

	after = "UPDATE automation.automations SET enabled_version = NULL"
	then = func() {
		_, err := p.rt.Service().Enable(ctx, human, am.ID, engine.EnableInput{Version: 1})
		assert.NoError(t, err)
	}
	got, err := rt.Service().Disable(ctx, human, am.ID)
	require.NoError(t, err)
	assert.Nil(t, got.EnabledVersion, "Disable answers with the graph it disabled")

	_, err = p.rt.Service().Disable(ctx, human, am.ID)
	require.NoError(t, err)
	seen.Store(false)
	fired.Store(false)
	after = "INSERT INTO automation.triggers"
	then = func() {
		_, err := p.rt.Service().Disable(ctx, human, am.ID)
		assert.NoError(t, err)
	}
	got, err = rt.Service().Enable(ctx, human, am.ID, engine.EnableInput{Version: 1})
	require.NoError(t, err)
	require.NotNil(t, got.EnabledVersion, "Enable answers with the graph it enabled")
	assert.Equal(t, 1, *got.EnabledVersion)
}

func TestScheduleActivationIsTakenAfterTheGraphLock(t *testing.T) {
	e := newEnv(t)
	p := e.proc()
	ctx := context.Background()
	human := e.human()
	am := e.create(p, human, `{"name": "Nightly", "triggers": [{"id": "night", "event": "automation.schedule.v1", "params": {"cron": "0 3 * * *"}}],
	 "nodes": [{"id": "fin", "type": "end", "params": {}}], "edges": [{"from": {"node": "night", "outcome": "fired"}, "to": "fin"}]}`)
	_, err := p.rt.Service().Disable(ctx, human, am.ID)
	require.NoError(t, err)
	// Another transaction holds the graph's row while the enable begins.
	holder, err := e.db.Begin(ctx)
	require.NoError(t, err)
	t.Cleanup(func() { _ = holder.Rollback(ctx) })
	_, err = holder.Exec(ctx, `SELECT 1 FROM automation.automations WHERE id = $1 FOR UPDATE`, am.ID)
	require.NoError(t, err)
	done := make(chan error, 1)
	go func() {
		_, err := p.rt.Service().Enable(ctx, human, am.ID, engine.EnableInput{Version: 1})
		done <- err
	}()
	e.waitLockWait(t, "%automation.automations%FOR UPDATE%", holder.Conn().PgConn().PID(), "the enable waits for the held graph row")
	time.Sleep(500 * time.Millisecond) // the transaction-start clock lags behind the lock by this much
	var released time.Time
	require.NoError(t, holder.QueryRow(ctx, `SELECT clock_timestamp()`).Scan(&released))
	require.NoError(t, holder.Commit(ctx))
	require.NoError(t, <-done)
	var since time.Time
	require.NoError(t, e.db.QueryRow(ctx, `SELECT created_at FROM automation.triggers WHERE automation_id = $1`, am.ID).Scan(&since))
	assert.False(t, since.Before(released), "the schedule is active from when the enable held the graph: %s, lock released %s", since, released)
}

// storeAvailability is the availability of a connector store set after the runtime it serves.
type storeAvailability struct{ s connectors.Store }

func (a *storeAvailability) Connected(ctx context.Context, tenant uuid.UUID) (map[catalog.ConnectorKind][]catalog.ConnectionRef, error) {
	return a.s.Connected(ctx, tenant)
}

// botNode is an effect that needs a telegram bot connection.
type botNode struct{ specNode }

func (botNode) Requires() []catalog.ConnectorKind {
	return []catalog.ConnectorKind{catalog.KindTelegramBot}
}
func (botNode) ApplyEffect(context.Context, pgx.Tx, engine.StepContext, graph.Node, graph.Effect) (map[string]any, error) {
	return map[string]any{}, nil
}

type sqlKey struct{}

// lockTracer calls after once, when a statement containing match has run.
type lockTracer struct {
	match string
	after func()
	once  atomic.Bool
	pid   atomic.Uint32 // the backend that ran the statement
}

func (l *lockTracer) TraceQueryStart(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
	return context.WithValue(ctx, sqlKey{}, data.SQL)
}

func (l *lockTracer) TraceQueryEnd(ctx context.Context, conn *pgx.Conn, _ pgx.TraceQueryEndData) {
	if sql, _ := ctx.Value(sqlKey{}).(string); strings.Contains(sql, l.match) && !l.once.Swap(true) {
		l.pid.Store(conn.PgConn().PID())
		l.after()
	}
}

func TestConnectionDeleteAndEnableShareASmallPool(t *testing.T) {
	e := newEnv(t)
	p := e.proc()
	ctx := context.Background()
	locked, enableWaits := make(chan struct{}), make(chan struct{})
	var release sync.Once
	letGo := func() { release.Do(func() { close(enableWaits) }) }
	tr := &lockTracer{match: "FROM automation.connections WHERE tenant_id = $1 AND id = $2 FOR UPDATE", after: func() {
		close(locked)
		<-enableWaits
	}}
	cfg := e.db.Config().Copy()
	cfg.MaxConns, cfg.MinConns = 2, 0
	cfg.ConnConfig.Tracer = tr
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	t.Cleanup(letGo) // before pool.Close: a failed test does not leave the delete holding a connection

	avail := &storeAvailability{}
	reg := catalog.NewRegistry(avail)
	require.NoError(t, reg.RegisterTriggers(engine.Triggers()))
	for _, spec := range append(graph.BuiltinNodes(nil, []string{engine.EventRunFinished}), graph.FlowNodes(nil)...) {
		require.NoError(t, reg.RegisterNode(specNode{spec}))
	}
	require.NoError(t, reg.RegisterNode(botNode{specNode{graph.NodeSpec{Type: "acme.bot", Title: "Bot", Behavior: graph.BehaviorEffect,
		Params: graph.MustSchemaParams(`{"type":"object","properties":{},"additionalProperties":false}`)}}}))
	rt := runtimeOn(t, e, p, pool, reg)
	keys, err := connectors.NewKeyring("k1", map[string][]byte{"k1": []byte("0123456789abcdef0123456789abcdef")})
	require.NoError(t, err)
	store := connectors.NewStore(pool, keys, rt.UsageIndex(), nil, nil)
	avail.s = store
	conn, err := store.Save(ctx, connectors.Connection{TenantID: e.tenant, Kind: catalog.KindTelegramBot, Name: "bot"}, connectors.Credentials{"token": "t0ken"})
	require.NoError(t, err)
	human := e.human()
	am, v, err := rt.Service().Create(ctx, human, engine.CreateInput{Graph: json.RawMessage(`{"name": "Bot", "triggers": [{"id": "go", "event": "automation.manual.v1"}],
	 "nodes": [{"id": "bot", "type": "acme.bot", "params": {}}, {"id": "fin", "type": "end", "params": {}}],
	 "edges": [{"from": {"node": "go", "outcome": "fired"}, "to": "bot"}, {"from": {"node": "bot", "outcome": "done"}, "to": "fin"}]}`)})
	require.NoError(t, err)

	// Delete locks the row with one connection; Enable waits for that row with the other.
	tctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	deleted := make(chan error, 1)
	go func() { deleted <- store.Delete(tctx, e.tenant, conn.ID) }()
	select {
	case <-locked:
	case err := <-deleted:
		t.Fatalf("the delete ended before it locked the row: %v", err)
	case <-tctx.Done():
		t.Fatal("the delete never locked the row")
	}
	enabled := make(chan error, 1)
	go func() {
		_, err := rt.Service().Enable(tctx, human, am.ID, engine.EnableInput{Version: v.Version})
		enabled <- err
	}()
	e.waitLockWait(t, "%automation.connections%FOR SHARE%", tr.pid.Load(), "the enable waits for the row the delete holds")
	letGo()
	var inUse *connectors.InUseError
	require.ErrorAs(t, <-deleted, &inUse, "the delete finishes its check in its own transaction")
	require.NoError(t, <-enabled)
}

// changingCatalog runs change once, right after the view of the first ForTenant after it is armed.
type changingCatalog struct {
	catalog.Registry
	armed  atomic.Bool
	change func(ctx context.Context)
}

func (c *changingCatalog) ForTenant(ctx context.Context, tenant uuid.UUID, opts ...catalog.ViewOption) (catalog.View, error) {
	v, err := c.Registry.ForTenant(ctx, tenant, opts...)
	if err == nil && c.armed.Swap(false) {
		c.change(ctx)
	}
	return v, err
}

func TestConnectionDeleteSeesAResolutionThatChangedBeforeItsLock(t *testing.T) {
	e := newEnv(t)
	p := e.proc()
	ctx := context.Background()
	avail := &storeAvailability{}
	reg := catalog.NewRegistry(avail)
	require.NoError(t, reg.RegisterTriggers(engine.Triggers()))
	for _, spec := range append(graph.BuiltinNodes(nil, []string{engine.EventRunFinished}), graph.FlowNodes(nil)...) {
		require.NoError(t, reg.RegisterNode(specNode{spec}))
	}
	require.NoError(t, reg.RegisterNode(botNode{specNode{graph.NodeSpec{Type: "acme.bot", Title: "Bot", Behavior: graph.BehaviorEffect,
		Params: graph.MustSchemaParams(`{"type":"object","properties":{},"additionalProperties":false}`)}}}))
	cat := &changingCatalog{Registry: reg}
	rt := runtimeOn(t, e, p, e.db, cat)
	keys, err := connectors.NewKeyring("k1", map[string][]byte{"k1": []byte("0123456789abcdef0123456789abcdef")})
	require.NoError(t, err)
	store := connectors.NewStore(e.db, keys, rt.UsageIndex(), nil, nil)
	avail.s = store
	a, err := store.Save(ctx, connectors.Connection{TenantID: e.tenant, Kind: catalog.KindTelegramBot, Name: "bot_a"}, connectors.Credentials{"token": "a"})
	require.NoError(t, err)
	b, err := store.Save(ctx, connectors.Connection{TenantID: e.tenant, Kind: catalog.KindTelegramBot, Name: "bot_b"}, connectors.Credentials{"token": "b"})
	require.NoError(t, err)
	require.NoError(t, store.SetStatus(ctx, e.tenant, b.ID, connectors.StatusDisabled, ""))
	// The graph names no connection: it runs with the only working bot, A.
	e.create(&proc{e: e, wf: p.wf, rt: rt, disp: p.disp}, e.human(), `{"name": "Bot", "triggers": [{"id": "go", "event": "automation.manual.v1"}],
	 "nodes": [{"id": "bot", "type": "acme.bot", "params": {}}, {"id": "fin", "type": "end", "params": {}}],
	 "edges": [{"from": {"node": "go", "outcome": "fired"}, "to": "bot"}, {"from": {"node": "bot", "outcome": "done"}, "to": "fin"}]}`)

	// After the delete read the catalog and before it locks B, A needs reauthorization and B is back:
	// the enabled graph resolves to B now.
	cat.change = func(ctx context.Context) {
		require.NoError(t, store.SetStatus(ctx, e.tenant, a.ID, connectors.StatusNeedsReauth, "expired"))
		require.NoError(t, store.SetStatus(ctx, e.tenant, b.ID, connectors.StatusConnected, ""))
	}
	cat.armed.Store(true)
	var inUse *connectors.InUseError
	require.ErrorAs(t, store.Delete(ctx, e.tenant, b.ID), &inUse, "the usage check is judged by the connections under the lock")
	assert.Equal(t, 1, e.count(`SELECT count(*) FROM automation.connections WHERE id = $1`, b.ID))
	assert.False(t, cat.armed.Load(), "the change ran between the catalog read and the lock")
}

func TestConnectionDeleteIsRefusedWhileAnOmittedConnectionMayResolveToIt(t *testing.T) {
	e := newEnv(t)
	p := e.proc()
	ctx := context.Background()
	avail := &storeAvailability{}
	reg := catalog.NewRegistry(avail)
	require.NoError(t, reg.RegisterTriggers(engine.Triggers()))
	for _, spec := range append(graph.BuiltinNodes(nil, []string{engine.EventRunFinished}), graph.FlowNodes(nil)...) {
		require.NoError(t, reg.RegisterNode(specNode{spec}))
	}
	require.NoError(t, reg.RegisterNode(botNode{specNode{graph.NodeSpec{Type: "acme.bot", Title: "Bot", Behavior: graph.BehaviorEffect,
		Params: graph.MustSchemaParams(`{"type":"object","properties":{},"additionalProperties":false}`)}}}))
	rt := runtimeOn(t, e, p, e.db, reg)
	keys, err := connectors.NewKeyring("k1", map[string][]byte{"k1": []byte("0123456789abcdef0123456789abcdef")})
	require.NoError(t, err)
	store := connectors.NewStore(e.db, keys, rt.UsageIndex(), nil, nil)
	avail.s = store
	a, err := store.Save(ctx, connectors.Connection{TenantID: e.tenant, Kind: catalog.KindTelegramBot, Name: "bot_a"}, connectors.Credentials{"token": "a"})
	require.NoError(t, err)
	// The graph names no connection: A is the only bot when it is enabled.
	e.create(&proc{e: e, wf: p.wf, rt: rt, disp: p.disp}, e.human(), `{"name": "Bot", "triggers": [{"id": "go", "event": "automation.manual.v1"}],
	 "nodes": [{"id": "bot", "type": "acme.bot", "params": {}}, {"id": "fin", "type": "end", "params": {}}],
	 "edges": [{"from": {"node": "go", "outcome": "fired"}, "to": "bot"}, {"from": {"node": "bot", "outcome": "done"}, "to": "fin"}]}`)
	b, err := store.Save(ctx, connectors.Connection{TenantID: e.tenant, Kind: catalog.KindTelegramBot, Name: "bot_b"}, connectors.Credentials{"token": "b"})
	require.NoError(t, err)
	// With A and B the graph's resolution is ambiguous; once A needs reauthorization it is B. A's status
	// is not under B's lock, so B counts as used while it is a candidate.
	var inUse *connectors.InUseError
	require.ErrorAs(t, store.Delete(ctx, e.tenant, b.ID), &inUse)
	require.Len(t, inUse.Uses, 1)
	assert.Equal(t, []string{"bot"}, inUse.Uses[0].NodeIDs)
	require.NoError(t, store.SetStatus(ctx, e.tenant, a.ID, connectors.StatusNeedsReauth, "expired"))
	assert.Equal(t, 1, e.count(`SELECT count(*) FROM automation.connections WHERE id = $1`, b.ID), "the graph now runs with B, which is still there")
}

// hookCatalog runs change once, right after the view of its at-th ForTenant.
type hookCatalog struct {
	catalog.Registry
	calls, at atomic.Int32
	change    func(ctx context.Context)
}

func (c *hookCatalog) ForTenant(ctx context.Context, tenant uuid.UUID, opts ...catalog.ViewOption) (catalog.View, error) {
	v, err := c.Registry.ForTenant(ctx, tenant, opts...)
	if n := c.calls.Add(1); err == nil && n == c.at.Load() {
		c.change(ctx)
	}
	return v, err
}

// botSetup is a runtime whose catalog reads a connector store, with one working bot A.
func botSetup(t *testing.T, e *env, p *proc) (*proc, *hookCatalog, connectors.Store, connectors.Connection) {
	t.Helper()
	ctx := context.Background()
	avail := &storeAvailability{}
	reg := catalog.NewRegistry(avail)
	require.NoError(t, reg.RegisterTriggers(engine.Triggers()))
	for _, spec := range append(graph.BuiltinNodes(nil, []string{engine.EventRunFinished}), graph.FlowNodes(nil)...) {
		require.NoError(t, reg.RegisterNode(specNode{spec}))
	}
	require.NoError(t, reg.RegisterNode(botNode{specNode{graph.NodeSpec{Type: "acme.bot", Title: "Bot", Behavior: graph.BehaviorEffect,
		Params: graph.MustSchemaParams(`{"type":"object","properties":{},"additionalProperties":false}`)}}}))
	cat := &hookCatalog{Registry: reg}
	rt := runtimeOn(t, e, p, e.db, cat)
	keys, err := connectors.NewKeyring("k1", map[string][]byte{"k1": []byte("0123456789abcdef0123456789abcdef")})
	require.NoError(t, err)
	store := connectors.NewStore(e.db, keys, rt.UsageIndex(), nil, nil)
	avail.s = store
	a, err := store.Save(ctx, connectors.Connection{TenantID: e.tenant, Kind: catalog.KindTelegramBot, Name: "bot_a"}, connectors.Credentials{"token": "a"})
	require.NoError(t, err)
	return &proc{e: e, wf: p.wf, rt: rt, disp: p.disp}, cat, store, a
}

// botGraph names no connection: it runs with the only working bot.
func botGraph(name string) string {
	return `{"name": "` + name + `", "triggers": [{"id": "go", "event": "automation.manual.v1"}],
	 "nodes": [{"id": "bot", "type": "acme.bot", "params": {}}, {"id": "fin", "type": "end", "params": {}}],
	 "edges": [{"from": {"node": "go", "outcome": "fired"}, "to": "bot"}, {"from": {"node": "bot", "outcome": "done"}, "to": "fin"}]}`
}

func TestEnableIsRefusedWhenItsResolvedConnectionChangedAfterValidation(t *testing.T) {
	e := newEnv(t)
	bp, cat, store, a := botSetup(t, e, e.proc())
	ctx := context.Background()
	human := e.human()
	am, v1, err := bp.rt.Service().Create(ctx, human, engine.CreateInput{Graph: json.RawMessage(botGraph("Bot"))})
	require.NoError(t, err)
	// A later version uses no bot: version 1 is neither latest nor enabled, so nothing blocks deleting a bot for it.
	_, err = bp.rt.Service().SaveVersion(ctx, human, am.ID, engine.SaveInput{BaseVersion: 1, Graph: json.RawMessage(`{"name": "Plain",
	 "triggers": [{"id": "go", "event": "automation.manual.v1"}], "nodes": [{"id": "fin", "type": "end", "params": {}}],
	 "edges": [{"from": {"node": "go", "outcome": "fired"}, "to": "fin"}]}`)})
	require.NoError(t, err)
	// After the enable validated version 1 (A), and before its lock: A needs reauthorization, B comes and goes.
	cat.change = func(ctx context.Context) {
		require.NoError(t, store.SetStatus(ctx, e.tenant, a.ID, connectors.StatusNeedsReauth, "expired"))
		b, err := store.Save(ctx, connectors.Connection{TenantID: e.tenant, Kind: catalog.KindTelegramBot, Name: "bot_b"}, connectors.Credentials{"token": "b"})
		require.NoError(t, err)
		require.NoError(t, store.Delete(ctx, e.tenant, b.ID))
	}
	cat.at.Store(cat.calls.Load() + 2) // the triggers' rights, then the validation
	_, err = bp.rt.Service().Enable(ctx, human, am.ID, engine.EnableInput{Version: v1.Version})
	var verr *engine.ValidationError
	require.ErrorAs(t, err, &verr, "validated again: the graph has no working bot")
	assert.Equal(t, engine.CodeConnectionUnavailable, verr.Errors[0].Code)
	assert.Equal(t, 0, e.count(`SELECT count(*) FROM automation.automations WHERE id = $1 AND enabled_version IS NOT NULL`, am.ID))
}

func TestAutoEnableKeepsTheEnabledVersionWhenItsConnectionChangedAfterValidation(t *testing.T) {
	e := newEnv(t)
	bp, cat, store, a := botSetup(t, e, e.proc())
	ctx := context.Background()
	human := e.human()
	am := e.create(bp, human, botGraph("Bot"))
	// The save validates against A; before its transaction A needs reauthorization.
	cat.change = func(ctx context.Context) {
		require.NoError(t, store.SetStatus(ctx, e.tenant, a.ID, connectors.StatusNeedsReauth, "expired"))
	}
	cat.at.Store(cat.calls.Load() + 1)
	v2, err := bp.rt.Service().SaveVersion(ctx, human, am.ID, engine.SaveInput{BaseVersion: 1, Graph: json.RawMessage(botGraph("Bot renamed"))})
	require.NoError(t, err, "the version is saved")
	assert.Equal(t, 2, v2.Version)
	assert.Equal(t, 1, e.count(`SELECT count(*) FROM automation.automations WHERE id = $1 AND enabled_version = 1`, am.ID),
		"the running version stays: the new one is not enabled over a view that changed")
}

func TestEnableIsRefusedWhenAnotherCandidateConnectionAppearedAfterValidation(t *testing.T) {
	e := newEnv(t)
	bp, cat, store, _ := botSetup(t, e, e.proc())
	ctx := context.Background()
	human := e.human()
	am, v1, err := bp.rt.Service().Create(ctx, human, engine.CreateInput{Graph: json.RawMessage(botGraph("Bot"))})
	require.NoError(t, err)
	// A stays connected (its lock succeeds); B, a second working bot, makes the omitted connection
	// ambiguous after the validation.
	cat.change = func(ctx context.Context) {
		_, err := store.Save(ctx, connectors.Connection{TenantID: e.tenant, Kind: catalog.KindTelegramBot, Name: "bot_b"}, connectors.Credentials{"token": "b"})
		require.NoError(t, err)
	}
	cat.at.Store(cat.calls.Load() + 2)
	_, err = bp.rt.Service().Enable(ctx, human, am.ID, engine.EnableInput{Version: v1.Version})
	var verr *engine.ValidationError
	require.ErrorAs(t, err, &verr, "validated again: the omitted connection is ambiguous now")
	require.NotEmpty(t, verr.Errors)
	assert.Equal(t, "nodes[0].params.connection", verr.Errors[0].Path, "%v", verr.Errors)
	assert.Equal(t, 0, e.count(`SELECT count(*) FROM automation.automations WHERE id = $1 AND enabled_version IS NOT NULL`, am.ID))
}
