package manage_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/pkg/extruntime/manage"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/manifest"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

// The documentation a package ships is part of the version: publish refuses
// a manifest whose documents the artifact does not hold, or whose permissions
// the catalog does not know; activation checks the stored artifact again.
func TestPublishValidatesPackageDocumentation(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	docs := func(extra ...manifest.Doc) manifest.Manifest {
		m := insuranceManifest(tm(manifest.KindString))
		m.Resources = append(m.Resources, manifest.Resource{ID: ns + ".manage", Kind: manifest.ResPermission})
		m.Docs = append([]manifest.Doc{{Path: "start", Locale: "en", Title: "Start"}}, extra...)
		return m
	}
	files := map[string]string{"docs/en/start.md": "# Start\n"}

	// the draft keeps files between drafts: start each with no documents
	clearDocs := func() {
		t.Helper()
		fs, err := e.svc.ListFiles(ctx, e.reader, ns)
		if err != nil {
			return // the package does not exist yet
		}
		for _, f := range fs {
			if strings.HasPrefix(f.Path, "docs/") {
				require.NoError(t, e.svc.DeleteFile(ctx, e.author, ns, f.Path))
			}
		}
	}

	refused := func(version string, m manifest.Manifest, extra map[string]string, want string) {
		t.Helper()
		clearDocs()
		e.draft(ns, version, m, extra)
		_, err := e.svc.Publish(ctx, e.admin, ns, version)
		require.Error(t, err, version)
		require.Equal(t, serrors.FailedPrecondition, serrors.CodeOf(err), version)
		require.Contains(t, err.Error()+diagnosticMessages(err), want, version)
	}

	// the manifest lists a document the artifact does not hold
	refused("1.0.0", docs(manifest.Doc{Path: "missing", Locale: "en", Title: "Missing"}), files, "docs/en/missing.md")
	// the artifact holds a document the manifest does not list
	refused("1.0.1", docs(), map[string]string{"docs/en/start.md": "# Start\n", "docs/en/stray.md": "x"}, "docs/en/stray.md, which the manifest does not list")
	// structure the build would have refused
	refused("1.0.2", docs(manifest.Doc{Path: "../x", Locale: "en", Title: "x"}), files, "document path")
	refused("1.0.3", docs(manifest.Doc{Path: "own", Locale: "en", Title: "x", Permission: ns + ".nope"}), files, "docs.unknownPermission")
	// a permission of neither the package nor the catalog
	refused("1.0.4", docs(manifest.Doc{Path: "gated", Locale: "en", Title: "Gated", Permission: "other.pkg.manage"}),
		map[string]string{"docs/en/start.md": "# Start\n", "docs/en/gated.md": "x"}, "docs.unknownPermission")
	require.Zero(t, e.count(`SELECT count(*) FROM extruntime.versions WHERE namespace = $1`, ns))

	// declared, native and catalog permissions are accepted; the version
	// carries its documents and they activate
	ok := docs(
		manifest.Doc{Path: "setup", Locale: "en", Title: "Setup", Permission: ns + ".manage"},
		manifest.Doc{Path: "limits", Locale: "en", Title: "Limits", Permission: "Client.Read"},
	)
	clearDocs()
	e.draft(ns, "2.0.0", ok, map[string]string{
		"docs/en/start.md": "# Start\n", "docs/en/setup.md": "Setup\n", "docs/en/limits.md": "Limits\n",
	})
	e.release(ns, "2.0.0")
	active, err := e.cat.Active(ctx)
	require.NoError(t, err)
	require.Len(t, active, 1)
	require.Len(t, active[0].Manifest.Docs, 3)
	tree, err := e.arts.Tree(ctx, active[0].ArtifactHash)
	require.NoError(t, err)
	require.Equal(t, "Setup\n", string(tree["docs/en/setup.md"]))

	// A version whose stored artifact lost a document (it did not come from
	// the build) is not activated, even though it was published and approved.
	clearDocs()
	e.draft(ns, "2.1.0", ok, map[string]string{
		"docs/en/start.md": "# Start\n", "docs/en/setup.md": "Setup 2\n", "docs/en/limits.md": "Limits 2\n",
	})
	_, err = e.svc.Publish(ctx, e.admin, ns, "2.1.0")
	require.NoError(t, err)
	_, err = e.svc.Review(ctx, e.admin, ns, "2.1.0", true, "lgtm")
	require.NoError(t, err)
	// Rows are immutable by trigger; the test plays the operator who bypassed it.
	_, err = e.db.Exec(ctx, `ALTER TABLE extruntime.versions DISABLE TRIGGER USER`)
	require.NoError(t, err)
	_, err = e.db.Exec(ctx, `UPDATE extruntime.versions
		SET manifest = jsonb_set(manifest, '{docs}', manifest->'docs' || '[{"path":"ghost","locale":"en","title":"Ghost"}]'::jsonb)
		WHERE namespace = $1 AND version = '2.1.0'`, ns)
	require.NoError(t, err)
	_, err = e.db.Exec(ctx, `ALTER TABLE extruntime.versions ENABLE TRIGGER USER`)
	require.NoError(t, err)
	_, err = e.svc.Activate(ctx, e.admin, ns, "2.1.0")
	require.ErrorContains(t, err, "docs/en/ghost.md")
	require.Equal(t, serrors.FailedPrecondition, serrors.CodeOf(err))
}

func diagnosticMessages(err error) string {
	var de *manage.DiagnosticsError
	if !errors.As(err, &de) {
		return ""
	}
	out := ""
	for _, d := range de.Diagnostics {
		out += " " + d.File + " " + d.Code + " " + d.Message
	}
	return out
}
