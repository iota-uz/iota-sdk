package manage_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/pkg/extruntime/build"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/manage"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/manifest"
)

// TestEveryManagementActionIsAudited walks through all actions once and
// checks the audit log: actor, hashes and outcome, including refusals.
func TestEveryManagementActionIsAudited(t *testing.T) {
	rights := subjectsOf{}
	e := newEnv(t, allowFile, withSubjects(rights))
	ctx := context.Background()
	const other = "acme.rejecter"

	_, err := e.svc.CreatePackage(ctx, e.author, ns, "")
	require.NoError(t, err)
	m := insuranceManifest(tm(manifest.KindString))
	e.draft(ns, "1.0.0", m, map[string]string{"tmp.txt": "x", "tmp2.txt": "y"})
	require.NoError(t, e.svc.MoveFile(ctx, e.author, ns, "tmp.txt", "moved.txt"))
	require.NoError(t, e.svc.DeleteFile(ctx, e.author, ns, "moved.txt"))
	_, err = e.svc.Check(ctx, e.author, ns)
	require.NoError(t, err)
	_, err = e.svc.Build(ctx, e.author, ns)
	require.NoError(t, err)
	_, err = e.svc.Test(ctx, e.author, ns)
	require.NoError(t, err)
	_, err = e.svc.ExportDraft(ctx, e.reader, ns, manage.FormatTar)
	require.NoError(t, err)
	_, err = e.svc.ImportTree(ctx, e.author, build.Tree{"package.json": []byte(pkgJSON("acme.tmp", "1.0.0"))}, manage.ImportMeta{})
	require.NoError(t, err)
	repo, sha := bareRepo(t, packageFiles(t, "1.0.0"))
	_, err = e.svc.ImportGit(ctx, e.author, repo, sha, "")
	require.NoError(t, err)

	pub, err := e.svc.Publish(ctx, e.admin, ns, "1.0.0")
	require.NoError(t, err)
	rev, err := e.svc.Review(ctx, e.admin, ns, "1.0.0", true, "ok")
	require.NoError(t, err)
	_, err = e.svc.Activate(ctx, e.admin, ns, "1.0.0")
	require.NoError(t, err)
	_, err = e.svc.ExportVersion(ctx, e.reader, ns, "1.0.0", manage.FormatZip)
	require.NoError(t, err)
	_, err = e.svc.KitTarball(ctx, e.reader, "", "core")
	require.NoError(t, err)

	om := manifest.Manifest{
		Schema: manifest.SchemaVersion, Namespace: other,
		Resources:       []manifest.Resource{{ID: other + ".dummy", Kind: manifest.ResQuery, Input: emptyObj(), Output: emptyObj()}},
		Implementations: []manifest.Implementation{{Export: "rejectIncoming", Implements: "core.commands.rejectIncoming", Kind: manifest.ImplCommand}},
	}
	e.draft(other, "1.0.0", om, nil)
	e.release(other, "1.0.0")
	_, err = e.svc.Bind(ctx, e.admin, manage.SlotCommand, "core.commands.rejectIncoming", other, "1.0.0", "rejectIncoming")
	require.NoError(t, err)
	require.NoError(t, e.svc.Unbind(ctx, e.admin, manage.SlotCommand, "core.commands.rejectIncoming"))
	owner := e.newUser("owner")
	rights[owner] = []string{manage.PermRead}
	_, err = e.svc.AssignOwner(ctx, e.admin, ns+".onRenewed", owner)
	require.NoError(t, err)
	require.NoError(t, e.svc.RemoveOwner(ctx, e.admin, ns+".onRenewed"))
	require.NoError(t, e.svc.Deactivate(ctx, e.admin, ns))
	require.NoError(t, e.svc.Purge(ctx, e.admin, ns))

	// refusals are audited too
	_, err = e.svc.Publish(ctx, e.author, ns, "9.9.9")
	require.Error(t, err)
	_, err = e.svc.Activate(ctx, e.admin, ns, "nope")
	require.Error(t, err)

	ok := e.auditActions(manage.OutcomeOK)
	for _, a := range []string{
		manage.ActionCreatePackage, manage.ActionPutFile, manage.ActionDeleteFile, manage.ActionMoveFile,
		manage.ActionImportTree, manage.ActionImportGit, manage.ActionExport, manage.ActionCheck, manage.ActionBuild,
		manage.ActionTest, manage.ActionPublish, manage.ActionReview, manage.ActionActivate, manage.ActionDeactivate,
		manage.ActionPurge, manage.ActionBind, manage.ActionUnbind, manage.ActionAssignOwner, manage.ActionRemoveOwner,
		manage.ActionKit,
	} {
		require.Contains(t, ok, a)
	}
	require.Contains(t, e.auditActions(manage.OutcomeDenied), manage.ActionPublish)
	require.Contains(t, e.auditActions(manage.OutcomeFailed), manage.ActionActivate)

	log, err := e.svc.ListAudit(ctx, e.reader, manage.AuditFilter{Namespace: ns, Action: manage.ActionPublish})
	require.NoError(t, err)
	require.Len(t, log, 2, "the denied attempt and the success")
	byOutcome := map[string]manage.AuditEntry{}
	for _, a := range log {
		byOutcome[a.Outcome] = a
	}
	require.Equal(t, int64(1), byOutcome["ok"].ActorUserID)
	require.Equal(t, "1.0.0", byOutcome["ok"].Version)
	require.Equal(t, pub.SourceHash, byOutcome["ok"].SourceHash)
	require.Equal(t, pub.ArtifactHash, byOutcome["ok"].ArtifactHash)
	require.Equal(t, int64(2), byOutcome["denied"].ActorUserID)

	log, err = e.svc.ListAudit(ctx, e.reader, manage.AuditFilter{Namespace: ns, Action: manage.ActionReview})
	require.NoError(t, err)
	require.Len(t, log, 1)
	require.Equal(t, rev.SourceHash, log[0].SourceHash)
	require.Equal(t, rev.ArtifactHash, log[0].ArtifactHash)
	require.Equal(t, true, log[0].Detail["approve"])

	page, err := e.svc.ListAudit(ctx, e.reader, manage.AuditFilter{Limit: 5})
	require.NoError(t, err)
	require.Len(t, page, 5)
	next, err := e.svc.ListAudit(ctx, e.reader, manage.AuditFilter{Limit: 5, BeforeID: page[4].ID})
	require.NoError(t, err)
	require.Less(t, next[0].ID, page[4].ID)
	_, err = e.svc.ListAudit(ctx, spiAnon(), manage.AuditFilter{})
	require.Error(t, err)
}
