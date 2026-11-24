package engine_test

// Regression tests of the follow-up to the engine's review (#1395).

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/pkg/automation/catalog"
	"github.com/iota-uz/iota-sdk/pkg/automation/engine"
)

// poolCatalog is a catalog whose views read the database, as a connector store with a keyring does.
type poolCatalog struct {
	catalog.Registry
	db *pgxpool.Pool
}

func (c poolCatalog) ForTenant(ctx context.Context, tenant uuid.UUID, opts ...catalog.ViewOption) (catalog.View, error) {
	if _, err := c.db.Exec(ctx, `SELECT 1`); err != nil {
		return catalog.View{}, err
	}
	return c.Registry.ForTenant(ctx, tenant, opts...)
}

// onePool is a pool of one connection to the test database.
func onePool(t *testing.T, e *env, tracer pgxTracer) *pgxpool.Pool {
	t.Helper()
	cfg := e.db.Config().Copy()
	cfg.MaxConns, cfg.MinConns = 1, 0
	if tracer != nil {
		cfg.ConnConfig.Tracer = tracer
		cfg.MaxConns = 4
	}
	pool, err := pgxpool.NewWithConfig(context.Background(), cfg)
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	return pool
}

type pgxTracer interface {
	TraceQueryStart(ctx context.Context, conn *pgx.Conn, data pgx.TraceQueryStartData) context.Context
	TraceQueryEnd(ctx context.Context, conn *pgx.Conn, data pgx.TraceQueryEndData)
}

func runtimeOn(t *testing.T, e *env, p *proc, db *pgxpool.Pool, reg catalog.Registry) *engine.Runtime {
	t.Helper()
	rt, err := engine.NewRuntime(engine.Config{DB: db, Workflows: p.wf, Users: e.subjects, Catalog: reg, Tasks: tasks{}, Logger: e.logger})
	require.NoError(t, err)
	return rt
}

func TestRunHistoryDoesNotHoldTheConnectionTheProjectorNeeds(t *testing.T) {
	e := newEnv(t)
	p := e.proc()
	owner := e.human()
	e.create(p, owner, noteGraph("x"))
	e.publish(e.tenant, orderCreated, "o-1", order{Secret: "s3cret"}, 0)
	run := onlyRun(t, e, p, owner)
	p.until(func() bool { return e.status(p, run.RunID) == engine.RunDone }, "the run ends")
	_, err := e.db.Exec(context.Background(), `UPDATE automation.runs SET error = '{"code": "x", "message": "m"}' WHERE id = $1`, run.RunID)
	require.NoError(t, err)

	one := onePool(t, e, nil)
	rt := runtimeOn(t, e, p, one, poolCatalog{Registry: e.reg, db: one})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	page, err := rt.Service().ListRuns(ctx, owner, engine.RunFilter{})
	require.NoError(t, err, "the page is projected after its rows are closed")
	require.Len(t, page.Items, 1)
	d, err := rt.Service().GetRun(ctx, owner, run.RunID)
	require.NoError(t, err, "the detail is projected after its snapshot ends")
	assert.NotEmpty(t, d.Steps)
}

func TestFilterErrorReasonIsHiddenFromAViewerWithoutTheField(t *testing.T) {
	e := newEnv(t)
	p := e.proc()
	owner := e.human()
	// An invalid pattern from a protected field: the CEL error quotes it.
	e.create(p, owner, `{"name": "Filtered", "triggers": [{"id": "created", "event": "acme.order.created.v1", "filter": "event.note.matches(event.secret)"}],
	  "nodes": [{"id": "fin", "type": "end", "params": {}}], "edges": [{"from": {"node": "created", "outcome": "fired"}, "to": "fin"}]}`)
	e.publish(e.tenant, orderCreated, "o-1", order{Note: "n", Secret: "(s3cret"}, 0)
	run := onlyRun(t, e, p, owner)
	require.Equal(t, engine.RunFilterFailed, run.Status)
	require.Contains(t, run.Reason, "s3cret", "the owner may read the field: the reason is whole")
	viewer := e.actor(e.tenant, 1, engine.OriginHuman, engine.PermRead, permRead)
	runs := e.runs(p, viewer, engine.RunFilter{})
	require.Len(t, runs, 1)
	assert.Equal(t, "filter_error: "+engine.Hidden, runs[0].Reason)
	d := e.run(p, viewer, run.RunID)
	assert.Equal(t, "filter_error: "+engine.Hidden, d.Reason)
}
