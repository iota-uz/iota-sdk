package manage_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/pkg/extruntime/build"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/data"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/manage"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/manifest"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

func TestDraftCheckBuildPublishReviewActivate(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()

	// only authors create packages
	_, err := e.svc.CreatePackage(ctx, e.reader, ns, "")
	require.Error(t, err)
	require.Equal(t, serrors.PermissionDenied, serrors.CodeOf(err))

	e.draft(ns, "1.0.0", insuranceManifest(tm(manifest.KindString)), nil)

	// file-by-file edits with optimistic concurrency
	files, err := e.svc.ListFiles(ctx, e.reader, ns)
	require.NoError(t, err)
	require.Len(t, files, 6) // scaffold: package.json, src/contracts.ts, src/server.ts + manifest, server, contracts
	stale := "not-the-hash"
	_, err = e.svc.PutFile(ctx, e.author, ns, "server.mjs", []byte("x"), &stale)
	require.Equal(t, serrors.Conflict, serrors.CodeOf(err))
	_, err = e.svc.PutFile(ctx, e.author, ns, "../escape.js", []byte("x"), nil)
	require.Equal(t, serrors.Invalid, serrors.CodeOf(err))
	require.NoError(t, e.svc.MoveFile(ctx, e.author, ns, "src/server.ts", "src/old-server.ts"))
	require.NoError(t, e.svc.DeleteFile(ctx, e.author, ns, "src/old-server.ts"))
	require.NoError(t, e.svc.DeleteFile(ctx, e.author, ns, "src/contracts.ts"))

	// check/build/test run against the current kit snapshot
	chk, err := e.svc.Check(ctx, e.author, ns)
	require.NoError(t, err)
	require.False(t, chk.HasErrors)
	require.NotEmpty(t, chk.KitHash)
	require.Equal(t, chk.KitHash, e.builder.LastKit.Hash)
	require.Contains(t, e.builder.LastKit.Modules, "@iota-kit/core", "native descriptors are in the kit")

	// a build with errors reports diagnostics and publish is refused
	_, err = e.svc.PutFile(ctx, e.author, ns, "src/bad.ts", []byte("const x: number = BAD_TYPE"), nil)
	require.NoError(t, err)
	rep, err := e.svc.Build(ctx, e.author, ns)
	require.NoError(t, err)
	require.True(t, rep.HasErrors)
	require.Equal(t, "src/bad.ts", rep.Diagnostics[0].File)
	_, err = e.svc.Publish(ctx, e.admin, ns, "1.0.0")
	var de *manage.DiagnosticsError
	require.ErrorAs(t, err, &de)
	require.Equal(t, "ts.2345", de.Diagnostics[0].Code)
	require.Zero(t, e.count(`SELECT count(*) FROM extruntime.versions`))
	require.NoError(t, e.svc.DeleteFile(ctx, e.author, ns, "src/bad.ts"))

	rep, err = e.svc.Build(ctx, e.author, ns)
	require.NoError(t, err)
	require.False(t, rep.HasErrors)
	require.NotEmpty(t, rep.ArtifactHash)
	tr, err := e.svc.Test(ctx, e.author, ns)
	require.NoError(t, err)
	require.True(t, tr.Passed)

	// publishing needs the publish permission
	_, err = e.svc.Publish(ctx, e.author, ns, "1.0.0")
	require.Equal(t, serrors.PermissionDenied, serrors.CodeOf(err))
	v, err := e.svc.Publish(ctx, e.admin, ns, "")
	require.NoError(t, err)
	require.Equal(t, "1.0.0", v.Version.Version, "defaults to the package.json version")
	require.Equal(t, manage.ProvenanceRuntime, v.Provenance)
	require.Equal(t, rep.SourceHash, v.SourceHash)
	require.Equal(t, rep.ArtifactHash, v.ArtifactHash)
	require.NotEmpty(t, v.KitHash)
	require.Equal(t, 1, e.count(`SELECT count(*) FROM extruntime.catalog_snapshots WHERE hash = $1`, v.KitHash))
	require.Equal(t, int64(1), v.AuthorID)
	require.False(t, v.Approved)
	for _, r := range v.Manifest.Resources {
		require.Equal(t, manifest.ContractHashOf(r), r.ContractHash, r.ID)
		require.NotEmpty(t, r.ContractHash)
	}
	_, err = e.svc.Publish(ctx, e.admin, ns, "1.0.0")
	require.Equal(t, serrors.AlreadyExists, serrors.CodeOf(err), "versions are never overwritten")

	// activation needs an approved version
	_, err = e.svc.Activate(ctx, e.admin, ns, "1.0.0")
	require.Equal(t, serrors.FailedPrecondition, serrors.CodeOf(err))
	require.Contains(t, err.Error(), "not approved")

	// no review permission -> refused; a rejection needs a comment
	_, err = e.svc.Review(ctx, e.author, ns, "1.0.0", true, "")
	require.Equal(t, serrors.PermissionDenied, serrors.CodeOf(err))
	_, err = e.svc.Review(ctx, e.reviewer, ns, "1.0.0", false, "")
	require.Equal(t, serrors.Invalid, serrors.CodeOf(err))
	rv, err := e.svc.Review(ctx, e.reviewer, ns, "1.0.0", false, "needs work")
	require.NoError(t, err)
	require.Equal(t, v.SourceHash, rv.SourceHash)
	_, err = e.svc.Activate(ctx, e.admin, ns, "1.0.0")
	require.Error(t, err, "a rejection does not approve")
	// reviewing exactly what was seen
	_, err = e.svc.ReviewExact(ctx, e.reviewer, ns, "1.0.0", manage.ReviewInput{Approve: true, ExpectSourceHash: "other"})
	require.Equal(t, serrors.Conflict, serrors.CodeOf(err))
	// self-review: the publisher holds the review permission
	rv, err = e.svc.Review(ctx, e.admin, ns, "1.0.0", true, "self-reviewed")
	require.NoError(t, err)
	require.Equal(t, v.ArtifactHash, rv.ArtifactHash)
	got, err := e.svc.GetVersion(ctx, e.reader, ns, "1.0.0")
	require.NoError(t, err)
	require.True(t, got.Approved)
	require.Equal(t, int64(1), got.LatestReview.ReviewerID)

	// activate: DDL applied, own commands bound, catalog updated
	_, ok, err := e.cat.Resolve(ctx, ns+".policy")
	require.NoError(t, err)
	require.False(t, ok)
	res, err := e.svc.Activate(ctx, e.admin, ns, "1.0.0")
	require.NoError(t, err)
	require.NotEmpty(t, res.SchemaSteps)
	require.Len(t, res.Bound, 2)
	require.Equal(t, 1, e.count(`SELECT count(*) FROM information_schema.tables WHERE table_schema = 'x_acme_insurance' AND table_name = 'policy'`))
	require.Equal(t, map[string]string{
		"command:" + ns + ".renewPolicy":    ns + "@1.0.0/renew",
		"command:" + ns + ".activatePolicy": ns + "@1.0.0/activate",
	}, e.openBindings())
	rr, ok, err := e.cat.Resolve(ctx, ns+".policy")
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, ns, rr.Package)
	require.Equal(t, "1.0.0", rr.Version)
	require.NotEmpty(t, rr.Resource.ContractHash)
	art, secrets, err := e.svc.ResolveArtifact(ctx, ns, "1.0.0")
	require.NoError(t, err)
	require.Empty(t, secrets)
	require.FileExists(t, art.Dir+"/server.mjs")
	require.Equal(t, v.ArtifactHash, art.Hash)

	pk, err := e.svc.GetPackage(ctx, e.reader, ns)
	require.NoError(t, err)
	require.Equal(t, "1.0.0", pk.ActiveVersion)
	_, err = e.svc.Activate(ctx, e.admin, ns, "1.0.0")
	require.Equal(t, serrors.AlreadyExists, serrors.CodeOf(err))

	// version 2 adds an optional field and switches the bindings
	m2 := insuranceManifest(tm(manifest.KindString), manifest.Field{ID: "notes", Type: opt(tm(manifest.KindString))})
	e.draft(ns, "2.0.0", m2, nil)
	res2 := e.release(ns, "2.0.0")
	require.Len(t, res2.Bound, 2)
	require.Contains(t, res2.SchemaSteps[0], "notes")
	require.Equal(t, map[string]string{
		"command:" + ns + ".renewPolicy":    ns + "@2.0.0/renew",
		"command:" + ns + ".activatePolicy": ns + "@2.0.0/activate",
	}, e.openBindings())
	require.Equal(t, 2, e.count(`SELECT count(*) FROM extruntime.bindings WHERE version = '1.0.0' AND active_to IS NOT NULL`), "old rows are history")
	hist, err := e.svc.ListActivations(ctx, e.reader, ns)
	require.NoError(t, err)
	require.Len(t, hist, 2)
	require.Equal(t, manage.ActivationActive, hist[0].State)
	require.Equal(t, "2.0.0", hist[0].Version)
	require.Equal(t, manage.ActivationDisabled, hist[1].State)

	// version and artifact rows never mutate
	for _, q := range []string{
		`UPDATE extruntime.versions SET version = 'x' WHERE version = '1.0.0'`,
		`DELETE FROM extruntime.versions WHERE version = '1.0.0'`,
		`UPDATE extruntime.artifacts SET content = '\x00'`,
		`DELETE FROM extruntime.artifacts`,
		`UPDATE extruntime.version_reviews SET approve = false`,
	} {
		_, err := e.db.Exec(ctx, q)
		require.Error(t, err, q)
	}
	old, err := e.svc.GetVersion(ctx, e.reader, ns, "1.0.0")
	require.NoError(t, err)
	require.Equal(t, v.SourceHash, old.SourceHash)
	require.False(t, old.Active)
	require.True(t, old.Approved)
	// v1 can be served again from its stored artifact
	_, _, err = e.svc.ResolveArtifact(ctx, ns, "1.0.0")
	require.NoError(t, err)

	// deactivate removes contributions and bindings but keeps the data
	e.exec(`INSERT INTO x_acme_insurance.policy (tenant_id, id, number) VALUES ($1, gen_random_uuid(), 'P-1')`, e.tenant)
	require.NoError(t, e.svc.Deactivate(ctx, e.admin, ns))
	require.Empty(t, e.openBindings())
	_, ok, _ = e.cat.Resolve(ctx, ns+".policy")
	require.False(t, ok)
	require.Equal(t, 1, e.count(`SELECT count(*) FROM x_acme_insurance.policy`), "deactivation keeps data")
	pk, _ = e.svc.GetPackage(ctx, e.reader, ns)
	require.Empty(t, pk.ActiveVersion)
	require.Error(t, e.svc.Deactivate(ctx, e.admin, ns), "nothing active any more")

	// purge is a separate, explicit action
	require.Equal(t, serrors.PermissionDenied, serrors.CodeOf(e.svc.Purge(ctx, e.author, ns)))
	require.NoError(t, e.svc.Purge(ctx, e.admin, ns))
	require.Zero(t, e.count(`SELECT count(*) FROM information_schema.tables WHERE table_schema = 'x_acme_insurance' AND table_name = 'policy'`))
	require.Zero(t, e.count(`SELECT count(*) FROM extruntime.entity_schemas WHERE package = $1`, ns))
	hist, _ = e.svc.ListActivations(ctx, e.reader, ns)
	for _, a := range hist {
		require.Equal(t, manage.ActivationPurged, a.State)
	}
	// the immutable versions survive a purge and can be activated again
	_, err = e.svc.Activate(ctx, e.admin, ns, "2.0.0")
	require.NoError(t, err)
	require.Error(t, e.svc.Purge(ctx, e.admin, ns), "an active package cannot be purged")
}

func (e *env) exec(sql string, args ...any) {
	e.t.Helper()
	_, err := e.db.Exec(context.Background(), sql, args...)
	require.NoError(e.t, err)
}

func TestIncompatibleSchemaActivationIsBlocked(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	required := tm(manifest.KindString)
	e.draft(ns, "1.0.0", insuranceManifest(required), nil)
	e.release(ns, "1.0.0")

	// a type change would lose data: refused with the data layer's diagnostic
	e.draft(ns, "2.0.0", insuranceManifest(tm(manifest.KindInt)), nil)
	_, err := e.svc.Publish(ctx, e.admin, ns, "2.0.0")
	require.NoError(t, err)
	_, err = e.svc.Review(ctx, e.admin, ns, "2.0.0", true, "ok")
	require.NoError(t, err)
	_, err = e.svc.Activate(ctx, e.admin, ns, "2.0.0")
	require.Error(t, err)
	require.Equal(t, serrors.FailedPrecondition, serrors.CodeOf(err))
	var se *data.SchemaError
	require.ErrorAs(t, err, &se)
	require.Equal(t, data.DiagTypeChange, se.Diagnostics[0].Code)
	require.Equal(t, map[string]string{
		"command:" + ns + ".renewPolicy":    ns + "@1.0.0/renew",
		"command:" + ns + ".activatePolicy": ns + "@1.0.0/activate",
	}, e.openBindings(), "a blocked activation changes nothing")
	pk, _ := e.svc.GetPackage(ctx, e.reader, ns)
	require.Equal(t, "1.0.0", pk.ActiveVersion)

	// relaxing a required field is automatic, but a live workflow instance
	// pinned to v1 expects a value on every row
	e.exec(`INSERT INTO extruntime.workflow_instances (id, tenant_id, workflow, package, version, export, input, status, subject_user_id, subject_kind)
		VALUES (gen_random_uuid(), $1, $2, $3, '1.0.0', 'flow', '{}', 'waiting', 1, 'initiator')`, e.tenant, ns+".flow", ns)
	e.draft(ns, "3.0.0", insuranceManifest(opt(tm(manifest.KindString))), nil)
	_, err = e.svc.Publish(ctx, e.admin, ns, "3.0.0")
	require.NoError(t, err)
	_, err = e.svc.Review(ctx, e.admin, ns, "3.0.0", true, "ok")
	require.NoError(t, err)
	_, err = e.svc.Activate(ctx, e.admin, ns, "3.0.0")
	require.ErrorAs(t, err, &se)
	require.Equal(t, data.DiagIncompatiblePinned, se.Diagnostics[0].Code)
	require.Contains(t, err.Error(), "pins an incompatible version")

	// once the instance is done the same activation goes through
	e.exec(`UPDATE extruntime.workflow_instances SET status = 'completed'`)
	res, err := e.svc.Activate(ctx, e.admin, ns, "3.0.0")
	require.NoError(t, err)
	require.Contains(t, res.SchemaSteps[0], "DROP NOT NULL")
	require.Contains(t, e.auditActions(manage.OutcomeFailed), manage.ActionActivate)
}

func TestReplacementBindingOfNativeCommand(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	const other = "acme.rejecter"
	m := manifest.Manifest{
		Schema: manifest.SchemaVersion, Namespace: other,
		Resources: []manifest.Resource{{ID: other + ".dummy", Kind: manifest.ResQuery, Input: emptyObj(), Output: emptyObj()}},
		Implementations: []manifest.Implementation{
			{Export: "rejectIncoming", Implements: "core.commands.rejectIncoming", Kind: manifest.ImplCommand},
		},
	}
	e.draft(other, "1.0.0", m, nil)
	res := e.release(other, "1.0.0")
	require.Empty(t, res.Bound, "a foreign command is never bound by activation")
	require.Empty(t, e.openBindings())

	// replacement is explicit, validated against the public contract
	_, err := e.svc.Bind(ctx, e.author, manage.SlotCommand, "core.commands.rejectIncoming", other, "1.0.0", "rejectIncoming")
	require.Equal(t, serrors.PermissionDenied, serrors.CodeOf(err))
	_, err = e.svc.Bind(ctx, e.admin, manage.SlotCommand, "core.commands.rejectIncoming", other, "1.0.0", "noSuchExport")
	require.Equal(t, serrors.Invalid, serrors.CodeOf(err), "only an export the version implements for that contract can be bound")
	_, err = e.svc.Bind(ctx, e.admin, manage.SlotCommand, "core.events.userCreated.v1", other, "1.0.0", "rejectIncoming")
	require.Equal(t, serrors.Invalid, serrors.CodeOf(err))
	genBefore := e.cat.Generation()
	b, err := e.svc.Bind(ctx, e.admin, manage.SlotCommand, "core.commands.rejectIncoming", other, "1.0.0", "rejectIncoming")
	require.NoError(t, err)
	require.Equal(t, other, b.Package)
	require.Greater(t, e.cat.Generation(), genBefore, "a binding change invalidates the catalog before Bind returns")
	require.Equal(t, map[string]string{"command:core.commands.rejectIncoming": other + "@1.0.0/rejectIncoming"}, e.openBindings())
	_, err = e.svc.Bind(ctx, e.admin, manage.SlotCommand, "core.commands.rejectIncoming", other, "1.0.0", "rejectIncoming")
	require.Equal(t, serrors.AlreadyExists, serrors.CodeOf(err))

	// a new version that still implements the command keeps the replacement
	e.draft(other, "2.0.0", m, nil)
	res = e.release(other, "2.0.0")
	require.Len(t, res.Rebound, 1)
	require.Equal(t, map[string]string{"command:core.commands.rejectIncoming": other + "@2.0.0/rejectIncoming"}, e.openBindings())

	// a version that no longer implements it releases the slot: no fallback
	m3 := m
	m3.Implementations = nil
	e.draft(other, "3.0.0", m3, nil)
	res = e.release(other, "3.0.0")
	require.Len(t, res.Released, 1)
	require.Empty(t, e.openBindings())

	// unbind returns the slot; deactivating the package does too
	_, err = e.svc.Activate(ctx, e.admin, other, "2.0.0")
	require.NoError(t, err)
	_, err = e.svc.Bind(ctx, e.admin, manage.SlotCommand, "core.commands.rejectIncoming", other, "2.0.0", "rejectIncoming")
	require.NoError(t, err)
	require.NoError(t, e.svc.Unbind(ctx, e.admin, manage.SlotCommand, "core.commands.rejectIncoming"))
	require.Empty(t, e.openBindings())
	require.Equal(t, serrors.NotFound, serrors.CodeOf(e.svc.Unbind(ctx, e.admin, manage.SlotCommand, "core.commands.rejectIncoming")))
	_, err = e.svc.Bind(ctx, e.admin, manage.SlotCommand, "core.commands.rejectIncoming", other, "2.0.0", "rejectIncoming")
	require.NoError(t, err)
	require.NoError(t, e.svc.Deactivate(ctx, e.admin, other))
	require.Empty(t, e.openBindings())
	// a version that is not active cannot hold a binding
	_, err = e.svc.Bind(ctx, e.admin, manage.SlotCommand, "core.commands.rejectIncoming", other, "2.0.0", "rejectIncoming")
	require.Equal(t, serrors.FailedPrecondition, serrors.CodeOf(err))
}

func TestReplacementOfAnotherPackagesCommandFallsBackToTheOwner(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	e.draft(ns, "1.0.0", insuranceManifest(tm(manifest.KindString)), nil)
	e.release(ns, "1.0.0")

	const other = "acme.override"
	m := manifest.Manifest{
		Schema: manifest.SchemaVersion, Namespace: other,
		Resources:       []manifest.Resource{{ID: other + ".dummy", Kind: manifest.ResQuery, Input: emptyObj(), Output: emptyObj()}},
		Implementations: []manifest.Implementation{{Export: "renew2", Implements: ns + ".renewPolicy", Kind: manifest.ImplCommand}},
	}
	e.draft(other, "1.0.0", m, nil)
	e.release(other, "1.0.0")
	_, err := e.svc.Bind(ctx, e.admin, manage.SlotCommand, ns+".renewPolicy", other, "1.0.0", "renew2")
	require.NoError(t, err)
	require.Equal(t, other+"@1.0.0/renew2", e.openBindings()["command:"+ns+".renewPolicy"])

	// the owner's new version does not steal the slot back
	e.draft(ns, "2.0.0", insuranceManifest(tm(manifest.KindString)), nil)
	e.release(ns, "2.0.0")
	require.Equal(t, other+"@1.0.0/renew2", e.openBindings()["command:"+ns+".renewPolicy"])
	require.Equal(t, ns+"@2.0.0/activate", e.openBindings()["command:"+ns+".activatePolicy"])

	// deactivating the replacement reinstates the owner's current implementation
	require.NoError(t, e.svc.Deactivate(ctx, e.admin, other))
	require.Equal(t, ns+"@2.0.0/renew", e.openBindings()["command:"+ns+".renewPolicy"])
}

func TestTriggerOwners(t *testing.T) {
	rights := subjectsOf{}
	e := newEnv(t, withSubjects(rights))
	ctx := context.Background()
	e.draft(ns, "1.0.0", insuranceManifest(tm(manifest.KindString)), nil)
	user := e.newUser("owner")
	rights[user] = []string{manage.PermRead}

	_, err := e.svc.AssignOwner(ctx, e.admin, ns+".onRenewed", user)
	require.Equal(t, serrors.NotFound, serrors.CodeOf(err), "the trigger is not active yet")
	e.release(ns, "1.0.0")
	_, err = e.svc.AssignOwner(ctx, e.author, ns+".onRenewed", user)
	require.Equal(t, serrors.PermissionDenied, serrors.CodeOf(err))
	_, err = e.svc.AssignOwner(ctx, e.admin, ns+".onRenewed", 999999)
	require.Equal(t, serrors.Invalid, serrors.CodeOf(err), "the owner must belong to the tenant")
	o, err := e.svc.AssignOwner(ctx, e.admin, ns+".onRenewed", user)
	require.NoError(t, err)
	require.Equal(t, user, o.UserID)
	got, ok, err := e.svc.OwnerOf(ctx, e.tenant, ns+".onRenewed")
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, user, got)
	_, ok, err = e.svc.OwnerOf(ctx, uuidOther(), ns+".onRenewed")
	require.NoError(t, err)
	require.False(t, ok, "owners are per tenant")
	list, err := e.svc.ListOwners(ctx, e.reader)
	require.NoError(t, err)
	require.Len(t, list, 1)
	require.NoError(t, e.svc.RemoveOwner(ctx, e.admin, ns+".onRenewed"))
	_, ok, _ = e.svc.OwnerOf(ctx, e.tenant, ns+".onRenewed")
	require.False(t, ok)
}

var _ = build.Tree{}

func TestForeignImplementationsAreValidatedAgainstTheCatalogAtPublish(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	const other = "acme.checker"
	base := func(impls ...manifest.Implementation) manifest.Manifest {
		return manifest.Manifest{
			Schema: manifest.SchemaVersion, Namespace: other,
			Resources:       []manifest.Resource{{ID: other + ".dummy", Kind: manifest.ResQuery, Input: emptyObj(), Output: emptyObj()}},
			Implementations: impls,
		}
	}
	publishDiags := func(version string, m manifest.Manifest) []string {
		e.draft(other, version, m, nil)
		_, err := e.svc.Publish(ctx, e.admin, other, version)
		require.Error(t, err)
		require.Equal(t, serrors.FailedPrecondition, serrors.CodeOf(err))
		var de *manage.DiagnosticsError
		require.ErrorAs(t, err, &de)
		var codes []string
		for _, d := range de.Diagnostics {
			require.Equal(t, build.SeverityError, d.Severity)
			codes = append(codes, d.Code)
		}
		return codes
	}
	// an atomic implementation of an ordinary native command
	require.Equal(t, []string{"contract.kindMismatch"}, publishDiags("1.0.0", base(
		manifest.Implementation{Export: "r", Implements: "core.commands.rejectIncoming", Kind: manifest.ImplAtomic})))
	// a query implementation of a command, an event as a command, an unknown contract
	require.Equal(t, []string{"contract.kindMismatch", "contract.kindMismatch", "contract.unknown"}, publishDiags("1.0.1", base(
		manifest.Implementation{Export: "a", Implements: "core.commands.rejectIncoming", Kind: manifest.ImplQuery},
		manifest.Implementation{Export: "b", Implements: "core.events.userCreated.v1", Kind: manifest.ImplCommand},
		manifest.Implementation{Export: "c", Implements: "core.commands.Nope", Kind: manifest.ImplCommand})))
	// an own implementation must match its own contract
	own := base(manifest.Implementation{Export: "o", Implements: other + ".dummy", Kind: manifest.ImplCommand})
	require.Equal(t, []string{"contract.kindMismatch"}, publishDiags("1.0.2", own))
	require.Zero(t, e.count(`SELECT count(*) FROM extruntime.versions WHERE namespace = $1`, other))

	// the matching implementation publishes; the contract hash is recorded
	e.draft(other, "1.1.0", base(manifest.Implementation{Export: "r", Implements: "core.commands.rejectIncoming", Kind: manifest.ImplCommand}), nil)
	_, err := e.svc.Publish(ctx, e.admin, other, "1.1.0")
	require.NoError(t, err)
	var recorded string
	require.NoError(t, e.db.QueryRow(ctx, `SELECT foreign_contracts::text FROM extruntime.versions WHERE namespace = $1`, other).Scan(&recorded))
	r, _, _ := e.cat.Resolve(ctx, "core.commands.rejectIncoming")
	require.Contains(t, recorded, r.Resource.ContractHash)
}

func TestActivationIsBlockedWhenAForeignContractChangedSinceTheBuild(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	e.draft(ns, "1.0.0", insuranceManifest(tm(manifest.KindString)), nil)
	e.release(ns, "1.0.0")

	const other = "acme.override"
	m := manifest.Manifest{
		Schema: manifest.SchemaVersion, Namespace: other,
		Resources:       []manifest.Resource{{ID: other + ".dummy", Kind: manifest.ResQuery, Input: emptyObj(), Output: emptyObj()}},
		Implementations: []manifest.Implementation{{Export: "renew2", Implements: ns + ".renewPolicy", Kind: manifest.ImplCommand}},
	}
	e.draft(other, "1.0.0", m, nil)
	_, err := e.svc.Publish(ctx, e.admin, other, "1.0.0")
	require.NoError(t, err)
	_, err = e.svc.Review(ctx, e.admin, other, "1.0.0", true, "ok")
	require.NoError(t, err)

	// the owner changes the public input of renewPolicy before the override goes live
	m2 := insuranceManifest(tm(manifest.KindString))
	m2.Resources[1].Input = &manifest.Type{Kind: manifest.KindObject, Properties: []manifest.Property{{Name: "months", Type: tm(manifest.KindInt)}}}
	e.draft(ns, "2.0.0", m2, nil)
	e.release(ns, "2.0.0")

	_, err = e.svc.Activate(ctx, e.admin, other, "1.0.0")
	require.Equal(t, serrors.FailedPrecondition, serrors.CodeOf(err))
	require.Contains(t, err.Error(), "changed since the version was built")

	// rebuilt against the new contract it goes live
	e.draft(other, "1.0.1", m, nil)
	e.release(other, "1.0.1")
}
