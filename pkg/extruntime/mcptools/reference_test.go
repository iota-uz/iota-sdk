package mcptools_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	corePerms "github.com/iota-uz/iota-sdk/modules/core/permissions"
	"github.com/iota-uz/iota-sdk/pkg/mcp"
)

// ext_reference returns the reference markdown of a namespace, filtered by the
// permissions of the caller: Extensions.Read sees everything, other users the
// contracts open to them, and an invisible namespace does not exist for them.
func TestReferenceToolFollowsTheCallersVisibility(t *testing.T) {
	e := newEnv(t)
	const ns = "acme.refdocs"
	e.ok(adminID, "ext_package_create", map[string]string{"namespace": ns}, nil)
	for p, c := range files(t, ns, "1.0.0") {
		if p != "src/bad.ts" {
			e.ok(adminID, "ext_file_write", map[string]string{"namespace": ns, "path": p, "content": c}, nil)
		}
	}
	e.ok(adminID, "ext_publish", map[string]string{"namespace": ns, "version": "1.0.0"}, nil)
	e.ok(adminID, "ext_review", map[string]any{"namespace": ns, "version": "1.0.0", "approve": true}, nil)
	e.ok(adminID, "ext_activate", map[string]string{"namespace": ns, "version": "1.0.0"}, nil)

	const strangerID = 50
	e.users[strangerID] = account(t, e.tenant, strangerID)
	const userReaderID = 51
	e.users[userReaderID] = account(t, e.tenant, userReaderID, corePerms.UserRead)

	type result struct {
		Namespace  string
		Markdown   string
		Namespaces []string
	}

	// a holder of Extensions.Read: the overview lists every namespace, the page everything
	var out result
	e.ok(readerID, "ext_reference", map[string]string{}, &out)
	require.Contains(t, out.Namespaces, ns)
	require.Contains(t, out.Namespaces, "core")
	require.Contains(t, out.Markdown, "# Extension reference")
	e.ok(readerID, "ext_reference", map[string]string{"namespace": ns}, &out)
	require.Equal(t, ns, out.Namespace)
	require.Contains(t, out.Markdown, "### `"+ns+".renewPolicy`")
	require.Contains(t, out.Markdown, "permission `Policy.Renew`")
	require.Contains(t, out.Markdown, "### `"+ns+".policy`")
	require.Contains(t, out.Markdown, "JSON Schema:")
	e.ok(readerID, "ext_reference", map[string]string{"namespace": "core"}, &out)
	require.Contains(t, out.Markdown, "`core.user`")

	// no Extensions.Read, no Policy.Renew: the open entity only
	e.ok(strangerID, "ext_reference", map[string]string{"namespace": ns}, &out)
	require.Contains(t, out.Markdown, "### `"+ns+".policy`")
	require.NotContains(t, out.Markdown, "renewPolicy")
	e.failed(strangerID, "ext_reference", map[string]string{"namespace": "core"}, mcp.CodeNotFound)
	e.ok(strangerID, "ext_reference", map[string]string{}, &out)
	require.Equal(t, []string{ns}, out.Namespaces)

	// User.Read opens the native user entity
	e.ok(userReaderID, "ext_reference", map[string]string{"namespace": "core"}, &out)
	require.Contains(t, out.Markdown, "`core.user`")

	e.failed(readerID, "ext_reference", map[string]string{"namespace": "nope"}, mcp.CodeNotFound)
	e.failed(readerID, "ext_reference", map[string]string{"namespace": "core", "x": "y"}, mcp.CodeInvalidInput)
}
