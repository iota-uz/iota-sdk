package controllers_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/internet"
	"github.com/iota-uz/iota-sdk/modules/superadmin"
	"github.com/iota-uz/iota-sdk/modules/superadmin/presentation/controllers"
	"github.com/iota-uz/iota-sdk/pkg/itf"
)

// createSuperAdminUser creates a test user with TypeSuperAdmin
func createSuperAdminUser() user.User {
	email, _ := internet.NewEmail("superadmin@test.com")
	return user.New(
		"Super",
		"Admin",
		email,
		user.UILanguageEN,
		user.WithID(1),
		user.WithType(user.TypeSuperAdmin),
		user.WithTenantID(uuid.New()),
	)
}

// createRegularUser creates a test user with TypeUser (non-superadmin)
func createRegularUser() user.User {
	email, _ := internet.NewEmail("user@test.com")
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

func TestDashboardController_Index(t *testing.T) {
	t.Parallel()

	// Create test suite with superadmin module and superadmin user
	suite := itf.NewSuiteBuilder(t).
		WithModules(append(modules.BuiltInModules, superadmin.NewModule(nil))...).
		WithUser(createSuperAdminUser()).
		Build()

	// Register dashboard controller
	controller := controllers.NewDashboardController(suite.Env().App)
	suite.Register(controller)

	// Test GET /
	suite.GET("/").
		Assert(t).
		ExpectOK().
		ExpectBodyContains("Super Admin Dashboard")
}

func TestDashboardController_GetMetrics(t *testing.T) {
	t.Parallel()

	suite := itf.NewSuiteBuilder(t).
		WithModules(append(modules.BuiltInModules, superadmin.NewModule(nil))...).
		WithUser(createSuperAdminUser()).
		Build()

	controller := controllers.NewDashboardController(suite.Env().App)
	suite.Register(controller)

	// Test GET /metrics without date filters
	suite.GET("/metrics").
		Assert(t).
		ExpectOK()
}

func TestDashboardController_GetMetrics_WithDateFilter(t *testing.T) {
	t.Parallel()

	suite := itf.NewSuiteBuilder(t).
		WithModules(append(modules.BuiltInModules, superadmin.NewModule(nil))...).
		WithUser(createSuperAdminUser()).
		Build()

	controller := controllers.NewDashboardController(suite.Env().App)
	suite.Register(controller)

	// Test with valid date range
	startDate := time.Now().AddDate(0, 0, -30).Format("2006-01-02")
	endDate := time.Now().Format("2006-01-02")

	suite.GET("/metrics").
		WithQuery(map[string]string{
			"startDate": startDate,
			"endDate":   endDate,
		}).
		Assert(t).
		ExpectOK()
}

func TestDashboardController_GetMetrics_InvalidDateFormat(t *testing.T) {
	t.Parallel()

	suite := itf.NewSuiteBuilder(t).
		WithModules(append(modules.BuiltInModules, superadmin.NewModule(nil))...).
		WithUser(createSuperAdminUser()).
		Build()

	controller := controllers.NewDashboardController(suite.Env().App)
	suite.Register(controller)

	// Test invalid startDate format
	suite.GET("/metrics").
		WithQuery(map[string]string{
			"startDate": "invalid-date",
		}).
		Assert(t).
		ExpectBadRequest().
		ExpectBodyContains("Invalid startDate format")

	// Test invalid endDate format
	suite.GET("/metrics").
		WithQuery(map[string]string{
			"endDate": "not-a-date",
		}).
		Assert(t).
		ExpectBadRequest().
		ExpectBodyContains("Invalid endDate format")
}

func TestDashboardController_GetMetrics_EdgeCases(t *testing.T) {
	t.Parallel()

	suite := itf.NewSuiteBuilder(t).
		WithModules(append(modules.BuiltInModules, superadmin.NewModule(nil))...).
		WithUser(createSuperAdminUser()).
		Build()

	controller := controllers.NewDashboardController(suite.Env().App)
	suite.Register(controller)

	cases := itf.Cases(
		itf.GET("/metrics").
			Named("Only_StartDate").
			WithQuery(map[string]string{
				"startDate": time.Now().AddDate(0, 0, -7).Format("2006-01-02"),
			}).
			ExpectOK(),

		itf.GET("/metrics").
			Named("Only_EndDate").
			WithQuery(map[string]string{
				"endDate": time.Now().Format("2006-01-02"),
			}).
			ExpectOK(),

		itf.GET("/metrics").
			Named("Future_Date").
			WithQuery(map[string]string{
				"startDate": time.Now().AddDate(0, 0, 1).Format("2006-01-02"),
				"endDate":   time.Now().AddDate(0, 0, 7).Format("2006-01-02"),
			}).
			ExpectOK(),
	)

	suite.RunCases(cases)
}

func TestDashboardController_Permissions(t *testing.T) {
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
					WithUser(createSuperAdminUser()).
					Build()
			},
			expectedStatus: 200,
			description:    "Superadmin users should have full access to dashboard",
		},
		{
			name: "Regular_User_Blocked",
			setupSuite: func(t *testing.T) *itf.Suite {
				t.Helper()
				return itf.NewSuiteBuilder(t).
					WithModules(append(modules.BuiltInModules, superadmin.NewModule(nil))...).
					WithUser(createRegularUser()).
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
			controller := controllers.NewDashboardController(suite.Env().App)
			suite.Register(controller)

			suite.GET("/").
				Assert(t).
				ExpectStatus(tc.expectedStatus)
		})
	}
}

// TestDashboardController_SuperAdminOnly verifies only superadmin users can access all endpoints
func TestDashboardController_SuperAdminOnly(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name     string
		endpoint string
		method   string
	}{
		{
			name:     "Dashboard_Index",
			endpoint: "/",
			method:   "GET",
		},
		{
			name:     "Dashboard_Metrics",
			endpoint: "/metrics",
			method:   "GET",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name+"_SuperAdmin_OK", func(t *testing.T) {
			suite := itf.NewSuiteBuilder(t).
				WithModules(append(modules.BuiltInModules, superadmin.NewModule(nil))...).
				WithUser(createSuperAdminUser()).
				Build()

			controller := controllers.NewDashboardController(suite.Env().App)
			suite.Register(controller)

			suite.GET(tc.endpoint).
				Assert(t).
				ExpectOK()
		})

		t.Run(tc.name+"_RegularUser_Forbidden", func(t *testing.T) {
			suite := itf.NewSuiteBuilder(t).
				WithModules(append(modules.BuiltInModules, superadmin.NewModule(nil))...).
				WithUser(createRegularUser()).
				Build()

			controller := controllers.NewDashboardController(suite.Env().App)
			suite.Register(controller)

			suite.GET(tc.endpoint).
				Assert(t).
				ExpectForbidden()
		})
	}
}

func TestDashboardController_Routes(t *testing.T) {
	t.Parallel()

	suite := itf.NewSuiteBuilder(t).
		WithModules(append(modules.BuiltInModules, superadmin.NewModule(nil))...).
		WithUser(createSuperAdminUser()).
		Build()

	controller := controllers.NewDashboardController(suite.Env().App)
	suite.Register(controller)

	cases := itf.Cases(
		itf.GET("/").
			Named("Dashboard_Index").
			ExpectOK(),

		itf.GET("/metrics").
			Named("Dashboard_Metrics").
			ExpectOK(),

		itf.POST("/").
			Named("POST_Not_Allowed").
			ExpectStatus(404), // Router returns 404 for unsupported methods

		itf.DELETE("/").
			Named("DELETE_Not_Allowed").
			ExpectStatus(404), // Router returns 404 for unsupported methods
	)

	suite.RunCases(cases)
}
