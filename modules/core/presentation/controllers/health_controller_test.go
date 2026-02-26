package controllers_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/iota-uz/iota-sdk/modules/core"
	"github.com/iota-uz/iota-sdk/modules/core/presentation/controllers"
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

func TestHealthController_Get(t *testing.T) {
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

	require.Contains(t, jsonResponse, "status")
	status := jsonResponse["status"].(string)
	require.Contains(t, []string{"healthy", "unhealthy"}, status)

	require.NotContains(t, jsonResponse, "timestamp")
	require.NotContains(t, jsonResponse, "version")
	require.NotContains(t, jsonResponse, "uptime")
	require.NotContains(t, jsonResponse, "checks")
}

func TestHealthController_QuickDBCheck_Timeout(t *testing.T) {
	t.Parallel()

	suite := itf.HTTP(t, core.NewModule(&core.ModuleOptions{
		PermissionSchema: defaults.PermissionSchema(),
	}))

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
	defer cancel()

	<-ctx.Done()

	db := suite.Environment().App.DB()
	require.NotNil(t, db, "database pool should be available")

	var result int
	err := db.QueryRow(ctx, "SELECT 1").Scan(&result)
	require.Error(t, err, "query should fail with expired context")
	require.ErrorIs(t, err, context.DeadlineExceeded, "error should be context.DeadlineExceeded")
}

func TestHealthController_QuickDBCheck_ErrorHandling(t *testing.T) {
	t.Parallel()

	suite := itf.HTTP(t, core.NewModule(&core.ModuleOptions{
		PermissionSchema: defaults.PermissionSchema(),
	}))

	pool := suite.Environment().App.DB()
	require.NotNil(t, pool, "database pool should exist initially")

	controller := controllers.NewHealthController(suite.Environment().App)
	suite.Register(controller)

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

	require.Contains(t, jsonResponse, "status")
	status := jsonResponse["status"].(string)
	require.Contains(t, []string{"healthy", "unhealthy"}, status)
}
