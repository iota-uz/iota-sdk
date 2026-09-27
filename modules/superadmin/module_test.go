package superadmin_test

import (
	"os"
	"testing"

	"github.com/iota-uz/iota-sdk/modules"
	"github.com/iota-uz/iota-sdk/modules/superadmin"
	"github.com/iota-uz/iota-sdk/modules/superadmin/presentation/controllers"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/itf"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMain(m *testing.M) {
	if err := os.Chdir("../../"); err != nil {
		panic(err)
	}
	os.Exit(m.Run())
}

func TestComponent_Register(t *testing.T) {
	t.Parallel()

	t.Run("ModuleInitialization", func(t *testing.T) {
		t.Parallel()

		// Setup test environment with built-in modules and superadmin module
		// Built-in modules are required because superadmin module depends on UserService from core module
		env := itf.Setup(t, itf.WithComponents(modules.Components()...), itf.WithComponents(superadmin.NewComponent(nil)))

		// Verify module is registered
		require.NotNil(t, env, "test environment should be initialized")
		require.NotNil(t, env.App, "application should be initialized")
	})

	t.Run("ModuleName", func(t *testing.T) {
		t.Parallel()

		component := superadmin.NewComponent(nil)
		assert.Equal(t, "superadmin", component.Descriptor().Name)
	})

	t.Run("ModuleWithOptions", func(t *testing.T) {
		t.Parallel()

		opts := &superadmin.ModuleOptions{}
		component := superadmin.NewComponent(opts)

		require.NotNil(t, component)
		assert.Equal(t, "superadmin", component.Descriptor().Name)
	})

	t.Run("ModuleWithNilOptions", func(t *testing.T) {
		t.Parallel()

		component := superadmin.NewComponent(nil)

		require.NotNil(t, component)
		assert.Equal(t, "superadmin", component.Descriptor().Name)
	})
}

func TestNavItems(t *testing.T) {
	t.Parallel()

	// Navigation is declared on each controller's descriptor via WithNav,
	// not as package-level NavigationItem vars.
	navByID := func(c application.Controller) map[string]application.NavNode {
		out := map[string]application.NavNode{}
		for _, n := range c.Descriptor().Nav {
			out[n.ID] = n
		}
		return out
	}

	t.Run("DashboardNav", func(t *testing.T) {
		t.Parallel()

		node, ok := navByID(controllers.NewDashboardController())["superadmin.dashboard"]
		require.True(t, ok, "dashboard nav node should be declared")
		assert.Equal(t, "SuperAdmin.NavigationLinks.Dashboard", node.TitleKey)
		assert.Equal(t, "/", node.Path)
		assert.NotNil(t, node.Icon)
	})

	t.Run("TenantsNav", func(t *testing.T) {
		t.Parallel()

		node, ok := navByID(controllers.NewTenantsController(nil))["superadmin.tenants"]
		require.True(t, ok, "tenants nav node should be declared")
		assert.Equal(t, "SuperAdmin.NavigationLinks.Tenants", node.TitleKey)
		assert.Equal(t, "/superadmin/tenants", node.Path)
		assert.NotNil(t, node.Icon)
	})
}
