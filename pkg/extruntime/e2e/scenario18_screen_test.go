package e2e_test

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/pkg/extruntime/manage"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/manifest"
)

// Scenario 18: a native screen (the CRM client view, route
// crm.routes.clientView) is replaced by a package page with the route's typed
// parameters, through a screen binding; the native requirement still applies;
// HTMX fragments keep reaching the native handler; reverting restores the
// Templ page.
func TestScenario18ScreenReplacementOfTheClientView(t *testing.T) {
	e := newEnv(t)
	admin := e.admin
	viewer := e.newActor("viewer", "Client.Read")
	outsider := e.newActor("outsider") // authenticated, but without the native screen's permission
	id := e.newClient(admin, "Ann")
	path := "/crm/clients/" + id

	e.deploy(admin, "e2e.clientpage", "1.0.0", loadTree(t, "clientpage"))

	// A page for a platform route is only a candidate: the native screen serves until it is bound.
	nativeResp := e.get(viewer, path)
	require.Equal(t, http.StatusOK, nativeResp.Status, nativeResp.String())
	native := nativeResp.String()
	require.Contains(t, native, `data-testid="crm-client-view"`)
	require.Contains(t, native, "Ann Doe")
	require.NotContains(t, native, "iota-client-context")

	// The binding is an explicit decision about the screen; an export that is not a page is refused.
	e.bind(admin, manage.SlotScreen, "crm.routes.clientView", "e2e.clientpage", "1.0.0", "noSuchPage", http.StatusBadRequest)
	e.bind(admin, manage.SlotScreen, "crm.routes.clientView", "e2e.clientpage", "1.0.0", "clientView", http.StatusCreated)

	// The UI source caches the snapshot for a few milliseconds; the binding is served from the next refresh on.
	var body string
	require.Eventually(t, func() bool {
		body = e.get(viewer, path).String()
		return !strings.Contains(body, `data-testid="crm-client-view"`)
	}, 3*time.Second, 20*time.Millisecond, "the native page no longer serves")
	pd := pageData(t, body)
	require.Equal(t, "e2e.clientpage.clientView", pd.Page.ID)
	require.Equal(t, "crm.routes.clientView", pd.ReplacedRoute)
	require.Equal(t, map[string]string{"id": id}, pd.Params, "the path parameter reaches the page")
	require.NotNil(t, pd.ParamTypes, "typed by the platform route")
	require.Equal(t, manifest.KindObject, pd.ParamTypes.Kind)
	require.Equal(t, "id", pd.ParamTypes.Properties[0].Name)
	require.Equal(t, manifest.KindInt, pd.ParamTypes.Properties[0].Type.Kind, "decoded to a number in the browser")
	js := e.get(viewer, pd.Page.Module)
	require.Equal(t, http.StatusOK, js.Status)

	// The replacement inherits the native requirement: it never widens access to the screen.
	require.Equal(t, http.StatusForbidden, e.get(outsider, path).Status)
	// A value the route's type refuses is not served by the package page.
	require.Equal(t, http.StatusNotFound, e.get(viewer, "/crm/clients/abc").Status)
	// The HTMX drawer fragment is not the screen: it still reaches the native handler.
	frag := e.do(viewer, http.MethodGet, path, nil, "HX-Request", "true", "HX-Target", "view-drawer")
	require.Equal(t, http.StatusOK, frag.Status)
	require.NotContains(t, frag.String(), "iota-client-context")
	require.Contains(t, frag.String(), "Ann")
	// The rest of the native module is untouched.
	require.Equal(t, http.StatusOK, e.get(viewer, "/crm/clients").Status)

	// Reverting restores the Templ page without a restart; the history keeps both decisions.
	e.unbind(admin, manage.SlotScreen, "crm.routes.clientView")
	require.Eventually(t, func() bool {
		r := e.get(viewer, path)
		require.Equal(t, http.StatusOK, r.Status, "a request routed just before the refresh is still served")
		body = r.String()
		return strings.Contains(body, `data-testid="crm-client-view"`)
	}, 3*time.Second, 20*time.Millisecond, "the Templ page is back")
	require.NotContains(t, body, "iota-client-context")
	var hist []manage.Binding
	e.mgmt(admin, http.MethodGet, "/bindings?history=true", nil, http.StatusOK, &hist)
	var replaced []manage.Binding
	for _, b := range hist {
		if b.TargetID == "crm.routes.clientView" && b.SlotKind == manage.SlotScreen {
			replaced = append(replaced, b)
		}
	}
	require.Len(t, replaced, 1)
	require.NotNil(t, replaced[0].ActiveTo)
}
