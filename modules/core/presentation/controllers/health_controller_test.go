package controllers_test

import (
	"encoding/json"
	"testing"

	"github.com/iota-uz/iota-sdk/modules/core"
	"github.com/iota-uz/iota-sdk/modules/core/presentation/controllers"
	"github.com/iota-uz/iota-sdk/pkg/defaults"
	"github.com/iota-uz/iota-sdk/pkg/itf"
	"github.com/stretchr/testify/require"
)

func TestHealthController_Key_ReturnsCorrectPath(t *testing.T) {
	suite := itf.HTTP(t, core.NewModule(&core.ModuleOptions{
		PermissionSchema: defaults.PermissionSchema(),
	}))
	controller := controllers.NewHealthController(suite.Environment().App)

	require.Equal(t, "/health", controller.Key())
}

func TestHealthController_Get_Integration(t *testing.T) {
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

	// Note: Detailed fields (timestamp, version, uptime, checks) are only present
	// when HEALTH_DETAILED=true environment variable is set. By default (false),
	// only the status field is returned for minimal overhead.
}

func TestHealthController_Get_SimpleMode_Default(t *testing.T) {
	// Test default behavior (HEALTH_DETAILED=false by default)
	// This test validates the simple mode which returns only {"status": "healthy"}
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

// Note: Testing detailed mode requires HEALTH_DETAILED=true to be set
// in the environment before the application starts, as the configuration
// is loaded once at package initialization. This can be verified manually:
// HEALTH_DETAILED=true go test -v -run TestHealthController_Get_DetailedMode
