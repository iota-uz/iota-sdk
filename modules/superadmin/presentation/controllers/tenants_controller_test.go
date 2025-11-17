package controllers_test

import (
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/role"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/internet"
	coreservices "github.com/iota-uz/iota-sdk/modules/core/services"
	"github.com/iota-uz/iota-sdk/modules/superadmin"
	"github.com/iota-uz/iota-sdk/modules/superadmin/presentation/controllers"
	"github.com/iota-uz/iota-sdk/pkg/defaults"
	"github.com/iota-uz/iota-sdk/pkg/itf"
	"github.com/stretchr/testify/require"
)

// createSuperAdminUserForTenants creates a test user with TypeSuperAdmin for tenant tests
func createSuperAdminUserForTenants() user.User {
	email, _ := internet.NewEmail("superadmin@tenants-test.com")

	// Create a role with all permissions needed for superadmin operations
	adminRole := role.New(
		"superadmin",
		role.WithID(1),
		role.WithPermissions(defaults.AllPermissions()),
	)

	return user.New(
		"Super",
		"Admin",
		email,
		user.UILanguageEN,
		user.WithID(1),
		user.WithType(user.TypeSuperAdmin),
		user.WithTenantID(uuid.New()),
		user.WithRoles([]role.Role{adminRole}),
	)
}

// createRegularUserForTenants creates a test user with TypeUser (non-superadmin) for tenant tests
func createRegularUserForTenants() user.User {
	email, _ := internet.NewEmail("user@tenants-test.com")
	return user.New(
		"Regular",
		"User",
		email,
		user.UILanguageEN,
		user.WithID(2),
		user.WithType(user.TypeUser),
		user.WithTenantID(uuid.New()),
	)
}

func TestTenantsController_Index(t *testing.T) {
	t.Parallel()

	suite := itf.NewSuiteBuilder(t).
		WithModules(append(modules.BuiltInModules, superadmin.NewModule(nil))...).
		WithUser(createSuperAdminUserForTenants()).
		Build()

	userService := itf.GetService[coreservices.UserService](suite.Env())
	controller := controllers.NewTenantsController(suite.Env().App, userService)
	suite.Register(controller)

	// Test GET /superadmin/tenants - should render template properly
	suite.GET("/superadmin/tenants").
		Assert(t).
		ExpectOK()
}

func TestTenantsController_Index_HTMX(t *testing.T) {
	t.Parallel()

	suite := itf.NewSuiteBuilder(t).
		WithModules(append(modules.BuiltInModules, superadmin.NewModule(nil))...).
		WithUser(createSuperAdminUserForTenants()).
		Build()

	userService := itf.GetService[coreservices.UserService](suite.Env())
	controller := controllers.NewTenantsController(suite.Env().App, userService)
	suite.Register(controller)

	// Test GET /superadmin/tenants with HTMX - should render table rows
	suite.GET("/superadmin/tenants").
		HTMX().
		Assert(t).
		ExpectOK()
}

func TestTenantsController_Index_WithPagination(t *testing.T) {
	t.Parallel()

	suite := itf.NewSuiteBuilder(t).
		WithModules(append(modules.BuiltInModules, superadmin.NewModule(nil))...).
		WithUser(createSuperAdminUserForTenants()).
		Build()

	userService := itf.GetService[coreservices.UserService](suite.Env())
	controller := controllers.NewTenantsController(suite.Env().App, userService)
	suite.Register(controller)

	cases := itf.Cases(
		itf.GET("/superadmin/tenants").
			Named("First_Page").
			WithQuery(map[string]string{
				"limit":  "10",
				"offset": "0",
			}).
			HTMX().
			ExpectOK(),

		itf.GET("/superadmin/tenants").
			Named("Second_Page").
			WithQuery(map[string]string{
				"limit":  "10",
				"offset": "10",
			}).
			HTMX().
			ExpectOK(),

		itf.GET("/superadmin/tenants").
			Named("Large_Limit").
			WithQuery(map[string]string{
				"limit": "100",
			}).
			HTMX().
			ExpectOK(),

		itf.GET("/superadmin/tenants").
			Named("Zero_Offset").
			WithQuery(map[string]string{
				"offset": "0",
			}).
			HTMX().
			ExpectOK(),
	)

	suite.RunCases(cases)
}

func TestTenantsController_Index_WithSearch(t *testing.T) {
	t.Parallel()

	suite := itf.NewSuiteBuilder(t).
		WithModules(append(modules.BuiltInModules, superadmin.NewModule(nil))...).
		WithUser(createSuperAdminUserForTenants()).
		Build()

	userService := itf.GetService[coreservices.UserService](suite.Env())
	controller := controllers.NewTenantsController(suite.Env().App, userService)
	suite.Register(controller)

	cases := itf.Cases(
		itf.GET("/superadmin/tenants").
			Named("Search_By_Name").
			WithQuery(map[string]string{
				"search": "test",
			}).
			HTMX().
			ExpectOK(),

		itf.GET("/superadmin/tenants").
			Named("Empty_Search").
			WithQuery(map[string]string{
				"search": "",
			}).
			HTMX().
			ExpectOK(),

		itf.GET("/superadmin/tenants").
			Named("Special_Characters_Search").
			WithQuery(map[string]string{
				"search": "test%domain",
			}).
			HTMX().
			ExpectOK(),
	)

	suite.RunCases(cases)
}

func TestTenantsController_Export(t *testing.T) {
	t.Parallel()

	suite := itf.NewSuiteBuilder(t).
		WithModules(append(modules.BuiltInModules, superadmin.NewModule(nil))...).
		WithUser(createSuperAdminUserForTenants()).
		Build()

	userService := itf.GetService[coreservices.UserService](suite.Env())
	controller := controllers.NewTenantsController(suite.Env().App, userService)
	suite.Register(controller)

	// Test POST /superadmin/tenants/export
	// Note: This will create an actual file export
	// Should redirect to download URL (status 303 See Other)
	suite.POST("/superadmin/tenants/export").
		Assert(t).
		ExpectStatus(303) // Should redirect with See Other
}

func TestTenantsController_Permissions(t *testing.T) {
	t.Parallel()

	// Test with different user types and permission levels
	testCases := []struct {
		name           string
		setupSuite     func(*testing.T) *itf.Suite
		expectedStatus int
		description    string
	}{
		{
			name: "SuperAdmin_Access_Allowed",
			setupSuite: func(t *testing.T) *itf.Suite {
				t.Helper()
				return itf.NewSuiteBuilder(t).
					WithModules(append(modules.BuiltInModules, superadmin.NewModule(nil))...).
					WithUser(createSuperAdminUserForTenants()).
					Build()
			},
			expectedStatus: 200,
			description:    "Superadmin users should have full access to tenants",
		},
		{
			name: "Regular_User_Blocked",
			setupSuite: func(t *testing.T) *itf.Suite {
				t.Helper()
				return itf.NewSuiteBuilder(t).
					WithModules(append(modules.BuiltInModules, superadmin.NewModule(nil))...).
					WithUser(createRegularUserForTenants()).
					Build()
			},
			expectedStatus: 403,
			description:    "Regular users should be blocked with 403 Forbidden",
		},
		{
			name: "Anonymous_User_Redirect",
			setupSuite: func(t *testing.T) *itf.Suite {
				t.Helper()
				return itf.NewSuiteBuilder(t).
					WithModules(append(modules.BuiltInModules, superadmin.NewModule(nil))...).
					AsAnonymous().
					Build()
			},
			expectedStatus: 302,
			description:    "Anonymous users should be redirected to login",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			suite := tc.setupSuite(t)
			userService := itf.GetService[coreservices.UserService](suite.Env())
			controller := controllers.NewTenantsController(suite.Env().App, userService)
			suite.Register(controller)

			suite.GET("/superadmin/tenants").
				Assert(t).
				ExpectStatus(tc.expectedStatus)
		})
	}
}

// TestTenantsController_SuperAdminOnly verifies only superadmin users can access all endpoints
func TestTenantsController_SuperAdminOnly(t *testing.T) {
	t.Parallel()

	t.Run("Tenants_Index_SuperAdmin_OK", func(t *testing.T) {
		suite := itf.NewSuiteBuilder(t).
			WithModules(append(modules.BuiltInModules, superadmin.NewModule(nil))...).
			WithUser(createSuperAdminUserForTenants()).
			Build()

		userService := itf.GetService[coreservices.UserService](suite.Env())
		controller := controllers.NewTenantsController(suite.Env().App, userService)
		suite.Register(controller)

		suite.GET("/superadmin/tenants").
			Assert(t).
			ExpectOK()
	})

	t.Run("Tenants_Index_RegularUser_Forbidden", func(t *testing.T) {
		suite := itf.NewSuiteBuilder(t).
			WithModules(append(modules.BuiltInModules, superadmin.NewModule(nil))...).
			WithUser(createRegularUserForTenants()).
			Build()

		userService := itf.GetService[coreservices.UserService](suite.Env())
		controller := controllers.NewTenantsController(suite.Env().App, userService)
		suite.Register(controller)

		suite.GET("/superadmin/tenants").
			Assert(t).
			ExpectForbidden()
	})

	t.Run("Tenants_Export_SuperAdmin_OK", func(t *testing.T) {
		suite := itf.NewSuiteBuilder(t).
			WithModules(append(modules.BuiltInModules, superadmin.NewModule(nil))...).
			WithUser(createSuperAdminUserForTenants()).
			Build()

		userService := itf.GetService[coreservices.UserService](suite.Env())
		controller := controllers.NewTenantsController(suite.Env().App, userService)
		suite.Register(controller)

		// Export endpoint redirects (303)
		suite.POST("/superadmin/tenants/export").
			Assert(t).
			ExpectStatus(303)
	})

	t.Run("Tenants_Export_RegularUser_Forbidden", func(t *testing.T) {
		suite := itf.NewSuiteBuilder(t).
			WithModules(append(modules.BuiltInModules, superadmin.NewModule(nil))...).
			WithUser(createRegularUserForTenants()).
			Build()

		userService := itf.GetService[coreservices.UserService](suite.Env())
		controller := controllers.NewTenantsController(suite.Env().App, userService)
		suite.Register(controller)

		suite.POST("/superadmin/tenants/export").
			Assert(t).
			ExpectForbidden()
	})

	t.Run("Tenants_Users_SuperAdmin_OK", func(t *testing.T) {
		suite := itf.NewSuiteBuilder(t).
			WithModules(append(modules.BuiltInModules, superadmin.NewModule(nil))...).
			WithUser(createSuperAdminUserForTenants()).
			Build()

		userService := itf.GetService[coreservices.UserService](suite.Env())
		controller := controllers.NewTenantsController(suite.Env().App, userService)
		suite.Register(controller)

		// Create test tenant within this test's suite/database
		tenant, err := itf.CreateTestTenant(suite.Env().Ctx, suite.Env().Pool)
		require.NoError(t, err)

		suite.GET(fmt.Sprintf("/superadmin/tenants/%s/users", tenant.ID.String())).
			Assert(t).
			ExpectOK()
	})

	t.Run("Tenants_Users_RegularUser_Forbidden", func(t *testing.T) {
		suite := itf.NewSuiteBuilder(t).
			WithModules(append(modules.BuiltInModules, superadmin.NewModule(nil))...).
			WithUser(createRegularUserForTenants()).
			Build()

		userService := itf.GetService[coreservices.UserService](suite.Env())
		controller := controllers.NewTenantsController(suite.Env().App, userService)
		suite.Register(controller)

		// Create test tenant within this test's suite/database
		tenant, err := itf.CreateTestTenant(suite.Env().Ctx, suite.Env().Pool)
		require.NoError(t, err)

		suite.GET(fmt.Sprintf("/superadmin/tenants/%s/users", tenant.ID.String())).
			Assert(t).
			ExpectForbidden()
	})
}

func TestTenantsController_Routes(t *testing.T) {
	t.Parallel()

	suite := itf.NewSuiteBuilder(t).
		WithModules(append(modules.BuiltInModules, superadmin.NewModule(nil))...).
		WithUser(createSuperAdminUserForTenants()).
		Build()

	userService := itf.GetService[coreservices.UserService](suite.Env())
	controller := controllers.NewTenantsController(suite.Env().App, userService)
	suite.Register(controller)

	cases := itf.Cases(
		itf.GET("/superadmin/tenants").
			Named("Tenants_Index").
			ExpectOK(),

		itf.PUT("/superadmin/tenants").
			Named("PUT_Not_Allowed").
			ExpectStatus(404), // Router returns 404 for unsupported methods

		itf.DELETE("/superadmin/tenants").
			Named("DELETE_Not_Allowed").
			ExpectStatus(404), // Router returns 404 for unsupported methods
	)

	suite.RunCases(cases)

	// Test export separately since it should redirect
	suite.POST("/superadmin/tenants/export").
		Assert(t).
		ExpectStatus(303) // Should redirect with See Other
}

func TestTenantsController_EdgeCases(t *testing.T) {
	t.Parallel()

	suite := itf.NewSuiteBuilder(t).
		WithModules(append(modules.BuiltInModules, superadmin.NewModule(nil))...).
		WithUser(createSuperAdminUserForTenants()).
		Build()

	userService := itf.GetService[coreservices.UserService](suite.Env())
	controller := controllers.NewTenantsController(suite.Env().App, userService)
	suite.Register(controller)

	cases := itf.Cases(
		itf.GET("/superadmin/tenants").
			Named("Negative_Offset").
			WithQuery(map[string]string{
				"offset": "-1",
			}).
			HTMX().
			ExpectOK(), // Should handle gracefully

		itf.GET("/superadmin/tenants").
			Named("Zero_Limit").
			WithQuery(map[string]string{
				"limit": "0",
			}).
			HTMX().
			ExpectOK(), // Should use default limit

		itf.GET("/superadmin/tenants").
			Named("Very_Large_Limit").
			WithQuery(map[string]string{
				"limit": "999999",
			}).
			HTMX().
			ExpectOK(),

		itf.GET("/superadmin/tenants").
			Named("Invalid_Pagination_Values").
			WithQuery(map[string]string{
				"limit":  "abc",
				"offset": "xyz",
			}).
			HTMX().
			ExpectOK(), // Should use defaults
	)

	suite.RunCases(cases)
}

func TestTenantsController_HTMX(t *testing.T) {
	t.Parallel()

	suite := itf.NewSuiteBuilder(t).
		WithModules(append(modules.BuiltInModules, superadmin.NewModule(nil))...).
		WithUser(createSuperAdminUserForTenants()).
		Build()

	userService := itf.GetService[coreservices.UserService](suite.Env())
	controller := controllers.NewTenantsController(suite.Env().App, userService)
	suite.Register(controller)

	// Test HTMX request
	suite.GET("/superadmin/tenants").
		HTMX().
		Assert(t).
		ExpectOK()
}

func TestTenantsController_Index_WithDateRange(t *testing.T) {
	t.Parallel()

	suite := itf.NewSuiteBuilder(t).
		WithModules(append(modules.BuiltInModules, superadmin.NewModule(nil))...).
		WithUser(createSuperAdminUserForTenants()).
		Build()

	userService := itf.GetService[coreservices.UserService](suite.Env())
	controller := controllers.NewTenantsController(suite.Env().App, userService)
	suite.Register(controller)

	cases := itf.Cases(
		itf.GET("/superadmin/tenants").
			Named("Valid_Start_Date_Only").
			WithQuery(map[string]string{
				"start_date": "2024-01-01T00:00:00Z",
			}).
			HTMX().
			ExpectOK(),

		itf.GET("/superadmin/tenants").
			Named("Valid_End_Date_Only").
			WithQuery(map[string]string{
				"end_date": "2024-12-31T23:59:59Z",
			}).
			HTMX().
			ExpectOK(),

		itf.GET("/superadmin/tenants").
			Named("Valid_Date_Range").
			WithQuery(map[string]string{
				"start_date": "2024-01-01T00:00:00Z",
				"end_date":   "2024-12-31T23:59:59Z",
			}).
			HTMX().
			ExpectOK(),

		itf.GET("/superadmin/tenants").
			Named("Invalid_Start_Date_Format").
			WithQuery(map[string]string{
				"start_date": "invalid-date",
			}).
			HTMX().
			ExpectBadRequest(),

		itf.GET("/superadmin/tenants").
			Named("Invalid_End_Date_Format").
			WithQuery(map[string]string{
				"end_date": "not-a-date",
			}).
			HTMX().
			ExpectBadRequest(),

		itf.GET("/superadmin/tenants").
			Named("Mixed_Valid_Invalid_Dates").
			WithQuery(map[string]string{
				"start_date": "2024-01-01T00:00:00Z",
				"end_date":   "invalid",
			}).
			HTMX().
			ExpectBadRequest(),
	)

	suite.RunCases(cases)
}

func TestTenantsController_Index_SortAscending(t *testing.T) {
	t.Parallel()

	suite := itf.NewSuiteBuilder(t).
		WithModules(append(modules.BuiltInModules, superadmin.NewModule(nil))...).
		WithUser(createSuperAdminUserForTenants()).
		Build()

	userService := itf.GetService[coreservices.UserService](suite.Env())
	controller := controllers.NewTenantsController(suite.Env().App, userService)
	suite.Register(controller)

	cases := itf.Cases(
		itf.GET("/superadmin/tenants").
			Named("Sort_Created_At_Asc").
			WithQuery(map[string]string{
				"sortField": "created_at",
				"sortOrder": "asc",
			}).
			HTMX().
			ExpectOK(),

		itf.GET("/superadmin/tenants").
			Named("Sort_Name_Asc").
			WithQuery(map[string]string{
				"sortField": "name",
				"sortOrder": "asc",
			}).
			HTMX().
			ExpectOK(),

		itf.GET("/superadmin/tenants").
			Named("Sort_Domain_Asc").
			WithQuery(map[string]string{
				"sortField": "domain",
				"sortOrder": "asc",
			}).
			HTMX().
			ExpectOK(),

		itf.GET("/superadmin/tenants").
			Named("Sort_User_Count_Asc").
			WithQuery(map[string]string{
				"sortField": "user_count",
				"sortOrder": "asc",
			}).
			HTMX().
			ExpectOK(),
	)

	suite.RunCases(cases)
}

func TestTenantsController_Index_SortDescending(t *testing.T) {
	t.Parallel()

	suite := itf.NewSuiteBuilder(t).
		WithModules(append(modules.BuiltInModules, superadmin.NewModule(nil))...).
		WithUser(createSuperAdminUserForTenants()).
		Build()

	userService := itf.GetService[coreservices.UserService](suite.Env())
	controller := controllers.NewTenantsController(suite.Env().App, userService)
	suite.Register(controller)

	cases := itf.Cases(
		itf.GET("/superadmin/tenants").
			Named("Sort_Created_At_Desc").
			WithQuery(map[string]string{
				"sortField": "created_at",
				"sortOrder": "desc",
			}).
			HTMX().
			ExpectOK(),

		itf.GET("/superadmin/tenants").
			Named("Sort_Name_Desc").
			WithQuery(map[string]string{
				"sortField": "name",
				"sortOrder": "desc",
			}).
			HTMX().
			ExpectOK(),

		itf.GET("/superadmin/tenants").
			Named("Sort_Domain_Desc").
			WithQuery(map[string]string{
				"sortField": "domain",
				"sortOrder": "desc",
			}).
			HTMX().
			ExpectOK(),

		itf.GET("/superadmin/tenants").
			Named("Sort_User_Count_Desc").
			WithQuery(map[string]string{
				"sortField": "user_count",
				"sortOrder": "desc",
			}).
			HTMX().
			ExpectOK(),
	)

	suite.RunCases(cases)
}

func TestTenantsController_Index_DefaultSort(t *testing.T) {
	t.Parallel()

	suite := itf.NewSuiteBuilder(t).
		WithModules(append(modules.BuiltInModules, superadmin.NewModule(nil))...).
		WithUser(createSuperAdminUserForTenants()).
		Build()

	userService := itf.GetService[coreservices.UserService](suite.Env())
	controller := controllers.NewTenantsController(suite.Env().App, userService)
	suite.Register(controller)

	cases := itf.Cases(
		itf.GET("/superadmin/tenants").
			Named("No_Sort_Params").
			HTMX().
			ExpectOK(),

		itf.GET("/superadmin/tenants").
			Named("Empty_Sort_Field").
			WithQuery(map[string]string{
				"sortField": "",
				"sortOrder": "asc",
			}).
			HTMX().
			ExpectOK(),

		itf.GET("/superadmin/tenants").
			Named("Empty_Sort_Order").
			WithQuery(map[string]string{
				"sortField": "name",
				"sortOrder": "",
			}).
			HTMX().
			ExpectOK(),

		itf.GET("/superadmin/tenants").
			Named("Both_Empty").
			WithQuery(map[string]string{
				"sortField": "",
				"sortOrder": "",
			}).
			HTMX().
			ExpectOK(),
	)

	suite.RunCases(cases)
}

func TestTenantsController_Index_InvalidSortField(t *testing.T) {
	t.Parallel()

	suite := itf.NewSuiteBuilder(t).
		WithModules(append(modules.BuiltInModules, superadmin.NewModule(nil))...).
		WithUser(createSuperAdminUserForTenants()).
		Build()

	userService := itf.GetService[coreservices.UserService](suite.Env())
	controller := controllers.NewTenantsController(suite.Env().App, userService)
	suite.Register(controller)

	cases := itf.Cases(
		itf.GET("/superadmin/tenants").
			Named("Invalid_Field_Name").
			WithQuery(map[string]string{
				"sortField": "invalid_field",
				"sortOrder": "asc",
			}).
			HTMX().
			ExpectOK(),

		itf.GET("/superadmin/tenants").
			Named("SQL_Injection_Attempt").
			WithQuery(map[string]string{
				"sortField": "name; DROP TABLE tenants;",
				"sortOrder": "asc",
			}).
			HTMX().
			ExpectOK(),

		itf.GET("/superadmin/tenants").
			Named("Invalid_Sort_Order").
			WithQuery(map[string]string{
				"sortField": "created_at",
				"sortOrder": "invalid",
			}).
			HTMX().
			ExpectOK(),

		itf.GET("/superadmin/tenants").
			Named("Case_Sensitive_Sort_Order").
			WithQuery(map[string]string{
				"sortField": "name",
				"sortOrder": "ASC",
			}).
			HTMX().
			ExpectOK(),
	)

	suite.RunCases(cases)
}

func TestTenantsController_Index_SortWithDateFilter(t *testing.T) {
	t.Parallel()

	suite := itf.NewSuiteBuilder(t).
		WithModules(append(modules.BuiltInModules, superadmin.NewModule(nil))...).
		WithUser(createSuperAdminUserForTenants()).
		Build()

	userService := itf.GetService[coreservices.UserService](suite.Env())
	controller := controllers.NewTenantsController(suite.Env().App, userService)
	suite.Register(controller)

	cases := itf.Cases(
		itf.GET("/superadmin/tenants").
			Named("Sort_With_Date_Range_Asc").
			WithQuery(map[string]string{
				"start_date": "2024-01-01T00:00:00Z",
				"end_date":   "2024-12-31T23:59:59Z",
				"sortField":  "name",
				"sortOrder":  "asc",
			}).
			HTMX().
			ExpectOK(),

		itf.GET("/superadmin/tenants").
			Named("Sort_With_Date_Range_Desc").
			WithQuery(map[string]string{
				"start_date": "2024-01-01T00:00:00Z",
				"end_date":   "2024-12-31T23:59:59Z",
				"sortField":  "created_at",
				"sortOrder":  "desc",
			}).
			HTMX().
			ExpectOK(),

		itf.GET("/superadmin/tenants").
			Named("Sort_With_Start_Date_Only").
			WithQuery(map[string]string{
				"start_date": "2024-01-01T00:00:00Z",
				"sortField":  "user_count",
				"sortOrder":  "desc",
			}).
			HTMX().
			ExpectOK(),

		itf.GET("/superadmin/tenants").
			Named("Sort_Multiple_Fields").
			WithQuery(map[string]string{
				"start_date": "2024-01-01T00:00:00Z",
				"end_date":   "2024-12-31T23:59:59Z",
				"sortField":  "domain",
				"sortOrder":  "asc",
			}).
			HTMX().
			ExpectOK(),
	)

	suite.RunCases(cases)
}

func TestTenantsController_Index_SortWithSearch(t *testing.T) {
	t.Parallel()

	suite := itf.NewSuiteBuilder(t).
		WithModules(append(modules.BuiltInModules, superadmin.NewModule(nil))...).
		WithUser(createSuperAdminUserForTenants()).
		Build()

	userService := itf.GetService[coreservices.UserService](suite.Env())
	controller := controllers.NewTenantsController(suite.Env().App, userService)
	suite.Register(controller)

	cases := itf.Cases(
		itf.GET("/superadmin/tenants").
			Named("Sort_With_Search_Asc").
			WithQuery(map[string]string{
				"search":    "test",
				"sortField": "name",
				"sortOrder": "asc",
			}).
			HTMX().
			ExpectOK(),

		itf.GET("/superadmin/tenants").
			Named("Sort_With_Search_Desc").
			WithQuery(map[string]string{
				"search":    "test",
				"sortField": "created_at",
				"sortOrder": "desc",
			}).
			HTMX().
			ExpectOK(),

		itf.GET("/superadmin/tenants").
			Named("Sort_User_Count_With_Search").
			WithQuery(map[string]string{
				"search":    "domain",
				"sortField": "user_count",
				"sortOrder": "desc",
			}).
			HTMX().
			ExpectOK(),

		itf.GET("/superadmin/tenants").
			Named("Empty_Search_With_Sort").
			WithQuery(map[string]string{
				"search":    "",
				"sortField": "name",
				"sortOrder": "asc",
			}).
			HTMX().
			ExpectOK(),
	)

	suite.RunCases(cases)
}

func TestTenantsController_Index_SortWithPagination(t *testing.T) {
	t.Parallel()

	suite := itf.NewSuiteBuilder(t).
		WithModules(append(modules.BuiltInModules, superadmin.NewModule(nil))...).
		WithUser(createSuperAdminUserForTenants()).
		Build()

	userService := itf.GetService[coreservices.UserService](suite.Env())
	controller := controllers.NewTenantsController(suite.Env().App, userService)
	suite.Register(controller)

	cases := itf.Cases(
		itf.GET("/superadmin/tenants").
			Named("Sort_First_Page").
			WithQuery(map[string]string{
				"sortField": "name",
				"sortOrder": "asc",
				"limit":     "10",
				"offset":    "0",
			}).
			HTMX().
			ExpectOK(),

		itf.GET("/superadmin/tenants").
			Named("Sort_Second_Page").
			WithQuery(map[string]string{
				"sortField": "name",
				"sortOrder": "asc",
				"limit":     "10",
				"offset":    "10",
			}).
			HTMX().
			ExpectOK(),

		itf.GET("/superadmin/tenants").
			Named("Sort_Large_Limit").
			WithQuery(map[string]string{
				"sortField": "created_at",
				"sortOrder": "desc",
				"limit":     "100",
			}).
			HTMX().
			ExpectOK(),
	)

	suite.RunCases(cases)
}

func TestTenantsController_Index_DateRangeWithPagination(t *testing.T) {
	t.Parallel()

	suite := itf.NewSuiteBuilder(t).
		WithModules(append(modules.BuiltInModules, superadmin.NewModule(nil))...).
		WithUser(createSuperAdminUserForTenants()).
		Build()

	userService := itf.GetService[coreservices.UserService](suite.Env())
	controller := controllers.NewTenantsController(suite.Env().App, userService)
	suite.Register(controller)

	cases := itf.Cases(
		itf.GET("/superadmin/tenants").
			Named("Date_Range_First_Page").
			WithQuery(map[string]string{
				"start_date": "2024-01-01T00:00:00Z",
				"end_date":   "2024-12-31T23:59:59Z",
				"limit":      "10",
				"offset":     "0",
			}).
			HTMX().
			ExpectOK(),

		itf.GET("/superadmin/tenants").
			Named("Date_Range_Second_Page").
			WithQuery(map[string]string{
				"start_date": "2024-01-01T00:00:00Z",
				"end_date":   "2024-12-31T23:59:59Z",
				"limit":      "10",
				"offset":     "10",
			}).
			HTMX().
			ExpectOK(),

		itf.GET("/superadmin/tenants").
			Named("Date_Range_Large_Limit").
			WithQuery(map[string]string{
				"start_date": "2024-01-01T00:00:00Z",
				"end_date":   "2024-12-31T23:59:59Z",
				"limit":      "100",
			}).
			HTMX().
			ExpectOK(),
	)

	suite.RunCases(cases)
}

func TestTenantsController_TenantUsers(t *testing.T) {
	t.Parallel()

	suite := itf.NewSuiteBuilder(t).
		WithModules(append(modules.BuiltInModules, superadmin.NewModule(nil))...).
		WithUser(createSuperAdminUserForTenants()).
		Build()

	userService := itf.GetService[coreservices.UserService](suite.Env())
	controller := controllers.NewTenantsController(suite.Env().App, userService)
	suite.Register(controller)

	t.Run("Happy_Path_Valid_Tenant", func(t *testing.T) {
		// Create test tenant
		tenant, err := itf.CreateTestTenant(suite.Env().Ctx, suite.Env().Pool)
		require.NoError(t, err)

		// Get users for tenant
		suite.GET(fmt.Sprintf("/superadmin/tenants/%s/users", tenant.ID.String())).
			Assert(t).
			ExpectOK()
	})

	t.Run("Invalid_Tenant_ID", func(t *testing.T) {
		suite.GET("/superadmin/tenants/invalid-uuid/users").
			Assert(t).
			ExpectBadRequest()
	})

	t.Run("NonExistent_Tenant", func(t *testing.T) {
		nonExistentID := uuid.New()
		suite.GET(fmt.Sprintf("/superadmin/tenants/%s/users", nonExistentID.String())).
			Assert(t).
			ExpectNotFound() // Should return 404 for non-existent tenant
	})
}

func TestTenantsController_TenantUsers_HTMX(t *testing.T) {
	t.Parallel()

	suite := itf.NewSuiteBuilder(t).
		WithModules(append(modules.BuiltInModules, superadmin.NewModule(nil))...).
		WithUser(createSuperAdminUserForTenants()).
		Build()

	userService := itf.GetService[coreservices.UserService](suite.Env())
	controller := controllers.NewTenantsController(suite.Env().App, userService)
	suite.Register(controller)

	tenant, err := itf.CreateTestTenant(suite.Env().Ctx, suite.Env().Pool)
	require.NoError(t, err)

	// Test HTMX request
	suite.GET(fmt.Sprintf("/superadmin/tenants/%s/users", tenant.ID.String())).
		HTMX().
		Assert(t).
		ExpectOK()
}

func TestTenantsController_TenantUsers_Pagination(t *testing.T) {
	t.Parallel()

	suite := itf.NewSuiteBuilder(t).
		WithModules(append(modules.BuiltInModules, superadmin.NewModule(nil))...).
		WithUser(createSuperAdminUserForTenants()).
		Build()

	userService := itf.GetService[coreservices.UserService](suite.Env())
	controller := controllers.NewTenantsController(suite.Env().App, userService)
	suite.Register(controller)

	tenant, err := itf.CreateTestTenant(suite.Env().Ctx, suite.Env().Pool)
	require.NoError(t, err)

	cases := itf.Cases(
		itf.GET(fmt.Sprintf("/superadmin/tenants/%s/users", tenant.ID.String())).
			Named("First_Page").
			WithQuery(map[string]string{
				"limit":  "10",
				"offset": "0",
			}).
			HTMX().
			ExpectOK(),

		itf.GET(fmt.Sprintf("/superadmin/tenants/%s/users", tenant.ID.String())).
			Named("Second_Page").
			WithQuery(map[string]string{
				"limit":  "10",
				"offset": "10",
			}).
			HTMX().
			ExpectOK(),

		itf.GET(fmt.Sprintf("/superadmin/tenants/%s/users", tenant.ID.String())).
			Named("Large_Limit").
			WithQuery(map[string]string{
				"limit": "100",
			}).
			HTMX().
			ExpectOK(),
	)

	suite.RunCases(cases)
}

func TestTenantsController_TenantUsers_Search(t *testing.T) {
	t.Parallel()

	suite := itf.NewSuiteBuilder(t).
		WithModules(append(modules.BuiltInModules, superadmin.NewModule(nil))...).
		WithUser(createSuperAdminUserForTenants()).
		Build()

	userService := itf.GetService[coreservices.UserService](suite.Env())
	controller := controllers.NewTenantsController(suite.Env().App, userService)
	suite.Register(controller)

	tenant, err := itf.CreateTestTenant(suite.Env().Ctx, suite.Env().Pool)
	require.NoError(t, err)

	cases := itf.Cases(
		itf.GET(fmt.Sprintf("/superadmin/tenants/%s/users", tenant.ID.String())).
			Named("Search_By_Name").
			WithQuery(map[string]string{
				"search": "test",
			}).
			HTMX().
			ExpectOK(),

		itf.GET(fmt.Sprintf("/superadmin/tenants/%s/users", tenant.ID.String())).
			Named("Empty_Search").
			WithQuery(map[string]string{
				"search": "",
			}).
			HTMX().
			ExpectOK(),

		itf.GET(fmt.Sprintf("/superadmin/tenants/%s/users", tenant.ID.String())).
			Named("Special_Characters").
			WithQuery(map[string]string{
				"search": "test%user",
			}).
			HTMX().
			ExpectOK(),
	)

	suite.RunCases(cases)
}

func TestTenantsController_TenantUsers_Sorting(t *testing.T) {
	t.Parallel()

	suite := itf.NewSuiteBuilder(t).
		WithModules(append(modules.BuiltInModules, superadmin.NewModule(nil))...).
		WithUser(createSuperAdminUserForTenants()).
		Build()

	userService := itf.GetService[coreservices.UserService](suite.Env())
	controller := controllers.NewTenantsController(suite.Env().App, userService)
	suite.Register(controller)

	tenant, err := itf.CreateTestTenant(suite.Env().Ctx, suite.Env().Pool)
	require.NoError(t, err)

	cases := itf.Cases(
		itf.GET(fmt.Sprintf("/superadmin/tenants/%s/users", tenant.ID.String())).
			Named("Sort_FirstName_Asc").
			WithQuery(map[string]string{
				"sortField": "first_name",
				"sortOrder": "asc",
			}).
			HTMX().
			ExpectOK(),

		itf.GET(fmt.Sprintf("/superadmin/tenants/%s/users", tenant.ID.String())).
			Named("Sort_Email_Desc").
			WithQuery(map[string]string{
				"sortField": "email",
				"sortOrder": "desc",
			}).
			HTMX().
			ExpectOK(),

		itf.GET(fmt.Sprintf("/superadmin/tenants/%s/users", tenant.ID.String())).
			Named("Sort_CreatedAt_Desc").
			WithQuery(map[string]string{
				"sortField": "created_at",
				"sortOrder": "desc",
			}).
			HTMX().
			ExpectOK(),
	)

	suite.RunCases(cases)
}

func TestTenantsController_Index_HTMXTargetHandling(t *testing.T) {
	t.Parallel()

	suite := itf.NewSuiteBuilder(t).
		WithModules(append(modules.BuiltInModules, superadmin.NewModule(nil))...).
		WithUser(createSuperAdminUserForTenants()).
		Build()

	userService := itf.GetService[coreservices.UserService](suite.Env())
	controller := controllers.NewTenantsController(suite.Env().App, userService)
	suite.Register(controller)

	t.Run("Sorting_Returns_Full_Table", func(t *testing.T) {
		// Sorting request - should return full table with headers
		resp := suite.GET("/superadmin/tenants").
			WithQuery(map[string]string{"sortField": "created_at", "sortOrder": "desc"}).
			Header("HX-Request", "true").
			Header("HX-Target", "sortable-table-container").
			Assert(t)

		resp.ExpectOK()
		htmlAssert := resp.ExpectHTML()
		htmlAssert.ExpectElement("//div[@id='sortable-table-container']")
		htmlAssert.ExpectElement("//table")
		htmlAssert.ExpectElement("//thead")
	})

	t.Run("Search_Returns_Only_Rows", func(t *testing.T) {
		// Search/filter request - should return only rows (empty search to show all data)
		resp := suite.GET("/superadmin/tenants").
			WithQuery(map[string]string{"search": ""}).
			Header("HX-Request", "true").
			Header("HX-Target", "table-body").
			Assert(t)

		resp.ExpectOK()
		// When targeting table-body, should NOT have table container or table tags
		// Only rows should be returned (or empty state message)
		htmlAssert := resp.ExpectHTML()
		htmlAssert.ExpectNoElement("//div[@id='sortable-table-container']")
		htmlAssert.ExpectNoElement("//table")
	})

	t.Run("Full_Page_Request", func(t *testing.T) {
		// Non-HTMX request - should return full page
		resp := suite.GET("/superadmin/tenants").Assert(t)

		resp.ExpectOK()
		htmlAssert := resp.ExpectHTML()
		htmlAssert.ExpectElement("//div[@id='sortable-table-container']")
		htmlAssert.ExpectElement("//h1")
	})
}

func TestTenantsController_TenantUsers_HTMXTargetHandling(t *testing.T) {
	t.Parallel()

	suite := itf.NewSuiteBuilder(t).
		WithModules(append(modules.BuiltInModules, superadmin.NewModule(nil))...).
		WithUser(createSuperAdminUserForTenants()).
		Build()

	userService := itf.GetService[coreservices.UserService](suite.Env())
	controller := controllers.NewTenantsController(suite.Env().App, userService)
	suite.Register(controller)

	tenant, err := itf.CreateTestTenant(suite.Env().Ctx, suite.Env().Pool)
	require.NoError(t, err)

	t.Run("Sorting_Returns_Full_Users_Table", func(t *testing.T) {
		// Sorting request - should return full table OR empty state (not just rows)
		resp := suite.GET(fmt.Sprintf("/superadmin/tenants/%s/users", tenant.ID.String())).
			WithQuery(map[string]string{"sortField": "first_name", "sortOrder": "asc"}).
			Header("HX-Request", "true").
			Header("HX-Target", "sortable-table-container").
			Assert(t)

		resp.ExpectOK()
		// When targeting sortable-table-container, response should NOT be just rows
		// It should be either: (1) full table with container, OR (2) empty state message
		// In both cases, should NOT have tbody#table-body as direct child (that's for row-only responses)
		htmlAssert := resp.ExpectHTML()
		// Should NOT start with tbody (that would indicate rows-only response)
		htmlAssert.ExpectNoElement("/html/body/tbody[@id='table-body']")
	})

	t.Run("Search_Returns_Only_User_Rows", func(t *testing.T) {
		// Search/filter request - should return only rows (empty search to show all users)
		resp := suite.GET(fmt.Sprintf("/superadmin/tenants/%s/users", tenant.ID.String())).
			WithQuery(map[string]string{"search": ""}).
			Header("HX-Request", "true").
			Header("HX-Target", "table-body").
			Assert(t)

		resp.ExpectOK()
		// When targeting table-body, should NOT have table container or table tags
		// Only rows should be returned (or empty state message)
		htmlAssert := resp.ExpectHTML()
		htmlAssert.ExpectNoElement("//div[@id='sortable-table-container']")
		htmlAssert.ExpectNoElement("//table")
	})

	t.Run("Full_Page_Request", func(t *testing.T) {
		// Non-HTMX request - should return full page
		resp := suite.GET(fmt.Sprintf("/superadmin/tenants/%s/users", tenant.ID.String())).
			Assert(t)

		resp.ExpectOK()
		// Full page should have breadcrumbs/nav and tenant info card
		htmlAssert := resp.ExpectHTML()
		htmlAssert.ExpectElement("//nav")
		// Should have the page structure, not just table rows
		htmlAssert.ExpectNoElement("/html/body/tbody")
	})
}

// TestTenantsController_ResetUserPassword_Success verifies successful password reset
func TestTenantsController_ResetUserPassword_Success(t *testing.T) {
	t.Parallel()

	suite := itf.NewSuiteBuilder(t).
		WithModules(append(modules.BuiltInModules, superadmin.NewModule(nil))...).
		WithUser(createSuperAdminUserForTenants()).
		Build()

	userService := itf.GetService[coreservices.UserService](suite.Env())
	controller := controllers.NewTenantsController(suite.Env().App, userService)
	suite.Register(controller)

	ctx := suite.Env().Ctx

	// Create test tenant
	tenant, err := itf.CreateTestTenant(ctx, suite.Env().Pool)
	require.NoError(t, err)

	// Create test user in that tenant
	email, _ := internet.NewEmail("testuser@reset.com")
	testUser := user.New(
		"Test",
		"User",
		email,
		user.UILanguageEN,
		user.WithTenantID(tenant.ID),
	)
	testUser, _ = testUser.SetPassword("oldpassword123")

	createdUser, err := userService.Create(ctx, testUser)
	require.NoError(t, err)

	// Reset password
	newPassword := "newpassword123"
	resp := suite.POST(fmt.Sprintf("/superadmin/tenants/%s/users/%d/reset-password", tenant.ID.String(), createdUser.ID())).
		JSON(map[string]string{"password": newPassword}).
		Assert(t)

	resp.ExpectOK()

	// Verify response
	jsonAssert := resp.ExpectJSON()
	jsonAssert.ExpectField("$.success", true)
	jsonAssert.ExpectField("$.message", "Password reset successfully")

	// Verify password was actually changed
	updatedUser, err := userService.GetByID(ctx, createdUser.ID())
	require.NoError(t, err)
	require.True(t, updatedUser.CheckPassword(newPassword), "New password should work")
	require.False(t, updatedUser.CheckPassword("oldpassword123"), "Old password should not work")
}

// TestTenantsController_ResetUserPassword_InvalidTenantID tests malformed tenant UUID
func TestTenantsController_ResetUserPassword_InvalidTenantID(t *testing.T) {
	t.Parallel()

	suite := itf.NewSuiteBuilder(t).
		WithModules(append(modules.BuiltInModules, superadmin.NewModule(nil))...).
		WithUser(createSuperAdminUserForTenants()).
		Build()

	userService := itf.GetService[coreservices.UserService](suite.Env())
	controller := controllers.NewTenantsController(suite.Env().App, userService)
	suite.Register(controller)

	suite.POST("/superadmin/tenants/invalid-uuid/users/1/reset-password").
		JSON(map[string]string{"password": "newpassword123"}).
		Assert(t).
		ExpectBadRequest()
}

// TestTenantsController_ResetUserPassword_InvalidUserID tests non-numeric user ID
func TestTenantsController_ResetUserPassword_InvalidUserID(t *testing.T) {
	t.Parallel()

	suite := itf.NewSuiteBuilder(t).
		WithModules(append(modules.BuiltInModules, superadmin.NewModule(nil))...).
		WithUser(createSuperAdminUserForTenants()).
		Build()

	userService := itf.GetService[coreservices.UserService](suite.Env())
	controller := controllers.NewTenantsController(suite.Env().App, userService)
	suite.Register(controller)

	tenant, err := itf.CreateTestTenant(suite.Env().Ctx, suite.Env().Pool)
	require.NoError(t, err)

	suite.POST(fmt.Sprintf("/superadmin/tenants/%s/users/invalid/reset-password", tenant.ID.String())).
		JSON(map[string]string{"password": "newpassword123"}).
		Assert(t).
		ExpectBadRequest()
}

// TestTenantsController_ResetUserPassword_UserNotFound tests non-existent user
func TestTenantsController_ResetUserPassword_UserNotFound(t *testing.T) {
	t.Parallel()

	suite := itf.NewSuiteBuilder(t).
		WithModules(append(modules.BuiltInModules, superadmin.NewModule(nil))...).
		WithUser(createSuperAdminUserForTenants()).
		Build()

	userService := itf.GetService[coreservices.UserService](suite.Env())
	controller := controllers.NewTenantsController(suite.Env().App, userService)
	suite.Register(controller)

	tenant, err := itf.CreateTestTenant(suite.Env().Ctx, suite.Env().Pool)
	require.NoError(t, err)

	// Use non-existent user ID (999999)
	suite.POST(fmt.Sprintf("/superadmin/tenants/%s/users/999999/reset-password", tenant.ID.String())).
		JSON(map[string]string{"password": "newpassword123"}).
		Assert(t).
		ExpectNotFound()
}

// TestTenantsController_ResetUserPassword_WrongTenant tests cross-tenant protection
func TestTenantsController_ResetUserPassword_WrongTenant(t *testing.T) {
	t.Parallel()

	suite := itf.NewSuiteBuilder(t).
		WithModules(append(modules.BuiltInModules, superadmin.NewModule(nil))...).
		WithUser(createSuperAdminUserForTenants()).
		Build()

	userService := itf.GetService[coreservices.UserService](suite.Env())
	controller := controllers.NewTenantsController(suite.Env().App, userService)
	suite.Register(controller)

	ctx := suite.Env().Ctx

	// Create tenant A
	tenantA, err := itf.CreateTestTenant(ctx, suite.Env().Pool)
	require.NoError(t, err)

	// Create tenant B
	tenantB, err := itf.CreateTestTenant(ctx, suite.Env().Pool)
	require.NoError(t, err)

	// Create user in tenant A
	email, _ := internet.NewEmail("user@tenanta.com")
	testUser := user.New("Test", "User", email, user.UILanguageEN, user.WithTenantID(tenantA.ID))
	testUser, _ = testUser.SetPassword("password123")

	createdUser, err := userService.Create(ctx, testUser)
	require.NoError(t, err)

	// Try to reset password using tenant B's ID (should fail - cross-tenant protection)
	suite.POST(fmt.Sprintf("/superadmin/tenants/%s/users/%d/reset-password", tenantB.ID.String(), createdUser.ID())).
		JSON(map[string]string{"password": "newpassword123"}).
		Assert(t).
		ExpectNotFound() // Should return 404 for security (don't reveal user exists)
}

// TestTenantsController_ResetUserPassword_EmptyPassword tests empty password validation
func TestTenantsController_ResetUserPassword_EmptyPassword(t *testing.T) {
	t.Parallel()

	suite := itf.NewSuiteBuilder(t).
		WithModules(append(modules.BuiltInModules, superadmin.NewModule(nil))...).
		WithUser(createSuperAdminUserForTenants()).
		Build()

	userService := itf.GetService[coreservices.UserService](suite.Env())
	controller := controllers.NewTenantsController(suite.Env().App, userService)
	suite.Register(controller)

	tenant, err := itf.CreateTestTenant(suite.Env().Ctx, suite.Env().Pool)
	require.NoError(t, err)

	suite.POST(fmt.Sprintf("/superadmin/tenants/%s/users/1/reset-password", tenant.ID.String())).
		JSON(map[string]string{"password": ""}).
		Assert(t).
		ExpectBadRequest()
}

// TestTenantsController_ResetUserPassword_PasswordTooShort tests minimum length validation
func TestTenantsController_ResetUserPassword_PasswordTooShort(t *testing.T) {
	t.Parallel()

	suite := itf.NewSuiteBuilder(t).
		WithModules(append(modules.BuiltInModules, superadmin.NewModule(nil))...).
		WithUser(createSuperAdminUserForTenants()).
		Build()

	userService := itf.GetService[coreservices.UserService](suite.Env())
	controller := controllers.NewTenantsController(suite.Env().App, userService)
	suite.Register(controller)

	tenant, err := itf.CreateTestTenant(suite.Env().Ctx, suite.Env().Pool)
	require.NoError(t, err)

	t.Run("1_char", func(t *testing.T) {
		suite.POST(fmt.Sprintf("/superadmin/tenants/%s/users/1/reset-password", tenant.ID.String())).
			JSON(map[string]string{"password": "a"}).
			Assert(t).
			ExpectBadRequest()
	})

	t.Run("7_chars", func(t *testing.T) {
		suite.POST(fmt.Sprintf("/superadmin/tenants/%s/users/1/reset-password", tenant.ID.String())).
			JSON(map[string]string{"password": "1234567"}).
			Assert(t).
			ExpectBadRequest()
	})
}

// TestTenantsController_ResetUserPassword_Permissions tests that only superadmins can access
func TestTenantsController_ResetUserPassword_Permissions(t *testing.T) {
	t.Parallel()

	t.Run("SuperAdmin_Allowed", func(t *testing.T) {
		suite := itf.NewSuiteBuilder(t).
			WithModules(append(modules.BuiltInModules, superadmin.NewModule(nil))...).
			WithUser(createSuperAdminUserForTenants()).
			Build()

		userService := itf.GetService[coreservices.UserService](suite.Env())
		controller := controllers.NewTenantsController(suite.Env().App, userService)
		suite.Register(controller)

		ctx := suite.Env().Ctx
		tenant, err := itf.CreateTestTenant(ctx, suite.Env().Pool)
		require.NoError(t, err)

		email, _ := internet.NewEmail("test@example.com")
		testUser := user.New("Test", "User", email, user.UILanguageEN, user.WithTenantID(tenant.ID))
		testUser, _ = testUser.SetPassword("oldpass123")

		createdUser, err := userService.Create(ctx, testUser)
		require.NoError(t, err)

		// Superadmin should be able to reset password
		suite.POST(fmt.Sprintf("/superadmin/tenants/%s/users/%d/reset-password", tenant.ID.String(), createdUser.ID())).
			JSON(map[string]string{"password": "newpass123"}).
			Assert(t).
			ExpectOK()
	})

	t.Run("RegularUser_Forbidden", func(t *testing.T) {
		suite := itf.NewSuiteBuilder(t).
			WithModules(append(modules.BuiltInModules, superadmin.NewModule(nil))...).
			WithUser(createRegularUserForTenants()).
			Build()

		userService := itf.GetService[coreservices.UserService](suite.Env())
		controller := controllers.NewTenantsController(suite.Env().App, userService)
		suite.Register(controller)

		tenant, err := itf.CreateTestTenant(suite.Env().Ctx, suite.Env().Pool)
		require.NoError(t, err)

		// Regular user should be blocked
		suite.POST(fmt.Sprintf("/superadmin/tenants/%s/users/1/reset-password", tenant.ID.String())).
			JSON(map[string]string{"password": "newpass123"}).
			Assert(t).
			ExpectForbidden()
	})
}

func TestTenantsController_ResetUserPassword_PasswordTooLong(t *testing.T) {
	t.Parallel()

	suite := itf.NewSuiteBuilder(t).
		WithModules(append(modules.BuiltInModules, superadmin.NewModule(nil))...).
		WithUser(createSuperAdminUserForTenants()).
		Build()

	userService := itf.GetService[coreservices.UserService](suite.Env())
	controller := controllers.NewTenantsController(suite.Env().App, userService)
	suite.Register(controller)

	tenant, err := itf.CreateTestTenant(suite.Env().Ctx, suite.Env().Pool)
	require.NoError(t, err)

	// Create user for tenant
	var userID int
	err = suite.Env().Pool.QueryRow(
		suite.Env().Ctx,
		`INSERT INTO users (tenant_id, first_name, last_name, email, password, type, ui_language, created_at, updated_at)
		 VALUES ($1, 'Test', 'User', 'testuser@example.com', 'hashedpass', 'user', 'en', NOW(), NOW())
		 RETURNING id`,
		tenant.ID,
	).Scan(&userID)
	require.NoError(t, err)

	// Create 200-character password (exceeds 128 limit)
	longPassword := strings.Repeat("a", 200)

	suite.POST(fmt.Sprintf("/superadmin/tenants/%s/users/%d/reset-password", tenant.ID.String(), userID)).
		JSON(map[string]string{"password": longPassword}).
		Assert(t).
		ExpectBadRequest().
		ExpectBodyContains("cannot exceed")
}

func TestTenantsController_ResetUserPassword_InvalidContentType(t *testing.T) {
	t.Parallel()

	suite := itf.NewSuiteBuilder(t).
		WithModules(append(modules.BuiltInModules, superadmin.NewModule(nil))...).
		WithUser(createSuperAdminUserForTenants()).
		Build()

	userService := itf.GetService[coreservices.UserService](suite.Env())
	controller := controllers.NewTenantsController(suite.Env().App, userService)
	suite.Register(controller)

	tenant, err := itf.CreateTestTenant(suite.Env().Ctx, suite.Env().Pool)
	require.NoError(t, err)

	// Create user for tenant
	var userID int
	err = suite.Env().Pool.QueryRow(
		suite.Env().Ctx,
		`INSERT INTO users (tenant_id, first_name, last_name, email, password, type, ui_language, created_at, updated_at)
		 VALUES ($1, 'Test', 'User', 'testuser2@example.com', 'hashedpass', 'user', 'en', NOW(), NOW())
		 RETURNING id`,
		tenant.ID,
	).Scan(&userID)
	require.NoError(t, err)

	// Send request with wrong content-type
	suite.POST(fmt.Sprintf("/superadmin/tenants/%s/users/%d/reset-password", tenant.ID.String(), userID)).
		Header("Content-Type", "text/plain").
		JSON(map[string]string{"password": "validpassword123"}).
		Assert(t).
		ExpectStatus(http.StatusUnsupportedMediaType)
}
