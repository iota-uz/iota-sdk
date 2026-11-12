package runtime_test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/pkg/extruntime/manage"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/reference"
)

// The management API serves the live reference of the activated package,
// filtered by the session: Extensions.Read sees everything, a user without
// permissions the open resources only.
func TestReferenceEndpointServesTheActivePackageFilteredBySession(t *testing.T) {
	e := newAPIEnv(t)
	const admin, reader, stranger = 1, 2, 3

	e.json(admin, http.MethodPost, "/packages", map[string]string{"namespace": ns}, http.StatusCreated, nil)
	for p, c := range files(t, "1.0.0") {
		status, out, _ := e.do(admin, http.MethodPut, "/packages/"+ns+"/files/"+p, []byte(c))
		require.Equal(t, http.StatusOK, status, string(out))
	}
	var v manage.VersionDetail
	e.json(admin, http.MethodPost, "/packages/"+ns+"/publish", nil, http.StatusCreated, &v)
	e.json(admin, http.MethodPost, "/packages/"+ns+"/versions/1.0.0/review",
		map[string]any{"approve": true, "expectSourceHash": v.SourceHash, "expectArtifactHash": v.ArtifactHash}, http.StatusCreated, nil)
	e.json(admin, http.MethodPost, "/packages/"+ns+"/versions/1.0.0/activate", nil, http.StatusOK, nil)

	var full reference.Model
	e.json(reader, http.MethodGet, "/reference", nil, http.StatusOK, &full)
	require.True(t, full.Full)
	pkg, ok := full.Namespace(ns)
	require.True(t, ok, "the activated package is part of the live reference")
	require.Equal(t, "1.0.0", pkg.Package.Version)
	cmd, ok := full.Lookup(ns + ".renewPolicy")
	require.True(t, ok)
	require.Equal(t, "{}", cmd.Input.Signature)
	require.Equal(t, "{ ok: bool }", cmd.Output.Signature)
	require.Len(t, pkg.Subscriptions, 1)
	_, ok = full.Lookup("core.user")
	require.True(t, ok, "native contracts are part of it too")

	// no permissions: the open entity and the open command (it declares no
	// permission), but not the gated native entity nor the subscription
	var limited reference.Model
	e.json(stranger, http.MethodGet, "/reference", nil, http.StatusOK, &limited)
	require.False(t, limited.Full)
	_, ok = limited.Lookup(ns + ".policy")
	require.True(t, ok)
	_, ok = limited.Lookup("core.user")
	require.False(t, ok, "core.user needs User.Read")
	pkg, _ = limited.Namespace(ns)
	require.Empty(t, pkg.Subscriptions)

	status, _, _ := e.do(0, http.MethodGet, "/reference", nil)
	require.GreaterOrEqual(t, status, 400)
}
