package controllers_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/iota-uz/iota-sdk/pkg/constants"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
)

func TestSettingsLogoController_ParseFailureEmitsOneBoundedEvent(t *testing.T) {
	// A direct presenter call would miss a controller's preceding raw log event.
	suite, _, _ := setupSettingsLogoControllerTest(t)
	var output bytes.Buffer
	logger := logrus.New()
	logger.SetOutput(&output)
	logger.SetFormatter(&logrus.JSONFormatter{})
	suite.WithMiddleware(func(ctx context.Context, _ *http.Request) context.Context {
		entry := logger.WithFields(logrus.Fields{"request-id": "settings-review", "private_payload": "inherited-secret"})
		return context.WithValue(ctx, constants.LoggerKey, entry)
	})
	response := suite.POST("/settings/logo").Form(url.Values{"LogoID": {"private-submitted-value"}}).Expect(t).Status(http.StatusBadRequest)
	require.Contains(t, response.Header("Content-Type"), "text/plain")
	require.NotContains(t, response.Body(), "private-submitted-value")
	raw := output.String()
	require.NotContains(t, raw, "private-submitted-value")
	require.NotContains(t, raw, "inherited-secret")
	lines := strings.Split(strings.TrimSpace(raw), "\n")
	require.Len(t, lines, 1)
	var event map[string]any
	require.NoError(t, json.Unmarshal([]byte(lines[0]), &event))
	require.Equal(t, "invalid", event["error.code"])
	require.Equal(t, "composables.UseForm", event["error.op"])
	require.Equal(t, "settings-review", event["request_id"])
	require.Equal(t, "info", event["level"])
}
