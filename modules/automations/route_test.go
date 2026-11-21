package automations_test

import (
	"net/http"
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/modules"
	"github.com/iota-uz/iota-sdk/modules/automations"
	"github.com/iota-uz/iota-sdk/modules/automations/permissions"
	"github.com/iota-uz/iota-sdk/modules/automations/presentation/integrations"
	corePerms "github.com/iota-uz/iota-sdk/modules/core/permissions"
	"github.com/iota-uz/iota-sdk/pkg/itf"
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
	script := regexp.MustCompile(`<script type="module" src="(/assets/automations/assets/integrations-[^"]+\.js)"`).FindStringSubmatch(body)
	require.Len(t, script, 2, "the page loads the module's own bundle")
	asset := manager.GET(script[1]).Expect(t).Status(http.StatusOK)
	assert.Contains(t, asset.Header("Cache-Control"), "immutable")
	manager.GET("/assets/automations/bundle.json").Expect(t).Status(http.StatusNotFound)

	var found bool
	for _, s := range manager.Env().Container.SettingsSections() {
		for _, p := range s.Pages {
			if p.Key == automations.SettingsPageIntegrations {
				found = true
			}
		}
	}
	assert.True(t, found, "the settings hub lists the page")

	other := mount(t, itf.NewSuiteBuilder(t).WithComponents(modules.Components()...).AsUser(corePerms.UserRead).Build())
	other.GET("/settings/integrations").Expect(t).Status(http.StatusForbidden).NotContains(`iota-client-route-mount`)
	other.POST("/settings/integrations/rpc").JSON(map[string]any{"jsonrpc": "2.0", "id": 1, "method": "automations.connectors.list", "params": map[string]any{}}).
		Expect(t).Status(http.StatusForbidden).Contains(`"code":"forbidden"`)
}

// mount registers the integrations controller the application composed (the whole route table needs
// bundles of other modules that a test build does not have).
func mount(t *testing.T, s *itf.Suite) *itf.Suite {
	t.Helper()
	for _, c := range s.Env().App.Controllers() {
		if c.Descriptor().ID == integrations.ControllerID {
			return s.Register(c)
		}
	}
	t.Fatal("the integrations controller is not composed")
	return nil
}
