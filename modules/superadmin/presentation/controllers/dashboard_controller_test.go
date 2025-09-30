package controllers_test

import (
	"testing"
	"time"

	"github.com/iota-uz/iota-sdk/modules"
	"github.com/iota-uz/iota-sdk/modules/superadmin"
	"github.com/iota-uz/iota-sdk/modules/superadmin/presentation/controllers"
	"github.com/iota-uz/iota-sdk/pkg/itf"
)

func TestDashboardController_Index(t *testing.T) {
	t.Parallel()

	// Create test suite with superadmin module
	suite := itf.NewSuiteBuilder(t).
		WithModules(append(modules.BuiltInModules, superadmin.NewModule(nil))...).
		AsAdmin().
		Build()

	// Register dashboard controller
	controller := controllers.NewDashboardController(suite.Env().App)
	suite.Register(controller)

	// Test GET /superadmin/dashboard
	suite.GET("/superadmin/dashboard").
		Assert(t).
		ExpectOK().
		ExpectBodyContains("SuperAdmin Dashboard")
}

func TestDashboardController_GetMetrics(t *testing.T) {
	t.Parallel()

	suite := itf.NewSuiteBuilder(t).
		WithModules(append(modules.BuiltInModules, superadmin.NewModule(nil))...).
		AsAdmin().
		Build()

	controller := controllers.NewDashboardController(suite.Env().App)
	suite.Register(controller)

	// Test GET /superadmin/dashboard/metrics without date filters
	suite.GET("/superadmin/dashboard/metrics").
		Assert(t).
		ExpectOK()
}

func TestDashboardController_GetMetrics_WithDateFilter(t *testing.T) {
	t.Parallel()

	suite := itf.NewSuiteBuilder(t).
		WithModules(append(modules.BuiltInModules, superadmin.NewModule(nil))...).
		AsAdmin().
		Build()

	controller := controllers.NewDashboardController(suite.Env().App)
	suite.Register(controller)

	// Test with valid date range
	startDate := time.Now().AddDate(0, 0, -30).Format(time.RFC3339)
	endDate := time.Now().Format(time.RFC3339)

	suite.GET("/superadmin/dashboard/metrics").
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
		AsAdmin().
		Build()

	controller := controllers.NewDashboardController(suite.Env().App)
	suite.Register(controller)

	// Test invalid startDate format
	suite.GET("/superadmin/dashboard/metrics").
		WithQuery(map[string]string{
			"startDate": "invalid-date",
		}).
		Assert(t).
		ExpectBadRequest().
		ExpectBodyContains("Invalid startDate format")

	// Test invalid endDate format
	suite.GET("/superadmin/dashboard/metrics").
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
		AsAdmin().
		Build()

	controller := controllers.NewDashboardController(suite.Env().App)
	suite.Register(controller)

	cases := itf.Cases(
		itf.GET("/superadmin/dashboard/metrics").
			Named("Only_StartDate").
			WithQuery(map[string]string{
				"startDate": time.Now().AddDate(0, 0, -7).Format(time.RFC3339),
			}).
			ExpectOK(),

		itf.GET("/superadmin/dashboard/metrics").
			Named("Only_EndDate").
			WithQuery(map[string]string{
				"endDate": time.Now().Format(time.RFC3339),
			}).
			ExpectOK(),

		itf.GET("/superadmin/dashboard/metrics").
			Named("Future_Date").
			WithQuery(map[string]string{
				"startDate": time.Now().AddDate(0, 0, 1).Format(time.RFC3339),
				"endDate":   time.Now().AddDate(0, 0, 7).Format(time.RFC3339),
			}).
			ExpectOK(),
	)

	suite.RunCases(cases)
}

func TestDashboardController_Permissions(t *testing.T) {
	t.Parallel()

	// Test with different permission levels
	testCases := []struct {
		name        string
		setupSuite  func(*testing.T) *itf.Suite
		expectation func(*itf.Request) *itf.ResponseAssertion
	}{
		{
			name: "Admin_Access",
			setupSuite: func(t *testing.T) *itf.Suite {
				return itf.NewSuiteBuilder(t).
					WithModules(append(modules.BuiltInModules, superadmin.NewModule(nil))...).
					AsAdmin().
					Build()
			},
			expectation: func(req *itf.Request) *itf.ResponseAssertion {
				return req.Assert(t).ExpectOK()
			},
		},
		{
			name: "Anonymous_User_Redirect",
			setupSuite: func(t *testing.T) *itf.Suite {
				return itf.NewSuiteBuilder(t).
					WithModules(append(modules.BuiltInModules, superadmin.NewModule(nil))...).
					AsAnonymous().
					Build()
			},
			expectation: func(req *itf.Request) *itf.ResponseAssertion {
				// Anonymous users should be redirected to login
				return req.Assert(t).ExpectStatus(302)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			suite := tc.setupSuite(t)
			controller := controllers.NewDashboardController(suite.Env().App)
			suite.Register(controller)

			tc.expectation(suite.GET("/superadmin/dashboard"))
		})
	}
}

func TestDashboardController_Routes(t *testing.T) {
	t.Parallel()

	suite := itf.NewSuiteBuilder(t).
		WithModules(append(modules.BuiltInModules, superadmin.NewModule(nil))...).
		AsAdmin().
		Build()

	controller := controllers.NewDashboardController(suite.Env().App)
	suite.Register(controller)

	cases := itf.Cases(
		itf.GET("/superadmin/dashboard").
			Named("Dashboard_Index").
			ExpectOK(),

		itf.GET("/superadmin/dashboard/metrics").
			Named("Dashboard_Metrics").
			ExpectOK(),

		itf.POST("/superadmin/dashboard").
			Named("POST_Not_Allowed").
			ExpectStatus(404), // Router returns 404 for unsupported methods

		itf.DELETE("/superadmin/dashboard").
			Named("DELETE_Not_Allowed").
			ExpectStatus(404), // Router returns 404 for unsupported methods
	)

	suite.RunCases(cases)
}
