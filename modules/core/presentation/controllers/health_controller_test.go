package controllers_test

import (
	"encoding/json"
	"testing"

	"github.com/iota-uz/iota-sdk/modules/core"
	"github.com/iota-uz/iota-sdk/modules/core/presentation/controllers"
	"github.com/iota-uz/iota-sdk/pkg/itf"
	"github.com/stretchr/testify/require"
)

func TestHealthController_Key_ReturnsCorrectPath(t *testing.T) {
	suite := itf.HTTP(t, core.NewModule())
	controller := controllers.NewHealthController(suite.Environment().App)

	require.Equal(t, "/health", controller.Key())
}

func TestHealthController_Get_Integration(t *testing.T) {
	suite := itf.HTTP(t, core.NewModule())
	controller := controllers.NewHealthController(suite.Environment().App)
	suite.Register(controller)

	response := suite.GET("/health").Expect(t).Status(200)

	require.Equal(t, "application/json", response.Header("Content-Type"))

	var jsonResponse map[string]interface{}
	err := json.Unmarshal([]byte(response.Body()), &jsonResponse)
	require.NoError(t, err)

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
