package manage_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/pkg/extruntime/manage"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/manifest"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/spi"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/workflow"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

func (e *env) outboxPosition() int64 {
	e.t.Helper()
	var n int64
	require.NoError(e.t, e.db.QueryRow(context.Background(), `SELECT CASE WHEN is_called THEN last_value ELSE 0 END FROM extruntime.outbox_events_seq_seq`).Scan(&n))
	return n
}

func (e *env) publishEvent() {
	e.t.Helper()
	e.exec(`INSERT INTO extruntime.outbox_events (id, tenant_id, event, payload) VALUES (gen_random_uuid(), $1, 'core.events.userCreated.v1', '{}')`, e.tenant)
}

func TestActivationWatermarksSubscriptionsAndSourcesFollowActiveVersions(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	m := insuranceManifest(tm(manifest.KindString))
	m.Schedules = []manifest.Schedule{{ID: ns + ".nightly", Export: "nightly", Cron: "0 3 * * *", Timezone: "Asia/Tashkent"}}
	m.Implementations = append(m.Implementations, manifest.Implementation{Export: "renewalFlow", Implements: ns + ".renewalFlow", Kind: manifest.ImplWorkflow})
	m.Resources = append(m.Resources, manifest.Resource{ID: ns + ".renewalFlow", Kind: manifest.ResWorkflow, Input: emptyObj()})
	e.draft(ns, "1.0.0", m, nil)

	e.publishEvent()
	e.publishEvent()
	before := e.outboxPosition()
	e.release(ns, "1.0.0")

	subs, err := e.svc.SubscriptionSource().ActiveSubscriptions(ctx)
	require.NoError(t, err)
	require.Len(t, subs, 1)
	require.Equal(t, ns+".onRenewed", subs[0].ID)
	require.Equal(t, "core.events.userCreated.v1", subs[0].Event)
	require.Equal(t, "1.0.0", subs[0].Version)
	require.Equal(t, before, subs[0].AfterSeq, "events older than the activation are never replayed")
	require.FileExists(t, subs[0].Artifact.Dir+"/server.mjs")

	// an update keeps the position; events published in between are delivered
	e.publishEvent()
	e.draft(ns, "2.0.0", m, nil)
	e.release(ns, "2.0.0")
	subs, _ = e.svc.SubscriptionSource().ActiveSubscriptions(ctx)
	require.Equal(t, before, subs[0].AfterSeq)
	require.Equal(t, "2.0.0", subs[0].Version)

	sched, err := e.svc.ScheduleSource().ActiveSchedules(ctx)
	require.NoError(t, err)
	require.Len(t, sched, 1)
	require.Equal(t, []string{ns + ".nightly", "0 3 * * *", "Asia/Tashkent", "2.0.0"}, []string{sched[0].ID, sched[0].Cron, sched[0].Timezone, sched[0].Version})

	impl, err := e.svc.Implementations().Active(ctx, ns+".renewalFlow")
	require.NoError(t, err)
	require.Equal(t, "2.0.0", impl.Version)
	require.Equal(t, "renewalFlow", impl.Export)
	old, err := e.svc.Implementations().At(ctx, ns+".renewalFlow", ns, "1.0.0")
	require.NoError(t, err)
	require.Equal(t, "1.0.0", old.Version, "a pinned instance keeps resolving its version")
	_, err = e.svc.Implementations().At(ctx, ns+".nope", ns, "1.0.0")
	require.Equal(t, serrors.NotFound, serrors.CodeOf(err))

	// deactivation removes them; a later activation starts after "now"
	require.NoError(t, e.svc.Deactivate(ctx, e.admin, ns))
	subs, _ = e.svc.SubscriptionSource().ActiveSubscriptions(ctx)
	require.Empty(t, subs)
	sched, _ = e.svc.ScheduleSource().ActiveSchedules(ctx)
	require.Empty(t, sched)
	_, err = e.svc.Implementations().Active(ctx, ns+".renewalFlow")
	require.Error(t, err)
	e.publishEvent()
	after := e.outboxPosition()
	_, err = e.svc.Activate(ctx, e.admin, ns, "2.0.0")
	require.NoError(t, err)
	subs, _ = e.svc.SubscriptionSource().ActiveSubscriptions(ctx)
	require.Equal(t, after, subs[0].AfterSeq)
}

// fakeEngine records the calls the service forwards to the workflow engine.
type fakeEngine struct {
	calls []string
	err   error
}

func (f *fakeEngine) rec(s string) error { f.calls = append(f.calls, s); return f.err }

func (f *fakeEngine) List(_ context.Context, fl workflow.ListFilter) ([]workflow.Instance, error) {
	return []workflow.Instance{{ID: uuid.New(), TenantID: fl.TenantID, Workflow: "w"}}, f.rec("list:" + fl.TenantID.String())
}
func (f *fakeEngine) Get(_ context.Context, tenant, id uuid.UUID) (workflow.History, error) {
	return workflow.History{}, f.rec("get:" + tenant.String())
}
func (f *fakeEngine) Signal(_ context.Context, tenant uuid.UUID, sender int64, _ uuid.UUID, name string, _ json.RawMessage, _ string) error {
	return f.rec("signal:" + name)
}
func (f *fakeEngine) DecideTask(_ context.Context, a spi.Subject, _ uuid.UUID, _ json.RawMessage) error {
	return f.rec("decide")
}
func (f *fakeEngine) Terminate(context.Context, uuid.UUID, uuid.UUID) error {
	return f.rec("terminate")
}
func (f *fakeEngine) StartV2(_ context.Context, _ spi.Subject, _ uuid.UUID, v string) (uuid.UUID, error) {
	return uuid.New(), f.rec("startv2:" + v)
}
func (f *fakeEngine) Resolve(context.Context, uuid.UUID, uuid.UUID, string, int, json.RawMessage) error {
	return f.rec("resolve")
}
func (f *fakeEngine) Retry(context.Context, uuid.UUID, uuid.UUID, string, int) error {
	return f.rec("retry")
}
func (f *fakeEngine) Resume(context.Context, uuid.UUID, uuid.UUID) error { return f.rec("resume") }

func TestWorkflowManagementIsPermissionedAndAudited(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	id := uuid.New()

	_, err := e.svc.ListWorkflows(ctx, e.reader, workflow.ListFilter{})
	require.Equal(t, serrors.FailedPrecondition, serrors.CodeOf(err), "no engine attached")

	eng := &fakeEngine{}
	e.svc.SetWorkflows(eng)
	list, err := e.svc.ListWorkflows(ctx, e.reader, workflow.ListFilter{TenantID: uuid.New()})
	require.NoError(t, err)
	require.Equal(t, e.tenant, list[0].TenantID, "always the actor's tenant")
	_, err = e.svc.GetWorkflow(ctx, e.reader, id)
	require.NoError(t, err)
	_, err = e.svc.ListWorkflows(ctx, spiAnon(), workflow.ListFilter{})
	require.Equal(t, serrors.PermissionDenied, serrors.CodeOf(err))

	// operating needs the operate permission; deciding is the engine's rule
	require.Equal(t, serrors.PermissionDenied, serrors.CodeOf(e.svc.SignalWorkflow(ctx, e.author, id, "approve", nil, "")))
	require.Equal(t, serrors.PermissionDenied, serrors.CodeOf(e.svc.TerminateWorkflow(ctx, e.reader, id)))
	require.NoError(t, e.svc.SignalWorkflow(ctx, e.admin, id, "approve", json.RawMessage(`{}`), "k1"))
	require.NoError(t, e.svc.DecideTask(ctx, e.reader, id, json.RawMessage(`{"approved":true}`)))
	require.NoError(t, e.svc.TerminateWorkflow(ctx, e.admin, id))
	_, err = e.svc.StartWorkflowV2(ctx, e.admin, id, "2.0.0")
	require.NoError(t, err)
	require.NoError(t, e.svc.ResolveWorkflowOccurrence(ctx, e.admin, id, "renew", 0, json.RawMessage(`{}`)))
	require.NoError(t, e.svc.RetryWorkflowOccurrence(ctx, e.admin, id, "renew", 0))
	require.NoError(t, e.svc.ResumeWorkflow(ctx, e.admin, id))
	require.Equal(t, []string{"list:" + e.tenant.String(), "get:" + e.tenant.String(), "signal:approve", "decide", "terminate", "startv2:2.0.0", "resolve", "retry", "resume"}, eng.calls)

	ok := e.auditActions(manage.OutcomeOK)
	for _, a := range []string{manage.ActionWorkflowSignal, manage.ActionWorkflowDecide, manage.ActionWorkflowTerminate, manage.ActionWorkflowStartV2,
		manage.ActionWorkflowResolve, manage.ActionWorkflowRetry, manage.ActionWorkflowResume} {
		require.Contains(t, ok, a)
	}
	require.Contains(t, e.auditActions(manage.OutcomeDenied), manage.ActionWorkflowSignal)

	// engine refusals keep their meaning
	eng.err = workflow.ErrNotAccepting
	require.Equal(t, serrors.FailedPrecondition, serrors.CodeOf(e.svc.ResumeWorkflow(ctx, e.admin, id)))
	eng.err = workflow.ErrNotFound
	require.Equal(t, serrors.NotFound, serrors.CodeOf(e.svc.TerminateWorkflow(ctx, e.admin, id)))
	eng.err = workflow.ErrForbidden
	require.Equal(t, serrors.PermissionDenied, serrors.CodeOf(e.svc.DecideTask(ctx, e.reader, id, nil)))
	require.Contains(t, e.auditActions(manage.OutcomeFailed), manage.ActionWorkflowResume)
}
