package manage_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/pkg/extruntime/manage"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/manifest"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/spi"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

// workflowManifest is the reference package plus a workflow implementation.
func workflowManifest() manifest.Manifest {
	m := insuranceManifest(tm(manifest.KindString))
	m.Resources = append(m.Resources, manifest.Resource{ID: ns + ".renewal", Kind: manifest.ResWorkflow, Input: emptyObj(), Output: emptyObj()})
	m.Implementations = append(m.Implementations, manifest.Implementation{Export: "renewal", Implements: ns + ".renewal", Kind: manifest.ImplWorkflow})
	return m
}

func TestStartV2ResolvesARejectedVersion(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	m := workflowManifest()
	e.draft(ns, "1.0.0", m, nil)
	e.release(ns, "1.0.0")

	e.draft(ns, "2.0.0", m, map[string]string{"server.mjs": "export const renewal = async () => { /* unreviewed */ };\n"})
	_, err := e.svc.Publish(ctx, e.admin, ns, "2.0.0")
	require.NoError(t, err)

	// published but never reviewed
	_, err = e.svc.Implementations().At(ctx, ns+".renewal", ns, "2.0.0")
	require.Equal(t, serrors.FailedPrecondition, serrors.CodeOf(err), "an unreviewed version must not be runnable through start-v2")

	_, err = e.svc.Review(ctx, e.reviewer, ns, "2.0.0", false, "do not run this")
	require.NoError(t, err)
	_, err = e.svc.Activate(ctx, e.admin, ns, "2.0.0")
	require.Error(t, err, "a rejected version cannot be activated")
	_, err = e.svc.Implementations().At(ctx, ns+".renewal", ns, "2.0.0")
	require.Equal(t, serrors.FailedPrecondition, serrors.CodeOf(err), "a rejected version must not be runnable through start-v2")

	// the reviewed, active version keeps resolving
	impl, err := e.svc.Implementations().At(ctx, ns+".renewal", ns, "1.0.0")
	require.NoError(t, err)
	require.Equal(t, "1.0.0", impl.Version)
}

func TestArtifactsOfUnapprovedVersionsAreNotResolvable(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	e.draft(ns, "1.0.0", insuranceManifest(tm(manifest.KindString)), nil)
	_, err := e.svc.Publish(ctx, e.admin, ns, "1.0.0")
	require.NoError(t, err)

	_, _, err = e.svc.ResolveArtifact(ctx, ns, "1.0.0")
	require.Equal(t, serrors.FailedPrecondition, serrors.CodeOf(err), "never reviewed")
	_, _, err = e.svc.ResolveArtifact(ctx, ns, "9.9.9")
	require.Equal(t, serrors.NotFound, serrors.CodeOf(err))

	_, err = e.svc.Review(ctx, e.reviewer, ns, "1.0.0", true, "ok")
	require.NoError(t, err)
	art, _, err := e.svc.ResolveArtifact(ctx, ns, "1.0.0")
	require.NoError(t, err, "approved: runnable even before activation (e.g. pinned instances)")
	require.NotEmpty(t, art.Dir)

	// an approval bound to other hashes does not count (the newest review decides)
	e.exec(`INSERT INTO extruntime.version_reviews (version_id, source_hash, artifact_hash, reviewer_id, approve, comment)
		SELECT id, source_hash, 'sha256:other', $2, true, 'stale' FROM extruntime.versions WHERE namespace = $1 AND version = '1.0.0'`, ns, e.newUser("rev"))
	_, _, err = e.svc.ResolveArtifact(ctx, ns, "1.0.0")
	require.Equal(t, serrors.FailedPrecondition, serrors.CodeOf(err), "approval is bound to the exact hashes")
}

func TestRejectionAfterApprovalStopsExecutionOfAnActiveVersion(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	m := workflowManifest()
	m.Schedules = []manifest.Schedule{{ID: ns + ".nightly", Export: "nightly", Cron: "0 3 * * *", Timezone: "Asia/Tashkent"}}
	e.draft(ns, "1.0.0", m, nil)
	e.release(ns, "1.0.0")
	subs, err := e.svc.SubscriptionSource().ActiveSubscriptions(ctx)
	require.NoError(t, err)
	require.Len(t, subs, 1)

	_, err = e.svc.Review(ctx, e.reviewer, ns, "1.0.0", false, "found a backdoor")
	require.NoError(t, err)

	_, _, err = e.svc.ResolveArtifact(ctx, ns, "1.0.0")
	require.Equal(t, serrors.FailedPrecondition, serrors.CodeOf(err))
	// the sources of the other active packages are not taken down by it
	subs, err = e.svc.SubscriptionSource().ActiveSubscriptions(ctx)
	require.NoError(t, err)
	require.Empty(t, subs)
	sched, err := e.svc.ScheduleSource().ActiveSchedules(ctx)
	require.NoError(t, err)
	require.Empty(t, sched)
	_, err = e.svc.Implementations().Active(ctx, ns+".renewal")
	require.Error(t, err)
}

// subjectsOf is a host.SubjectLoader over a fixed table of rights.
type subjectsOf map[int64][]string

func (s subjectsOf) LoadSubject(_ context.Context, tenant uuid.UUID, id int64, kind spi.SubjectKind) (spi.Subject, error) {
	return spi.Subject{TenantID: tenant, UserID: id, Kind: kind, Permissions: s[id]}, nil
}

func withSubjects(s subjectsOf) func(*manage.Config) {
	return func(c *manage.Config) { c.Subjects = s }
}

func (e *env) subject(id int64, perms ...string) spi.Subject {
	return spi.Subject{TenantID: e.tenant, UserID: id, Kind: spi.SubjectInitiator, Permissions: perms, Locale: "en"}
}

func TestTriggerOwnerMustNotOutrankTheAssigner(t *testing.T) {
	ctx := context.Background()
	rights := subjectsOf{}
	e := newEnv(t, withSubjects(rights))
	weak, strong, publisherID, fullID := e.newUser("weak"), e.newUser("strong"), e.newUser("publisher"), e.newUser("full")
	rights[weak] = []string{manage.PermRead}
	rights[strong] = []string{manage.PermRead, manage.PermPublish, "Users.Delete"}
	rights[publisherID] = []string{manage.PermRead, manage.PermPublish}
	rights[fullID] = []string{manage.PermRead, manage.PermPublish, "Users.Delete"}
	e.draft(ns, "1.0.0", insuranceManifest(tm(manifest.KindString)), nil)
	e.release(ns, "1.0.0")
	trigger := ns + ".onRenewed"

	// an extensions.publish holder (no user administration) cannot hand a
	// trigger to a user with rights it lacks
	publisher := e.subject(publisherID, manage.PermRead, manage.PermPublish)
	_, err := e.svc.AssignOwner(ctx, publisher, trigger, strong)
	require.Equal(t, serrors.PermissionDenied, serrors.CodeOf(err))
	_, ok, err := e.svc.OwnerOf(ctx, e.tenant, trigger)
	require.NoError(t, err)
	require.False(t, ok, "nothing was assigned")
	require.Equal(t, 1, e.count(`SELECT count(*) FROM extruntime.audit_log WHERE action = $1 AND outcome = 'failed' AND target = $2`, manage.ActionAssignOwner, trigger),
		"the refused assignment is audited")

	// a user whose rights the assigner holds in full is fine
	o, err := e.svc.AssignOwner(ctx, publisher, trigger, weak)
	require.NoError(t, err)
	require.Equal(t, weak, o.UserID)
	// the assigner itself is always fine
	_, err = e.svc.AssignOwner(ctx, publisher, trigger, publisherID)
	require.NoError(t, err)

	// a holder of every permission of the target may assign it
	full := e.subject(fullID, manage.PermRead, manage.PermPublish, "Users.Delete")
	_, err = e.svc.AssignOwner(ctx, full, trigger, strong)
	require.NoError(t, err)
}

func TestTriggerOwnerOtherThanTheCallerNeedsARightsSource(t *testing.T) {
	e := newEnv(t) // no SubjectLoader configured
	ctx := context.Background()
	e.draft(ns, "1.0.0", insuranceManifest(tm(manifest.KindString)), nil)
	e.release(ns, "1.0.0")
	adminID, other := e.newUser("admin"), e.newUser("other")
	admin := e.subject(adminID, e.admin.Permissions...)
	_, err := e.svc.AssignOwner(ctx, admin, ns+".onRenewed", other)
	require.Equal(t, serrors.PermissionDenied, serrors.CodeOf(err), "without a way to compare rights only self-assignment is allowed")
	_, err = e.svc.AssignOwner(ctx, admin, ns+".onRenewed", adminID)
	require.NoError(t, err)
}
