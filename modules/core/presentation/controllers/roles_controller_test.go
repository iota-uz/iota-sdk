package controllers_test

import (
	"fmt"
	"testing"

	"github.com/iota-uz/iota-sdk/modules"
	"github.com/iota-uz/iota-sdk/modules/core/permissions"
	"github.com/iota-uz/iota-sdk/modules/core/presentation/controllers"
	"github.com/iota-uz/iota-sdk/pkg/itf"
	"github.com/iota-uz/iota-sdk/pkg/rbac"
)

// createTestPermissionSchema creates an empty permission schema for testing
// to avoid translation dependencies in controller tests. The permission UI
// won't render when the schema has no sets, which is fine for controller tests
// that focus on HTTP behavior, validation, and authorization.
func createTestPermissionSchema() *rbac.PermissionSchema {
	return &rbac.PermissionSchema{
		Sets: []rbac.PermissionSet{},
	}
}

func TestRolesController_BasicRoutes(t *testing.T) {
	// Test that basic role routes work with proper permissions
	t.Parallel()

	suite := itf.NewSuiteBuilder(t).
		WithModules(modules.BuiltInModules...).
		AsUser(permissions.RoleCreate, permissions.RoleRead,
			permissions.RoleUpdate, permissions.RoleDelete).
		Build()

	controller := controllers.NewRolesController(suite.Env().App, &controllers.RolesControllerOptions{
		BasePath:         "/roles",
		PermissionSchema: createTestPermissionSchema(),
	})
	suite.Register(controller)

	t.Run("List_Roles", func(t *testing.T) {
		suite.GET("/roles").Assert(t).
			ExpectStatus(200).
			ExpectBodyContains("roles-table-body"). // Verify table is rendered
			ExpectBodyContains("new-role-btn")      // Verify new button is present
	})

	t.Run("New_Role_Form", func(t *testing.T) {
		suite.GET("/roles/new").Assert(t).
			ExpectStatus(200).
			ExpectBodyContains("role-name-input").        // Verify name input
			ExpectBodyContains("role-description-input"). // Verify description input
			ExpectBodyContains("save-role-btn")           // Verify save button
	})
}

func TestRolesController_Validation(t *testing.T) {
	// Test invalid inputs for role operations
	t.Parallel()

	suite := itf.NewSuiteBuilder(t).
		WithModules(modules.BuiltInModules...).
		AsUser(permissions.RoleCreate, permissions.RoleRead,
			permissions.RoleUpdate, permissions.RoleDelete).
		Build()

	controller := controllers.NewRolesController(suite.Env().App, &controllers.RolesControllerOptions{
		BasePath:         "/roles",
		PermissionSchema: createTestPermissionSchema(),
	})
	suite.Register(controller)

	cases := itf.Cases(
		itf.GET("/roles/0").
			Named("Zero_ID_Edit").
			ExpectStatus(500), // Invalid ID

		itf.GET("/roles/-1").
			Named("Negative_ID_Edit").
			ExpectStatus(404), // Route pattern doesn't match

		itf.GET("/roles/abc").
			Named("Non_Numeric_ID_Edit").
			ExpectStatus(404), // Route pattern doesn't match

		itf.GET("/roles/999999999").
			Named("Non_Existent_ID_Edit").
			ExpectStatus(500), // Role not found

		itf.DELETE("/roles/0").
			Named("Zero_ID_Delete").
			ExpectStatus(500),

		itf.DELETE("/roles/999999999").
			Named("Non_Existent_ID_Delete").
			ExpectStatus(500),
	)

	suite.RunCases(cases)
}

func TestRolesController_Delete_EdgeCases(t *testing.T) {
	t.Parallel()

	suite := itf.NewSuiteBuilder(t).
		WithModules(modules.BuiltInModules...).
		AsUser(permissions.RoleDelete, permissions.RoleRead).
		Build()

	controller := controllers.NewRolesController(suite.Env().App, &controllers.RolesControllerOptions{
		BasePath:         "/roles",
		PermissionSchema: createTestPermissionSchema(),
	})
	suite.Register(controller)

	cases := itf.Cases(
		itf.DELETE("/roles/0").
			Named("Zero_ID").
			ExpectStatus(500), // Role ID 0 is invalid

		itf.DELETE("/roles/-1").
			Named("Negative_ID").
			ExpectStatus(404), // Route pattern doesn't match negative numbers

		itf.DELETE("/roles/abc").
			Named("Non_Numeric_ID").
			ExpectStatus(404), // Route pattern doesn't match non-numeric values

		itf.DELETE("/roles/999999999").
			Named("Large_ID").
			ExpectStatus(500), // Large ID should still reach controller but role not found
	)

	suite.RunCases(cases)
}

func TestRolesController_Create_ValidationErrors(t *testing.T) {
	// Test that create validates required fields
	t.Parallel()

	suite := itf.NewSuiteBuilder(t).
		WithModules(modules.BuiltInModules...).
		AsUser(permissions.RoleCreate, permissions.RoleRead).
		Build()

	controller := controllers.NewRolesController(suite.Env().App, &controllers.RolesControllerOptions{
		BasePath:         "/roles",
		PermissionSchema: createTestPermissionSchema(),
	})
	suite.Register(controller)

	// POST without name should return validation error (200 with form re-rendered)
	request := suite.POST("/roles").
		FormFields(map[string]interface{}{
			"Description": "Test description",
			// Name is missing - should trigger validation error
		})

	// The controller returns 200 with the form containing validation errors
	// (This is the HTMX pattern - re-render form with errors instead of redirect)
	// Verify the form is re-rendered with the name input still present
	request.Assert(t).
		ExpectStatus(200).
		ExpectBodyContains("role-name-input"). // Form should be re-rendered
		ExpectBodyContains("save-role-btn")    // Save button should be present
}

func TestRolesController_List_Search(t *testing.T) {
	// Test that list endpoint handles search parameter
	t.Parallel()

	suite := itf.NewSuiteBuilder(t).
		WithModules(modules.BuiltInModules...).
		AsUser(permissions.RoleRead).
		Build()

	controller := controllers.NewRolesController(suite.Env().App, &controllers.RolesControllerOptions{
		BasePath:         "/roles",
		PermissionSchema: createTestPermissionSchema(),
	})
	suite.Register(controller)

	// List with search parameter should work and return table body content
	response := suite.GET("/roles?name=admin")
	response.Assert(t).
		ExpectStatus(200).
		ExpectBodyContains("roles-table-body") // Table should be rendered

	// List without search parameter should also work
	response = suite.GET("/roles")
	response.Assert(t).
		ExpectStatus(200).
		ExpectBodyContains("roles-table-body").
		ExpectBodyContains("new-role-btn") // New button should be visible
}

func TestRolesController_Update_NonExistent(t *testing.T) {
	// Test update on non-existent role
	t.Parallel()

	suite := itf.NewSuiteBuilder(t).
		WithModules(modules.BuiltInModules...).
		AsUser(permissions.RoleUpdate, permissions.RoleRead).
		Build()

	controller := controllers.NewRolesController(suite.Env().App, &controllers.RolesControllerOptions{
		BasePath:         "/roles",
		PermissionSchema: createTestPermissionSchema(),
	})
	suite.Register(controller)

	// Attempt to update non-existent role
	response := suite.POST(fmt.Sprintf("/roles/%d", 999999)).
		FormFields(map[string]interface{}{
			"Name":        "Updated Name",
			"Description": "Updated Description",
		})

	// Should return error (role not found)
	response.Assert(t).ExpectStatus(500)
}
