package workflow_test

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/pkg/extruntime/events"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/host"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/host/hosttest"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/manifest"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/pool"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/spi"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/workflow"
)

const (
	flowID  = "acme.test.flow"
	prelude = `const item = { id: "acme.test.item" };
const go = { workflow: "acme.test.flow", name: "go" };
`
)

// impls is the Implementations double: which package version runs a workflow.
type impls struct {
	mu     sync.Mutex
	active map[string]workflow.Implementation
	at     map[string]workflow.Implementation
}

func newImpls() *impls {
	return &impls{active: map[string]workflow.Implementation{}, at: map[string]workflow.Implementation{}}
}

// Publish registers pkg@version's export as the implementation of the
// workflow; Activate makes a version the one new instances pin.
func (i *impls) Publish(workflowID, pkg, version, export string, art pool.Artifact) {
	i.mu.Lock()
	i.at[workflowID+"|"+pkg+"|"+version] = workflow.Implementation{Package: pkg, Version: version, Export: export, Artifact: art}
	i.mu.Unlock()
}

func (i *impls) Activate(workflowID, pkg, version string) {
	i.mu.Lock()
	i.active[workflowID] = i.at[workflowID+"|"+pkg+"|"+version]
	i.mu.Unlock()
}

func (i *impls) Active(_ context.Context, workflowID string) (workflow.Implementation, error) {
	i.mu.Lock()
	defer i.mu.Unlock()
	impl, ok := i.active[workflowID]
	if !ok {
		return workflow.Implementation{}, fmt.Errorf("no active implementation of %s", workflowID)
	}
	return impl, nil
}

func (i *impls) At(_ context.Context, workflowID, pkg, version string) (workflow.Implementation, error) {
	i.mu.Lock()
	defer i.mu.Unlock()
	impl, ok := i.at[workflowID+"|"+pkg+"|"+version]
	if !ok {
		return workflow.Implementation{}, fmt.Errorf("no implementation of %s in %s@%s", workflowID, pkg, version)
	}
	return impl, nil
}

// counting records the wf.* host calls that reach Go.
type counting struct {
	inner host.WorkflowHandler
	mu    sync.Mutex
	calls []string
}

func (c *counting) CallWorkflow(ctx context.Context, call host.WorkflowCall, method string, args json.RawMessage) (json.RawMessage, error) {
	c.mu.Lock()
	c.calls = append(c.calls, method)
	c.mu.Unlock()
	return c.inner.CallWorkflow(ctx, call, method, args)
}

func (c *counting) count(method string) int {
	c.mu.Lock()
	defer c.mu.Unlock()
	n := 0
	for _, m := range c.calls {
		if m == method {
			n++
		}
	}
	return n
}

func (c *counting) reset() {
	c.mu.Lock()
	c.calls = nil
	c.mu.Unlock()
}

// stack is one "process": a Node pool, a host and an engine. Two stacks on
// the same Env model an engine restart on the same database.
type stack struct {
	t      *testing.T
	env    *hosttest.Env
	pool   *pool.Pool
	host   *host.Host
	engine *workflow.Engine
	calls  *counting
}

type stackOption func(*workflow.Config, *host.Config)

func newStack(t *testing.T, env *hosttest.Env, im *impls, opts ...stackOption) *stack {
	t.Helper()
	p := env.NewPool(1)
	wcfg := workflow.Config{
		DB: env.DB, Catalog: env.Catalog, Bindings: env.Bindings, Artifacts: env.Artifacts, Impl: im, Subjects: env.Subjects,
		LeaseTTL: 5 * time.Second, Heartbeat: time.Second, PollInterval: 50 * time.Millisecond,
		RetryBackoff: 20 * time.Millisecond, RunTimeout: 30 * time.Second, Logger: env.Logger,
		TaskAdminPermission: "tasks.admin",
	}
	hcfg := host.Config{
		Pool: p, DB: env.DB, Bindings: env.Bindings, Catalog: env.Catalog, Data: env.Store,
		Artifacts: env.Artifacts, Outbox: events.NewOutbox(env.Catalog), Logger: env.Logger,
	}
	for _, o := range opts {
		o(&wcfg, &hcfg)
	}
	eng, err := workflow.New(wcfg)
	require.NoError(t, err)
	c := &counting{inner: eng}
	hcfg.Starter, hcfg.Workflow, hcfg.Signals = eng, c, eng
	h, err := host.New(hcfg)
	require.NoError(t, err)
	eng.Attach(h)
	st := &stack{t: t, env: env, pool: p, host: h, engine: eng, calls: c}
	t.Cleanup(eng.Shutdown)
	return st
}

// publishFlow declares the workflow contract (once) and registers pkg@version.
func publishFlow(t *testing.T, env *hosttest.Env, im *impls, pkg, version, source string) {
	t.Helper()
	env.Catalog.Add(spi.ResolvedResource{Package: pkg, Version: version, Resource: manifest.Resource{
		ID: flowID, Kind: manifest.ResWorkflow,
		Input:   hosttest.Obj("who", hosttest.Str(), "url", hosttest.Optional(hosttest.Str())),
		Signals: map[string]manifest.Type{"go": hosttest.JSONType(hosttest.Obj("note", hosttest.Str()))},
	}})
	art := env.Artifacts.Add(t, pkg, version, prelude+source)
	im.Publish(flowID, pkg, version, "flow", art)
}

// start writes a start intent in its own committed transaction.
func (s *stack) start(user int64, key, input string) uuid.UUID {
	s.t.Helper()
	tx, err := s.env.DB.Begin(context.Background())
	require.NoError(s.t, err)
	id, err := s.engine.Start(context.Background(), tx, spi.Scope{Subject: s.env.Subject(user)}, spi.StartIntent{Workflow: flowID, Input: json.RawMessage(input), Key: key})
	require.NoError(s.t, err)
	require.NoError(s.t, tx.Commit(context.Background()))
	return uuid.MustParse(id)
}

// drain runs ready instances until none is ready; it returns the runs made.
func (s *stack) drain() int {
	s.t.Helper()
	n, err := s.engine.RunOnce(context.Background())
	require.NoError(s.t, err)
	return n
}

// until runs the engine until cond holds (timers and retries need time).
func (s *stack) until(timeout time.Duration, cond func() bool, msg string) {
	s.t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		s.drain()
		if cond() {
			return
		}
		time.Sleep(30 * time.Millisecond)
	}
	require.Fail(s.t, "condition not met: "+msg)
}

func (s *stack) instance(id uuid.UUID) workflow.Instance {
	s.t.Helper()
	h, err := s.engine.Get(context.Background(), s.env.Tenant, id)
	require.NoError(s.t, err)
	return h.Instance
}

func (s *stack) history(id uuid.UUID) workflow.History {
	s.t.Helper()
	h, err := s.engine.Get(context.Background(), s.env.Tenant, id)
	require.NoError(s.t, err)
	return h
}

func (s *stack) status(id uuid.UUID) string { return s.instance(id).Status }

func (s *stack) signal(id uuid.UUID, name, payload, key string) error {
	return s.engine.Signal(context.Background(), s.env.Tenant, 9, id, name, json.RawMessage(payload), key)
}

func occ(h workflow.History, name string, n int) (workflow.Occurrence, bool) {
	for _, o := range h.Occurrences {
		if o.Name == name && o.N == n {
			return o, true
		}
	}
	return workflow.Occurrence{}, false
}

var _ = pgx.ErrNoRows
