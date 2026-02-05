package applet

import (
	"bytes"
	"context"
	"encoding/json"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/permission"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/internet"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing/fstest"
)

type testApplet struct {
	name     string
	basePath string
	config   Config
}

func (a *testApplet) Name() string     { return a.name }
func (a *testApplet) BasePath() string { return a.basePath }
func (a *testApplet) Config() Config   { return a.config }

type countingFS struct {
	fs fs.FS
	n  int
}

func (c *countingFS) Open(name string) (fs.File, error) {
	if name == "manifest.json" {
		c.n++
	}
	return c.fs.Open(name)
}

func TestAppletController_ManifestLoadedOnce(t *testing.T) {
	t.Parallel()

	manifest := `{"index.html":{"file":"assets/main-123.js","css":["assets/main-123.css"],"isEntry":true}}`
	mapFS := fstest.MapFS{
		"manifest.json":       {Data: []byte(manifest)},
		"assets/main-123.js":  {Data: []byte("console.log('x')")},
		"assets/main-123.css": {Data: []byte("body{}")},
	}
	cfs := &countingFS{fs: mapFS}

	a := &testApplet{
		name:     "t",
		basePath: "/t",
		config: Config{
			WindowGlobal: "__T__",
			Shell: ShellConfig{
				Mode:  ShellModeStandalone,
				Title: "t",
			},
			Assets: AssetConfig{
				FS:           cfs,
				BasePath:     "/assets",
				ManifestPath: "manifest.json",
				Entrypoint:   "index.html",
			},
		},
	}

	_ = NewAppletController(a, nil, DefaultSessionConfig, nil, nil)
	assert.Equal(t, 1, cfs.n)
}

func TestAppletController_DevProxy_SkipsManifestRequirements(t *testing.T) {
	t.Parallel()

	a := &testApplet{
		name:     "t",
		basePath: "/t",
		config: Config{
			WindowGlobal: "__T__",
			Shell: ShellConfig{
				Mode:  ShellModeStandalone,
				Title: "t",
			},
			Assets: AssetConfig{
				BasePath: "/assets",
				Dev: &DevAssetConfig{
					Enabled:     true,
					TargetURL:   "http://localhost:5173",
					EntryModule: "/src/main.tsx",
				},
			},
		},
	}

	c := NewAppletController(a, nil, DefaultSessionConfig, nil, nil)
	_, scripts, err := c.buildAssetTags()
	require.NoError(t, err)
	assert.Contains(t, scripts, "/@vite/client")
	assert.Contains(t, scripts, "/src/main.tsx")
}

func TestAppletController_RPC_MethodNotFound(t *testing.T) {
	t.Parallel()

	a := &testApplet{
		name:     "t",
		basePath: "/t",
		config: Config{
			WindowGlobal: "__T__",
			Shell: ShellConfig{
				Mode:  ShellModeStandalone,
				Title: "t",
			},
			Assets: AssetConfig{
				FS:           fstest.MapFS{"manifest.json": {Data: []byte(`{"index.html":{"file":"a.js","isEntry":true}}`)}},
				BasePath:     "/assets",
				ManifestPath: "manifest.json",
				Entrypoint:   "index.html",
			},
			RPC: &RPCConfig{
				Path: "/rpc",
				Methods: map[string]RPCMethod{
					"ok": {Handler: func(ctx context.Context, params json.RawMessage) (any, error) { return map[string]any{"ok": true}, nil }},
				},
			},
		},
	}

	c := NewAppletController(a, nil, DefaultSessionConfig, nil, nil)

	body := bytes.NewBufferString(`{"id":"1","method":"missing","params":{}}`)
	req := httptest.NewRequest(http.MethodPost, "/t/rpc", body)
	req.Host = "example.com"
	w := httptest.NewRecorder()

	c.handleRPC(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var resp rpcResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.NotNil(t, resp.Error)
	assert.Equal(t, "method_not_found", resp.Error.Code)
}

func TestAppletController_RPC_PermissionDenied(t *testing.T) {
	t.Parallel()

	a := &testApplet{
		name:     "t",
		basePath: "/t",
		config: Config{
			WindowGlobal: "__T__",
			Shell:        ShellConfig{Mode: ShellModeStandalone},
			Assets: AssetConfig{
				FS:           fstest.MapFS{"manifest.json": {Data: []byte(`{"index.html":{"file":"a.js","isEntry":true}}`)}},
				BasePath:     "/assets",
				ManifestPath: "manifest.json",
				Entrypoint:   "index.html",
			},
			RPC: &RPCConfig{
				Path: "/rpc",
				Methods: map[string]RPCMethod{
					"secret": {
						RequirePermissions: []string{"test.secret"},
						Handler:            func(ctx context.Context, params json.RawMessage) (any, error) { return "ok", nil },
					},
				},
			},
		},
	}

	c := NewAppletController(a, nil, DefaultSessionConfig, nil, nil)

	u := user.New("T", "U", internet.MustParseEmail("t@example.com"), user.UILanguageEN, user.WithID(1))
	req := httptest.NewRequest(http.MethodPost, "/t/rpc", bytes.NewBufferString(`{"id":"1","method":"secret","params":{}}`))
	req.Host = "example.com"
	req = req.WithContext(composables.WithUser(req.Context(), u))
	w := httptest.NewRecorder()

	c.handleRPC(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var resp rpcResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.NotNil(t, resp.Error)
	assert.Equal(t, "forbidden", resp.Error.Code)
}

func TestAppletController_RPC_SameOriginEnforced(t *testing.T) {
	t.Parallel()

	a := &testApplet{
		name:     "t",
		basePath: "/t",
		config: Config{
			WindowGlobal: "__T__",
			Shell:        ShellConfig{Mode: ShellModeStandalone},
			Assets: AssetConfig{
				FS:           fstest.MapFS{"manifest.json": {Data: []byte(`{"index.html":{"file":"a.js","isEntry":true}}`)}},
				BasePath:     "/assets",
				ManifestPath: "manifest.json",
				Entrypoint:   "index.html",
			},
			RPC: &RPCConfig{
				Path: "/rpc",
				Methods: map[string]RPCMethod{
					"ok": {Handler: func(ctx context.Context, params json.RawMessage) (any, error) { return "ok", nil }},
				},
			},
		},
	}

	c := NewAppletController(a, nil, DefaultSessionConfig, nil, nil)

	req := httptest.NewRequest(http.MethodPost, "/t/rpc", bytes.NewBufferString(`{"id":"1","method":"ok","params":{}}`))
	req.Host = "example.com"
	req.Header.Set("Origin", "http://evil.com")
	w := httptest.NewRecorder()

	c.handleRPC(w, req)
	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestAppletController_RPC_PayloadTooLarge(t *testing.T) {
	t.Parallel()

	maxBytes := int64(32)
	a := &testApplet{
		name:     "t",
		basePath: "/t",
		config: Config{
			WindowGlobal: "__T__",
			Shell:        ShellConfig{Mode: ShellModeStandalone},
			Assets: AssetConfig{
				FS:           fstest.MapFS{"manifest.json": {Data: []byte(`{"index.html":{"file":"a.js","isEntry":true}}`)}},
				BasePath:     "/assets",
				ManifestPath: "manifest.json",
				Entrypoint:   "index.html",
			},
			RPC: &RPCConfig{
				Path:         "/rpc",
				MaxBodyBytes: maxBytes,
				Methods: map[string]RPCMethod{
					"ok": {Handler: func(ctx context.Context, params json.RawMessage) (any, error) { return "ok", nil }},
				},
			},
		},
	}

	c := NewAppletController(a, nil, DefaultSessionConfig, nil, nil)

	req := httptest.NewRequest(http.MethodPost, "/t/rpc", bytes.NewBufferString(`{"id":"1","method":"ok","params":{"x":"this is too long"}}`))
	req.Host = "example.com"
	w := httptest.NewRecorder()

	c.handleRPC(w, req)
	assert.Equal(t, http.StatusRequestEntityTooLarge, w.Code)
}

func TestRequirePermissionStrings_Allows(t *testing.T) {
	t.Parallel()

	p := permission.New(
		permission.WithID(uuid.New()),
		permission.WithName("test.secret"),
		permission.WithResource("test"),
		permission.WithAction(permission.ActionRead),
		permission.WithModifier(permission.ModifierAll),
	)
	u := user.New("T", "U", internet.MustParseEmail("t@example.com"), user.UILanguageEN, user.WithID(1))
	u = u.AddPermission(p)

	ctx := composables.WithUser(context.Background(), u)
	require.NoError(t, requirePermissionStrings(ctx, []string{"test.secret"}))
}
