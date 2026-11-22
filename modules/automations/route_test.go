package automations_test

import (
	"net/http"
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/modules"
	"github.com/iota-uz/iota-sdk/modules/automations/permissions"
	"github.com/iota-uz/iota-sdk/modules/automations/presentation/integrations"
	corePerms "github.com/iota-uz/iota-sdk/modules/core/permissions"
	"github.com/iota-uz/iota-sdk/modules/core/presentation/controllers"
	"github.com/iota-uz/iota-sdk/pkg/itf"
	"github.com/iota-uz/iota-sdk/pkg/sdkidentity"
)

// The module as modules.Components lists it, with no options and no application bundle: a manager
// opens the page from the embedded bundle; anyone else gets the 403 page and no settings entry.
func TestIntegrationsPageThroughTheApplicationRoutes(t *testing.T) {
	manager := mount(t, itf.NewSuiteBuilder(t).WithComponents(modules.Components()...).AsUser(permissions.ConnectionsManage).Build())
	body := manager.GET("/settings/integrations").Expect(t).Status(http.StatusOK).
		Contains(`id="iota-client-route-mount"`).
		Contains(`"id":"automations.integrations"`).
		Contains(`"rpc":"/settings/integrations/rpc"`).
		Body()
	script := regexp.MustCompile(`<script type="module" src="(/assets/automations/[0-9][^/]*/assets/integrations-[^"]+\.js)"`).FindStringSubmatch(body)
	require.Len(t, script, 2, "the page loads the module's own bundle")
	asset := manager.GET(script[1]).Expect(t).Status(http.StatusOK)
	assert.Contains(t, asset.Header("Cache-Control"), "immutable")
	manager.GET("/assets/automations/" + sdkidentity.ReleaseVersion + "/bundle.json").Expect(t).Status(http.StatusNotFound)

	// The settings hub as each user gets it: the entry links the page for a manager only.
	manager.GET("/settings").Expect(t).Status(http.StatusOK).Contains(`href="/settings/integrations"`)

	other := mount(t, itf.NewSuiteBuilder(t).WithComponents(modules.Components()...).AsUser(corePerms.UserRead).Build())
	other.GET("/settings/integrations").Expect(t).Status(http.StatusForbidden).NotContains(`iota-client-route-mount`)
	other.GET("/settings").Expect(t).Status(http.StatusOK).NotContains(`href="/settings/integrations"`)
	other.POST("/settings/integrations/rpc").JSON(map[string]any{"jsonrpc": "2.0", "id": 1, "method": "automations.connectors.list", "params": map[string]any{}}).
		Expect(t).Status(http.StatusForbidden).Contains(`"code":"forbidden"`)
}

// mount registers the integrations controller the application composed (the whole route table needs
// bundles of other modules that a test build does not have).
func mount(t *testing.T, s *itf.Suite) *itf.Suite {
	t.Helper()
	for _, c := range s.Env().App.Controllers() {
		if c.Descriptor().ID == integrations.ControllerID {
			// The hub renders the registry of the request's container, filtered for the user.
			return s.Register(c).Register(controllers.NewSettingsHubController())
		}
	}
	t.Fatal("the integrations controller is not composed")
	return nil
}
