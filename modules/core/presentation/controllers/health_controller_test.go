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

	suite := itf.NewSuiteBuilder(t).WithModules(core.NewModule(&core.ModuleOptions{
		PermissionSchema: defaults.PermissionSchema(),
	})).Build()
	controller := controllers.NewHealthController(suite.Environment().App)

	require.Equal(t, "/health", controller.Key())
}

func TestHealthController_Get_ReturnsMinimalHealthyPayload(t *testing.T) {
	t.Parallel()

	suite := itf.NewSuiteBuilder(t).WithModules(core.NewModule(&core.ModuleOptions{
		PermissionSchema: defaults.PermissionSchema(),
	})).Build()
	controller := controllers.NewHealthController(suite.Environment().App)
	suite.Register(controller)

	response := suite.GET("/health").Expect(t).Status(200)
	require.Equal(t, "application/json", response.Header("Content-Type"))

	var body map[string]any
	err := json.Unmarshal([]byte(response.Body()), &body)
	require.NoError(t, err)

	require.Equal(t, "healthy", body["status"])
	require.Len(t, body, 1)
}

func TestHealthController_QuickDBCheck_Timeout(t *testing.T) {
	t.Parallel()

	suite := itf.NewSuiteBuilder(t).WithModules(core.NewModule(&core.ModuleOptions{
		PermissionSchema: defaults.PermissionSchema(),
	})).Build()

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
	defer cancel()
	<-ctx.Done()

	db := suite.Environment().App.DB()
	require.NotNil(t, db, "database pool should be available")

	var result int
	err := db.QueryRow(ctx, "SELECT 1").Scan(&result)
	require.Error(t, err)
	require.ErrorIs(t, err, context.DeadlineExceeded)
}

func TestHealthController_Integration_ResponseFormat(t *testing.T) {
	t.Parallel()

	suite := itf.NewSuiteBuilder(t).WithModules(core.NewModule(&core.ModuleOptions{
		PermissionSchema: defaults.PermissionSchema(),
	})).Build()
	controller := controllers.NewHealthController(suite.Environment().App)
	suite.Register(controller)

	response := suite.GET("/health").Expect(t).Status(200)

	var body map[string]any
	err := json.Unmarshal([]byte(response.Body()), &body)
	require.NoError(t, err)

	require.Contains(t, body, "status")
	require.NotContains(t, body, "timestamp")
	require.NotContains(t, body, "version")
	require.NotContains(t, body, "uptime")
	require.NotContains(t, body, "checks")
}
