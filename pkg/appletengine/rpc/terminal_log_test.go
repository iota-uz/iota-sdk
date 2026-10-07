package rpc

import (
	"bytes"
	"encoding/json"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func TestObservePermissionDenialLogsOneBoundedEvent(t *testing.T) {
	var output bytes.Buffer
	logger := logrus.New()
	logger.SetOutput(&output)
	logger.SetFormatter(&logrus.JSONFormatter{})
	d := &Dispatcher{logger: logger}
	d.observe(Method{Name: "dashboard.save"}, transportPublic, "request-123", time.Now(), &rpcError{Code: "forbidden", Message: "permission denied"}, dispatchIdentity{})
	var event map[string]any
	decoder := json.NewDecoder(&output)
	require.NoError(t, decoder.Decode(&event))
	require.False(t, decoder.More())
	require.Equal(t, "permission_denied", event["error.code"])
	require.Equal(t, "dashboard.save", event["error.op"])
	require.Equal(t, "request-123", event["request_id"])
	require.Equal(t, "info", event["level"])
}
