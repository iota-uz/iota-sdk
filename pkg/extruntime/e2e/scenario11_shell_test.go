package e2e_test

import (
	"encoding/json"
	"net/http"
	"regexp"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/pkg/extruntime/manage"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/ui"
)

var runtimeContextRe = regexp.MustCompile(`<script id="` + ui.RuntimeContextElementID + `" type="application/json">(.*?)</script>`)

func runtimeContext(t *testing.T, body string) *ui.RuntimeContext {
	t.Helper()
	m := runtimeContextRe.FindStringSubmatch(body)
	if m == nil {
		return nil
	}
	var rc ui.RuntimeContext
	require.NoError(t, json.Unmarshal([]byte(m[1]), &rc))
	return &rc
}

// Scenario 11: a package's shellRenderer, bound to the shell slot, replaces
// the whole authenticated shell: legacy Templ/HTMX pages and runtime Solid
// pages carry the same activation (runtime context, host entry) and the same
// host-owned outlet; reverting the binding restores the standard shell.
func TestScenario11ShellRendererForLegacyAndRuntimePages(t *testing.T) {
	e := newEnv(t)
	admin := e.admin
	e.deploy(admin, "e2e.notes", "0.1.0", loadTree(t, "notes"))
	act := e.deploy(admin, "e2e.shell", "1.0.0", loadTree(t, "shell"))
	require.Empty(t, act.Bound, "activating a shell renderer replaces nothing by itself")
	user := e.newActor("shell-user", permNotesWrite, "Client.Read")
	pages := []string{"/crm/clients", "/notes", "/shell-home"}

	// Standard shell: no runtime activation anywhere, the outlet is present.
	for _, path := range pages {
		r := e.get(user, path)
		require.Equal(t, http.StatusOK, r.Status, path)
		require.Nil(t, runtimeContext(t, r.String()), path)
		require.Contains(t, r.String(), "data-iota-shell-outlet", path)
		if path == "/crm/clients" { // runtime pages always load the host entry; a legacy page only with a shell
			require.NotContains(t, r.String(), "/assets/runtime-host.js", path)
		}
	}

	// Binding: an explicit decision about a slot of the application.
	e.bind(admin, manage.SlotShell, "app", "e2e.shell", "1.0.0", "shell", http.StatusCreated)
	e.bind(admin, manage.SlotShell, "app", "e2e.shell", "1.0.0", "home", http.StatusBadRequest) // a page is not a shell
	var module string
	for _, path := range pages {
		r := e.get(user, path)
		require.Equal(t, http.StatusOK, r.Status, path)
		body := r.String()
		rc := runtimeContext(t, body)
		require.NotNil(t, rc, "%s carries the shell activation", path)
		require.Equal(t, "e2e.shell.shell", rc.Shell.ID, path)
		require.Equal(t, "e2e.shell", rc.Shell.Package)
		require.Equal(t, "/_ext/rpc", rc.RPC)
		require.Contains(t, rc.Permissions, permNotesWrite, "the session user's rights, not the package's")
		require.Contains(t, body, `src="/assets/runtime-host.js"`, path)
		// The standard shell and the outlet stay, so the host can fall back and
		// move the one page element into the renderer's outlet.
		require.Contains(t, body, "data-iota-shell-outlet", path)
		require.Contains(t, body, "data-iota-shell-standard", path)
		if module == "" {
			module = rc.Shell.Module
		}
		require.Equal(t, module, rc.Shell.Module, "one renderer module for every page")
		// Workspaces and palette of the package reach the renderer.
		require.Len(t, rc.Navigation.Workspaces, 1, path)
		require.Equal(t, "e2e.shell.main", rc.Navigation.Workspaces[0].ID)
		require.Len(t, rc.Navigation.Palette, 1, path)
		require.Equal(t, "/shell-home", rc.Navigation.Palette[0].Href)
	}
	js := e.get(user, module)
	require.Equal(t, http.StatusOK, js.Status)
	require.Contains(t, js.Header.Get("Content-Type"), "javascript")
	// A runtime page and a legacy page see the same navigation model source.
	legacy := e.get(user, "/crm/clients").String()
	runtimePage := e.get(user, "/notes").String()
	require.Contains(t, legacy, `href="/notes"`)
	require.Contains(t, runtimePage, `href="/notes"`)
	// The runtime page still mounts through the canonical client host.
	require.Contains(t, runtimePage, `id="iota-client-route`)

	// Revert: closing the binding returns the standard shell without a restart.
	e.unbind(admin, manage.SlotShell, "app")
	for _, path := range pages {
		body := e.get(user, path).String()
		require.Nil(t, runtimeContext(t, body), path)
		if path == "/crm/clients" {
			require.NotContains(t, body, "/assets/runtime-host.js", path)
		}
		require.Contains(t, body, "data-iota-shell-outlet", path)
	}

	// The newest binding wins when several are open.
	e.bind(admin, manage.SlotShell, "app", "e2e.shell", "1.0.0", "shell", http.StatusCreated)
	require.NotNil(t, runtimeContext(t, e.get(user, "/notes").String()))
	// Deactivating the shell's package takes the shell binding away with it.
	e.mgmt(admin, http.MethodPost, "/packages/e2e.shell/deactivate", nil, http.StatusNoContent, nil)
	require.Nil(t, runtimeContext(t, e.get(user, "/notes").String()))
}
