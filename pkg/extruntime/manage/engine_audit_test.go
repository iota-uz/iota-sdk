package manage_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/pkg/extruntime/manifest"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/workflow"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

// The workflow engine asks the management slice whether an instance's package
// still has an active version; schedules carry the activation time so they
// never fire slots from before it.
func TestPackageGateAndScheduleActivationFollowTheActivation(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	m := insuranceManifest(tm(manifest.KindString))
	m.Schedules = []manifest.Schedule{{ID: ns + ".nightly", Export: "nightly", Cron: "0 3 * * *"}}
	e.draft(ns, "1.0.0", m, nil)

	gate, ok := e.svc.Implementations().(workflow.PackageGate)
	require.True(t, ok, "the implementations resolver is the engine's package gate")
	active, err := gate.PackageActive(ctx, ns)
	require.NoError(t, err)
	require.False(t, active, "nothing is activated yet")

	before := time.Now().Add(-time.Second)
	e.release(ns, "1.0.0")
	active, err = gate.PackageActive(ctx, ns)
	require.NoError(t, err)
	require.True(t, active)
	other, err := gate.PackageActive(ctx, "acme.other")
	require.NoError(t, err)
	require.False(t, other)

	sched, err := e.svc.ScheduleSource().ActiveSchedules(ctx)
	require.NoError(t, err)
	require.Len(t, sched, 1)
	require.True(t, sched[0].ActiveFrom.After(before) && !sched[0].ActiveFrom.After(time.Now().Add(time.Second)), "ActiveFrom is the activation time: %s", sched[0].ActiveFrom)

	// A version update keeps the package active for pinned instances.
	e.draft(ns, "2.0.0", m, nil)
	e.release(ns, "2.0.0")
	active, err = gate.PackageActive(ctx, ns)
	require.NoError(t, err)
	require.True(t, active)

	require.NoError(t, e.svc.Deactivate(ctx, e.admin, ns))
	active, err = gate.PackageActive(ctx, ns)
	require.NoError(t, err)
	require.False(t, active, "a deactivated package's instances must not run")
}

func TestRequeueReceiptsIsOperatorOnlyScopedToTheTenantAndAudited(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	const sub = ns + ".onRenewed"
	otherTenant := uuid.New()
	e.exec(`INSERT INTO tenants (id, name, domain, created_at, updated_at) VALUES ($1, $2, $3, NOW(), NOW())`, otherTenant, "t-"+otherTenant.String()[:8], otherTenant.String()[:8]+".test")
	insert := func(tenant uuid.UUID, status string) string {
		id := e.scalar(`INSERT INTO extruntime.outbox_events (id, tenant_id, event, payload) VALUES (gen_random_uuid(), $1, 'core.events.userCreated.v1', '{}') RETURNING id::text`, tenant)
		e.exec(`INSERT INTO extruntime.subscription_receipts (subscription, event_id, tenant_id, status, attempts, next_at, last_error)
			VALUES ($1, $2::uuid, $3, $4, 5, now() + interval '1 hour', 'poison')`, sub, id, tenant, status)
		return id
	}
	dead := insert(e.tenant, "dead")
	blocked := insert(e.tenant, "blocked")
	done := insert(e.tenant, "done")
	foreign := insert(otherTenant, "dead")

	_, err := e.svc.RequeueReceipts(ctx, e.author, sub, "")
	require.Equal(t, serrors.PermissionDenied, serrors.CodeOf(err), "requeue needs the operate permission")
	require.Equal(t, 1, e.count(`SELECT count(*) FROM extruntime.audit_log WHERE action = 'events.requeue' AND outcome = 'denied'`))

	_, err = e.svc.RequeueReceipts(ctx, e.admin, sub, "not-a-uuid")
	require.Equal(t, serrors.Invalid, serrors.CodeOf(err))

	n, err := e.svc.RequeueReceipts(ctx, e.admin, sub, dead)
	require.NoError(t, err)
	require.EqualValues(t, 1, n)
	n, err = e.svc.RequeueReceipts(ctx, e.admin, sub, "")
	require.NoError(t, err)
	require.EqualValues(t, 1, n, "the remaining blocked receipt; delivered and foreign-tenant ones are untouched")

	status := func(id string) string {
		return e.scalar(`SELECT status || ':' || attempts || ':' || (next_at <= now())::text FROM extruntime.subscription_receipts WHERE event_id = $1::uuid`, id)
	}
	require.Equal(t, "failed:0:true", status(dead))
	require.Equal(t, "failed:0:true", status(blocked))
	require.Equal(t, "done:5:false", status(done))
	require.Equal(t, "dead:5:false", status(foreign))
	require.Equal(t, 2, e.count(`SELECT count(*) FROM extruntime.audit_log WHERE action = 'events.requeue' AND outcome = 'ok' AND target = $1 AND actor_user_id = 1`, sub))
}
