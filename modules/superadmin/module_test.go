package superadmin_test

import (
	"os"
	"testing"

	"github.com/iota-uz/iota-sdk/modules"
	"github.com/iota-uz/iota-sdk/modules/superadmin"
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

func TestModule_Register(t *testing.T) {
	t.Parallel()

	t.Run("ModuleInitialization", func(t *testing.T) {
		t.Parallel()

		// Setup test environment with built-in modules and superadmin module
		// Built-in modules are required because superadmin module depends on UserService from core module
		env := itf.Setup(t, itf.WithModules(append(modules.BuiltInModules, superadmin.NewModule(nil))...))

		// Verify module is registered
		require.NotNil(t, env, "test environment should be initialized")
		require.NotNil(t, env.App, "application should be initialized")
	})

	t.Run("ModuleName", func(t *testing.T) {
		t.Parallel()

		module := superadmin.NewModule(nil)
		assert.Equal(t, "superadmin", module.Name())
	})

	t.Run("ModuleWithOptions", func(t *testing.T) {
		t.Parallel()

		opts := &superadmin.ModuleOptions{}
		module := superadmin.NewModule(opts)

		require.NotNil(t, module)
		assert.Equal(t, "superadmin", module.Name())
	})

	t.Run("ModuleWithNilOptions", func(t *testing.T) {
		t.Parallel()

		module := superadmin.NewModule(nil)

		require.NotNil(t, module)
		assert.Equal(t, "superadmin", module.Name())
	})
}

func TestNavItems(t *testing.T) {
	t.Parallel()

	t.Run("NavItemsExist", func(t *testing.T) {
		t.Parallel()

		assert.NotEmpty(t, superadmin.NavItems, "navigation items should be defined")
		assert.Len(t, superadmin.NavItems, 2, "should have 2 navigation items")
	})

	t.Run("DashboardLink", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, "SuperAdmin.NavigationLinks.Dashboard", superadmin.DashboardLink.Name)
		assert.Equal(t, "/", superadmin.DashboardLink.Href)
		assert.NotNil(t, superadmin.DashboardLink.Icon)
	})

	t.Run("TenantsLink", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, "SuperAdmin.NavigationLinks.Tenants", superadmin.TenantsLink.Name)
		assert.Equal(t, "/superadmin/tenants", superadmin.TenantsLink.Href)
		assert.NotNil(t, superadmin.TenantsLink.Icon)
	})
}
