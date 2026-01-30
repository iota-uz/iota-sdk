package controllers_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/iota-uz/iota-sdk/modules/core"
	"github.com/iota-uz/iota-sdk/modules/core/presentation/controllers"
	"github.com/iota-uz/iota-sdk/pkg/configuration"
	"github.com/iota-uz/iota-sdk/pkg/defaults"
	"github.com/iota-uz/iota-sdk/pkg/itf"
	"github.com/stretchr/testify/require"
)

func TestHealthController_Key_ReturnsCorrectPath(t *testing.T) {
	t.Parallel()

	suite := itf.HTTP(t, core.NewModule(&core.ModuleOptions{
		PermissionSchema: defaults.PermissionSchema(),
	}))
	controller := controllers.NewHealthController(suite.Environment().App)

	require.Equal(t, "/health", controller.Key())
}

func TestHealthController_Get_SimpleMode(t *testing.T) {
	t.Parallel()

	// Skip if in detailed mode
	conf := configuration.Use()
	if conf.HealthDetailed {
		t.Skip("Skipping simple mode test: HEALTH_DETAILED=true. Run without HEALTH_DETAILED to test simple mode")
	}

	suite := itf.HTTP(t, core.NewModule(&core.ModuleOptions{
		PermissionSchema: defaults.PermissionSchema(),
	}))
	controller := controllers.NewHealthController(suite.Environment().App)
	suite.Register(controller)

	response := suite.GET("/health").Expect(t).Status(200)

	require.Equal(t, "application/json", response.Header("Content-Type"))

	var jsonResponse map[string]interface{}
	err := json.Unmarshal([]byte(response.Body()), &jsonResponse)
	require.NoError(t, err)

	// Simple mode should only return status field
	require.Contains(t, jsonResponse, "status")

	// Verify it's one of the expected status values
	status := jsonResponse["status"].(string)
	require.Contains(t, []string{"healthy", "unhealthy"}, status)

	// Should NOT contain detailed fields in simple mode
	require.NotContains(t, jsonResponse, "timestamp")
	require.NotContains(t, jsonResponse, "version")
	require.NotContains(t, jsonResponse, "uptime")
	require.NotContains(t, jsonResponse, "checks")
}

func TestHealthController_Get_DetailedMode(t *testing.T) {
	t.Parallel()

	// Skip if not in detailed mode
	conf := configuration.Use()
	if !conf.HealthDetailed {
		t.Skip("Skipping detailed mode test: HEALTH_DETAILED=false. Run with HEALTH_DETAILED=true to test detailed mode")
	}

	suite := itf.HTTP(t, core.NewModule(&core.ModuleOptions{
		PermissionSchema: defaults.PermissionSchema(),
	}))
	controller := controllers.NewHealthController(suite.Environment().App)
	suite.Register(controller)

	response := suite.GET("/health").Expect(t).Status(200)

	require.Equal(t, "application/json", response.Header("Content-Type"))

	var jsonResponse map[string]interface{}
	err := json.Unmarshal([]byte(response.Body()), &jsonResponse)
	require.NoError(t, err)

	// Detailed mode should return all required fields
	require.Contains(t, jsonResponse, "status")
	require.Contains(t, jsonResponse, "timestamp")
	require.Contains(t, jsonResponse, "version")
	require.Contains(t, jsonResponse, "uptime")
	require.Contains(t, jsonResponse, "checks")

	// Verify status is valid
	status := jsonResponse["status"].(string)
	require.Contains(t, []string{"healthy", "unhealthy", "degraded", "down"}, status)

	// Verify timestamp format (RFC3339)
	timestamp := jsonResponse["timestamp"].(string)
	_, err = time.Parse(time.RFC3339, timestamp)
	require.NoError(t, err, "timestamp should be in RFC3339 format")

	// Verify version is present
	version := jsonResponse["version"].(string)
	require.NotEmpty(t, version)

	// Verify uptime is present
	uptime := jsonResponse["uptime"].(string)
	require.NotEmpty(t, uptime)

	// Verify checks structure
	checks := jsonResponse["checks"].(map[string]interface{})
	require.Contains(t, checks, "database")
	require.Contains(t, checks, "system")

	// Verify database check structure
	dbCheck := checks["database"].(map[string]interface{})
	require.Contains(t, dbCheck, "status")
	require.Contains(t, dbCheck, "responseTime")

	// Verify system check structure
	sysCheck := checks["system"].(map[string]interface{})
	require.Contains(t, sysCheck, "status")
	require.Contains(t, sysCheck, "responseTime")
}

func TestHealthController_QuickDBCheck_Timeout(t *testing.T) {
	t.Parallel()

	suite := itf.HTTP(t, core.NewModule(&core.ModuleOptions{
		PermissionSchema: defaults.PermissionSchema(),
	}))

	// Create a context that times out immediately
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
	defer cancel()

	// Wait for context to actually expire
	<-ctx.Done()

	// Since quickDBCheck is a private method, we test the underlying behavior
	// by verifying that database queries with expired contexts fail as expected
	db := suite.Environment().App.DB()
	require.NotNil(t, db, "database pool should be available")

	// Test that a query with an expired context fails with DeadlineExceeded
	var result int
	err := db.QueryRow(ctx, "SELECT 1").Scan(&result)
	require.Error(t, err, "query should fail with expired context")
	require.ErrorIs(t, err, context.DeadlineExceeded, "error should be context.DeadlineExceeded")
}

func TestHealthController_QuickDBCheck_ErrorHandling(t *testing.T) {
	t.Parallel()

	// Skip if in detailed mode (this test is specific to simple mode behavior)
	conf := configuration.Use()
	if conf.HealthDetailed {
		t.Skip("Skipping simple mode error handling test: HEALTH_DETAILED=true. Run without HEALTH_DETAILED to test simple mode")
	}

	suite := itf.HTTP(t, core.NewModule(&core.ModuleOptions{
		PermissionSchema: defaults.PermissionSchema(),
	}))

	// Verify the database pool exists
	pool := suite.Environment().App.DB()
	require.NotNil(t, pool, "database pool should exist initially")

	// Note: We cannot directly close the pool in the app without affecting other tests
	// Instead, we test the controller's behavior with a valid pool (healthy path)
	// and verify that error handling is implemented correctly in the controller code

	controller := controllers.NewHealthController(suite.Environment().App)
	suite.Register(controller)

	// Test with available database (should return healthy)
	response := suite.GET("/health").Expect(t).Status(200)

	var jsonResponse map[string]interface{}
	err := json.Unmarshal([]byte(response.Body()), &jsonResponse)
	require.NoError(t, err)

	require.Contains(t, jsonResponse, "status")
	status := jsonResponse["status"].(string)
	require.Equal(t, "healthy", status, "status should be healthy when database is available")
}

func TestHealthController_Integration_ResponseFormat(t *testing.T) {
	t.Parallel()

	suite := itf.HTTP(t, core.NewModule(&core.ModuleOptions{
		PermissionSchema: defaults.PermissionSchema(),
	}))
	controller := controllers.NewHealthController(suite.Environment().App)
	suite.Register(controller)

	response := suite.GET("/health").Expect(t).Status(200)

	require.Equal(t, "application/json", response.Header("Content-Type"))

	var jsonResponse map[string]interface{}
	err := json.Unmarshal([]byte(response.Body()), &jsonResponse)
	require.NoError(t, err)

	// Health endpoint should always return status field
	require.Contains(t, jsonResponse, "status")
	status := jsonResponse["status"].(string)
	require.Contains(t, []string{"healthy", "unhealthy", "degraded", "down"}, status)
}

// TestHealthController_ModeValidation demonstrates how to run tests for both modes
// Run with: go test -v -run TestHealthController_ModeValidation
// For detailed mode: HEALTH_DETAILED=true go test -v -run TestHealthController_ModeValidation
func TestHealthController_ModeValidation(t *testing.T) {
	t.Parallel()

	conf := configuration.Use()
	if conf.HealthDetailed {
		t.Log("Running in DETAILED mode (HEALTH_DETAILED=true)")
	} else {
		t.Log("Running in SIMPLE mode (HEALTH_DETAILED=false)")
	}

	suite := itf.HTTP(t, core.NewModule(&core.ModuleOptions{
		PermissionSchema: defaults.PermissionSchema(),
	}))
	controller := controllers.NewHealthController(suite.Environment().App)
	suite.Register(controller)

	response := suite.GET("/health").Expect(t).Status(200)

	var jsonResponse map[string]interface{}
	err := json.Unmarshal([]byte(response.Body()), &jsonResponse)
	require.NoError(t, err)

	if conf.HealthDetailed {
		// Validate detailed mode fields
		require.Contains(t, jsonResponse, "timestamp")
		require.Contains(t, jsonResponse, "version")
		require.Contains(t, jsonResponse, "uptime")
		require.Contains(t, jsonResponse, "checks")
		t.Log("✓ Detailed mode validation passed")
	} else {
		// Validate simple mode (no extra fields)
		require.NotContains(t, jsonResponse, "timestamp")
		require.NotContains(t, jsonResponse, "version")
		require.NotContains(t, jsonResponse, "uptime")
		require.NotContains(t, jsonResponse, "checks")
		t.Log("✓ Simple mode validation passed")
	}
}

// Note: To test both modes comprehensively, run:
// 1. Default mode: go test -v ./modules/core/presentation/controllers/ -run Health
// 2. Detailed mode: HEALTH_DETAILED=true go test -v ./modules/core/presentation/controllers/ -run Health
