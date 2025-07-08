package controllers_test

import (
	"testing"

	"github.com/iota-uz/iota-sdk/modules/core/presentation/controllers"
	"github.com/iota-uz/iota-sdk/pkg/testutils/controllertest"
	"github.com/stretchr/testify/require"
)

func TestHealthController_Key_ReturnsCorrectPath(t *testing.T) {
	suite := controllertest.New(t, []string{"core"})
	controller := controllers.NewHealthController(suite.App)

	require.Equal(t, "/health", controller.Key())
}

func TestHealthController_Get_Integration(t *testing.T) {
	suite := controllertest.New(t, []string{"core"})
	controller := controllers.NewHealthController(suite.App)

	response := suite.GET("/health", controller)

	require.Equal(t, 200, response.Code)
	require.Equal(t, "application/json", response.Header().Get("Content-Type"))

	jsonResponse := response.JSONObject()
	require.Contains(t, jsonResponse, "status")
	require.Contains(t, jsonResponse, "timestamp")
	require.Contains(t, jsonResponse, "version")
	require.Contains(t, jsonResponse, "uptime")
	require.Contains(t, jsonResponse, "checks")

	status := jsonResponse["status"].(string)
	require.Contains(t, []string{"healthy", "degraded", "down"}, status)

	checks := jsonResponse["checks"].(map[string]interface{})
	require.Contains(t, checks, "database")
	require.Contains(t, checks, "system")
}