package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	goruntime "runtime"
	"testing"
	"time"

	appletenginehandlers "github.com/iota-uz/iota-sdk/pkg/appletengine/handlers"
	appletenginerpc "github.com/iota-uz/iota-sdk/pkg/appletengine/rpc"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBiChatRuntimeProbe_WithKVDBStubs(t *testing.T) {
	if _, err := exec.LookPath("bun"); err != nil {
		t.Skip("bun is not available in PATH")
	}

	entrypoint, ok := resolveBiChatRuntimeEntrypointForTest()
	if !ok {
		t.Skip("bichat runtime entrypoint or applets runtime sdk is not available in this checkout")
	}

	registry := appletenginerpc.NewRegistry()
	require.NoError(t, appletenginehandlers.NewKVStub().Register(registry, "bichat"))
	require.NoError(t, appletenginehandlers.NewDBStub().Register(registry, "bichat"))
	dispatcher := appletenginerpc.NewDispatcher(registry, nil, logrus.New())
	manager := NewManager(t.TempDir(), dispatcher, logrus.New())

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	process, err := manager.EnsureStarted(ctx, "bichat", entrypoint)
	require.NoError(t, err)
	require.NotNil(t, process)

	res, err := httpOverUnix(http.MethodGet, process.AppletSocket, "/__probe", nil, map[string]string{
		"x-iota-tenant-id":   "tenant-1",
		"x-iota-user-id":     "user-1",
		"x-iota-permissions": "BiChat.Access",
		"x-iota-request-id":  "req-1",
	})
	require.NoError(t, err)
	defer func() { _ = res.Body.Close() }()
	require.Equal(t, http.StatusOK, res.StatusCode)

	payload, err := io.ReadAll(res.Body)
	require.NoError(t, err)

	var decoded map[string]any
	require.NoError(t, json.Unmarshal(payload, &decoded))
	assert.Equal(t, true, decoded["ok"])
	assert.NotNil(t, decoded["kv"])
	assert.NotNil(t, decoded["db"])

	require.NoError(t, manager.Shutdown(context.Background()))
}

func resolveBiChatRuntimeEntrypointForTest() (string, bool) {
	_, currentFile, _, ok := goruntime.Caller(0)
	if !ok {
		return "", false
	}
	// pkg/appletengine/runtime/pilot_integration_test.go -> iota-sdk root
	iotaRoot := filepath.Clean(filepath.Join(filepath.Dir(currentFile), "..", "..", ".."))
	entrypoint := filepath.Join(iotaRoot, "modules", "bichat", "runtime", "index.ts")
	if _, err := os.Stat(entrypoint); err != nil {
		return "", false
	}
	appletRuntimeSDK := filepath.Clean(filepath.Join(iotaRoot, "..", "applets", "ui", "src", "applet-runtime", "index.ts"))
	if _, err := os.Stat(appletRuntimeSDK); err != nil {
		return "", false
	}
	return entrypoint, true
}

func httpOverUnix(method, socketPath, path string, body io.Reader, headers map[string]string) (*http.Response, error) {
	dialer := &net.Dialer{}
	transport := &http.Transport{
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			return dialer.DialContext(ctx, "unix", socketPath)
		},
	}
	defer transport.CloseIdleConnections()

	client := &http.Client{Transport: transport}
	req, err := http.NewRequestWithContext(context.Background(), method, "http://unix"+path, body)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	return client.Do(req)
}
